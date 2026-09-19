package net.radiobalkan.app;

import android.content.Context;
import android.util.JsonReader;
import android.util.JsonToken;
import org.json.JSONArray;
import org.json.JSONObject;
import java.io.BufferedInputStream;
import java.io.BufferedOutputStream;
import java.io.BufferedWriter;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStreamReader;
import java.io.OutputStreamWriter;
import java.net.HttpURLConnection;
import java.net.URI;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.CompletionService;
import java.util.concurrent.ExecutorCompletionService;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.RejectedExecutionException;

public final class RadioRepository {
    public interface Listener {
        void onCached(List<RadioStation> stations);
        void onLoaded(List<RadioStation> stations);
        void onError(Throwable error, boolean hasCache);
    }

    public static final String FOREIGN_CODE = "INT";
    public static final String DIASPORA_CODE = "DIA";
    public static final String[][] COUNTRIES = {
            {"", "Sve postaje"}, {"HR", "Hrvatska"}, {"BA", "Bosna i Hercegovina"},
            {"RS", "Srbija"}, {"SI", "Slovenija"}, {"MK", "Sjeverna Makedonija"},
            {"AL", "Albanija"}, {"ME", "Crna Gora"}, {"BG", "Bugarska"}, {DIASPORA_CODE, "Dijaspora"},
            {FOREIGN_CODE, "Strano"}
    };

    private static final String[] REGION_CODES = {"HR", "BA", "RS", "SI", "MK", "AL", "ME", "BG"};
    private static final Set<String> REGION_SET = new LinkedHashSet<>();
    static { Collections.addAll(REGION_SET, REGION_CODES); }

    private static final String[] FALLBACK_BASES = {
            "https://de1.api.radio-browser.info", "https://de2.api.radio-browser.info",
            "https://at1.api.radio-browser.info", "https://nl1.api.radio-browser.info"
    };
    private static final int PAGE = 250;
    private static final int MAX_PER_COUNTRY = 1800;
    private static final int MAX_FOREIGN = 180;
    private static final int MAX_DIASPORA = 180;
    private static final int FOREIGN_SCAN_LIMIT = 1600;
    private static final int DIASPORA_QUERY_LIMIT = 140;
    private static final int MAX_CATALOG = 7360;
    private static final int PRODUCTION_MIN_REGIONAL = 600;
    private static final int MAX_REDIRECTS = 4;

    private final Context context;
    private final ExecutorService worker = Executors.newSingleThreadExecutor(r -> {
        Thread t = new Thread(r, "radio-catalog");
        t.setPriority(Thread.NORM_PRIORITY - 1);
        return t;
    });
    private final ExecutorService countryPool = Executors.newFixedThreadPool(3, r -> {
        Thread t = new Thread(r, "radio-country");
        t.setPriority(Thread.MIN_PRIORITY);
        return t;
    });
    private volatile boolean closed;
    private volatile List<String> bases;

    public RadioRepository(Context context) {
        this.context = context.getApplicationContext();
    }

    public void load(Listener listener) {
        if (closed) return;
        try {
            worker.execute(() -> {
                List<RadioStation> cached = loadCache();
                if (closed) return;
                if (!cached.isEmpty()) safeCached(listener, cached);
                try {
                    List<RadioStation> online = mergeMissingGroups(fetchAll(), cached);
                    if (countRegional(online) < PRODUCTION_MIN_REGIONAL && !cached.isEmpty()) {
                        List<RadioStation> combined = new ArrayList<>(online);
                        combined.addAll(cached);
                        online = mergeMissingGroups(combined, cached);
                    }
                    if (closed) return;
                    if (!online.isEmpty()) {
                        saveCache(online);
                        safeLoaded(listener, online);
                    } else if (cached.isEmpty()) {
                        safeError(listener, new IllegalStateException("Popis radio stanica trenutačno nije dostupan"), false);
                    }
                } catch (Throwable t) {
                    if (closed) return;
                    AppLog.e(context, "catalog", t);
                    safeError(listener, t, !cached.isEmpty());
                }
            });
        } catch (RejectedExecutionException ignored) {
            if (!closed) safeError(listener, new IllegalStateException("Katalog se trenutačno ne može učitati"), false);
        }
    }

    public void shutdown() {
        closed = true;
        worker.shutdownNow();
        countryPool.shutdownNow();
    }

    private List<RadioStation> fetchAll() throws Exception {
        CompletionService<List<RadioStation>> completion = new ExecutorCompletionService<>(countryPool);
        int tasks = 0;
        for (String code : REGION_CODES) {
            completion.submit(() -> fetchCountry(code));
            tasks++;
        }
        completion.submit(this::fetchForeign);
        tasks++;
        completion.submit(this::fetchDiaspora);
        tasks++;

        CatalogAccumulator unique = new CatalogAccumulator();
        Throwable first = null;
        for (int n = 0; n < tasks; n++) {
            if (closed || Thread.currentThread().isInterrupted()) break;
            try {
                for (RadioStation station : completion.take().get()) unique.add(station);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw e;
            } catch (Throwable t) {
                if (first == null) first = t;
                AppLog.e(context, "catalog-partial", t);
            }
        }
        if (unique.size() == 0 && first != null) throw new Exception(first);
        return sortCatalog(unique.values());
    }

    private static final class CatalogAccumulator {
        private final Map<String, Integer> byUuid = new LinkedHashMap<>();
        private final Map<String, Integer> byIdentity = new LinkedHashMap<>();
        private final List<RadioStation> items = new ArrayList<>();

        int size() { return items.size(); }
        List<RadioStation> values() { return new ArrayList<>(items); }

        void add(RadioStation station) {
            if (!isUsable(station)) return;
            String uuid = safe(station.stationUuid);
            String identity = identity(station);
            Integer pos = uuid.isEmpty() ? null : byUuid.get(uuid);
            if (pos == null && !identity.isEmpty()) pos = byIdentity.get(identity);
            if (pos != null) {
                RadioStation merged = merge(items.get(pos), station);
                items.set(pos, merged);
                if (!safe(merged.stationUuid).isEmpty()) byUuid.put(merged.stationUuid, pos);
                String mergedIdentity = identity(merged);
                if (!mergedIdentity.isEmpty()) byIdentity.put(mergedIdentity, pos);
                return;
            }
            int next = items.size();
            items.add(station);
            if (!uuid.isEmpty()) byUuid.put(uuid, next);
            if (!identity.isEmpty()) byIdentity.put(identity, next);
        }
    }

    private static boolean isUsable(RadioStation station) {
        return station != null && !safe(station.name).isEmpty()
                && isSupportedCountry(station.countryCode)
                && (StreamResolver.isSafeHttp(station.url) || StreamResolver.isSafeHttp(station.urlResolved));
    }

    public static boolean isSupportedCountry(String code) {
        String normalized = safe(code).toUpperCase(Locale.ROOT);
        return FOREIGN_CODE.equals(normalized) || DIASPORA_CODE.equals(normalized) || REGION_SET.contains(normalized);
    }

    private static boolean isRegionalCountry(String code) {
        return REGION_SET.contains(safe(code).toUpperCase(Locale.ROOT));
    }

    private static String safe(String value) { return value == null ? "" : value.trim(); }

    private static int quality(RadioStation station) {
        int q = Math.max(0, station.votes);
        if (station.lastCheckOk == 1) q += 1_000_000;
        if (StreamResolver.isSafeHttp(station.favicon)) q += 20_000;
        if (StreamResolver.isSafeHttp(station.homepage)) q += 10_000;
        if (StreamResolver.isSafeHttp(station.urlResolved)) q += 5_000;
        q += Math.min(512, Math.max(0, station.bitrate));
        return q;
    }

    private static RadioStation merge(RadioStation a, RadioStation b) {
        RadioStation primary = quality(b) > quality(a) ? b : a;
        RadioStation other = primary == a ? b : a;
        if (safe(primary.stationUuid).isEmpty()) primary.stationUuid = other.stationUuid;
        if (safe(primary.name).isEmpty()) primary.name = other.name;
        if (safe(primary.url).isEmpty()) primary.url = other.url;
        if (safe(primary.urlResolved).isEmpty()) primary.urlResolved = other.urlResolved;
        if (safe(primary.homepage).isEmpty()) primary.homepage = other.homepage;
        if (safe(primary.favicon).isEmpty()) primary.favicon = !safe(other.favicon).isEmpty() ? other.favicon : RadioStation.websiteIcon(primary.homepage);
        if (safe(primary.tags).isEmpty()) primary.tags = other.tags;
        if (safe(primary.country).isEmpty()) primary.country = other.country;
        if (safe(primary.countryCode).isEmpty()) primary.countryCode = other.countryCode;
        if (safe(primary.sourceCountryCode).isEmpty()) primary.sourceCountryCode = other.sourceCountryCode;
        if (safe(primary.state).isEmpty()) primary.state = other.state;
        if (safe(primary.language).isEmpty()) primary.language = other.language;
        if (safe(primary.codec).isEmpty()) primary.codec = other.codec;
        if (primary.bitrate <= 0) primary.bitrate = other.bitrate;
        primary.votes = Math.max(primary.votes, other.votes);
        primary.lastCheckOk = Math.max(primary.lastCheckOk, other.lastCheckOk);
        if (DIASPORA_CODE.equalsIgnoreCase(a.countryCode) || DIASPORA_CODE.equalsIgnoreCase(b.countryCode)) {
            primary.countryCode = DIASPORA_CODE;
            if (safe(primary.sourceCountryCode).isEmpty()) {
                primary.sourceCountryCode = !safe(a.sourceCountryCode).isEmpty() ? a.sourceCountryCode : b.sourceCountryCode;
            }
            if (!safe(primary.tags).toLowerCase(Locale.ROOT).contains("dijaspora")) {
                primary.tags = safe(primary.tags).isEmpty() ? "dijaspora" : "dijaspora," + primary.tags;
            }
        }
        primary.refreshIndexes();
        return primary;
    }

    private static String identity(RadioStation station) {
        String name = RadioStation.fold(safe(station.name)).replaceAll("[^\\p{L}\\p{Nd}]", "");
        String country = safe(station.countryCode).toUpperCase(Locale.ROOT);
        if (name.isEmpty()) return "";
        String home = host(station.homepage);
        if (!home.isEmpty()) return country + "|" + name + "|home:" + home;
        String stream = !safe(station.urlResolved).isEmpty() ? station.urlResolved : station.url;
        try {
            URI uri = URI.create(stream);
            String h = uri.getHost() == null ? "" : uri.getHost().toLowerCase(Locale.ROOT).replaceAll("\\.+$", "");
            String path = uri.getPath() == null ? "" : uri.getPath().toLowerCase(Locale.ROOT);
            if (!h.isEmpty()) return country + "|" + name + "|stream:" + h + path;
        } catch (Throwable ignored) { }
        return country + "|" + name;
    }

    private static String host(String raw) {
        try {
            URI uri = URI.create(safe(raw));
            String h = uri.getHost();
            if (h == null) return "";
            h = h.toLowerCase(Locale.ROOT).replaceAll("\\.+$", "");
            return h.startsWith("www.") ? h.substring(4) : h;
        } catch (Throwable ignored) { return ""; }
    }

    private List<RadioStation> fetchCountry(String rawCode) throws Exception {
        String code = safe(rawCode).toUpperCase(Locale.ROOT);
        if (!isRegionalCountry(code)) throw new IllegalArgumentException("Nepodržana država");
        Exception last = null;
        for (String base : apiBases()) {
            if (closed) throw new InterruptedException("zatvaranje");
            try {
                List<RadioStation> out = new ArrayList<>();
                for (int offset = 0; offset < MAX_PER_COUNTRY; offset += PAGE) {
                    if (closed || Thread.currentThread().isInterrupted()) throw new InterruptedException("zatvaranje");
                    String endpoint = base + "/json/stations/search?countrycode=" + code
                            + "&hidebroken=true&order=votes&reverse=true&limit=" + PAGE + "&offset=" + offset;
                    JSONArray rows = new JSONArray(get(endpoint, 6 * 1024 * 1024));
                    if (rows.length() == 0) break;
                    int accepted = 0;
                    for (int i = 0; i < rows.length(); i++) {
                        JSONObject object = rows.optJSONObject(i);
                        if (object == null) continue;
                        RadioStation station = RadioStation.fromJson(object);
                        String actual = safe(station.countryCode).toUpperCase(Locale.ROOT);
                        if (actual.isEmpty()) actual = code;
                        if (!code.equals(actual)) continue;
                        station.countryCode = code;
                        if (safe(station.country).isEmpty()) station.country = countryName(code);
                        station.refreshIndexes();
                        if (isUsable(station)) {
                            out.add(station);
                            accepted++;
                        }
                    }
                    if (rows.length() < PAGE || accepted == 0) break;
                }
                if (out.isEmpty()) {
                    String endpoint = base + "/json/stations/bycountrycodeexact/" + code
                            + "?hidebroken=true&order=votes&reverse=true&limit=" + MAX_PER_COUNTRY;
                    JSONArray rows = new JSONArray(get(endpoint, 10 * 1024 * 1024));
                    for (int i = 0; i < rows.length(); i++) {
                        JSONObject object = rows.optJSONObject(i);
                        if (object == null) continue;
                        RadioStation station = RadioStation.fromJson(object);
                        String actual = safe(station.countryCode).toUpperCase(Locale.ROOT);
                        if (actual.isEmpty()) actual = code;
                        if (!code.equals(actual)) continue;
                        station.countryCode = code;
                        if (safe(station.country).isEmpty()) station.country = countryName(code);
                        station.refreshIndexes();
                        if (isUsable(station)) out.add(station);
                    }
                }
                if (!out.isEmpty()) return dedupe(out);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw e;
            } catch (Exception e) {
                last = e;
            }
        }
        throw last == null ? new IllegalStateException("API nije dostupan") : last;
    }

    private List<RadioStation> fetchForeign() throws Exception {
        Exception last = null;
        for (String base : apiBases()) {
            if (closed) throw new InterruptedException("zatvaranje");
            try {
                String endpoint = base + "/json/stations/search?hidebroken=true&order=votes&reverse=true&limit=" + FOREIGN_SCAN_LIMIT;
                JSONArray rows = new JSONArray(get(endpoint, 10 * 1024 * 1024));
                List<RadioStation> out = new ArrayList<>();
                for (int i = 0; i < rows.length(); i++) {
                    JSONObject object = rows.optJSONObject(i);
                    if (object == null) continue;
                    RadioStation station = RadioStation.fromJson(object);
                    String originalCode = safe(station.countryCode).toUpperCase(Locale.ROOT);
                    if (originalCode.isEmpty() || isRegionalCountry(originalCode) || station.lastCheckOk != 1) continue;
                    if (!StreamResolver.isSafeHttp(station.url) && !StreamResolver.isSafeHttp(station.urlResolved)) continue;
                    station.sourceCountryCode = originalCode;
                    station.countryCode = FOREIGN_CODE;
                    if (safe(station.country).isEmpty()) station.country = "Strana postaja";
                    station.refreshIndexes();
                    out.add(station);
                }
                out = dedupe(out);
                out.sort(Comparator.comparingInt((RadioStation station) -> station.votes).reversed()
                        .thenComparing(station -> RadioStation.fold(station.name)));
                if (out.size() > MAX_FOREIGN) out = new ArrayList<>(out.subList(0, MAX_FOREIGN));
                if (!out.isEmpty()) return out;
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw e;
            } catch (Exception e) {
                last = e;
            }
        }
        if (last != null) throw last;
        return Collections.emptyList();
    }

    private List<RadioStation> fetchDiaspora() throws Exception {
        final String[] queries = {
                "tag=diaspora", "tag=balkan", "tag=exyu",
                "name=balkan", "name=ex%20yu", "name=radio%20diaspora",
                "language=croatian", "language=serbian", "language=bosnian",
                "language=macedonian", "language=albanian", "language=slovenian",
                "language=bulgarian"
        };
        Exception last = null;
        for (String base : apiBases()) {
            if (closed) throw new InterruptedException("zatvaranje");
            List<RadioStation> out = new ArrayList<>();
            Exception baseFailure = null;
            for (String query : queries) {
                if (closed || Thread.currentThread().isInterrupted()) throw new InterruptedException("zatvaranje");
                try {
                    String endpoint = base + "/json/stations/search?" + query
                            + "&hidebroken=true&order=votes&reverse=true&limit=" + DIASPORA_QUERY_LIMIT;
                    JSONArray rows = new JSONArray(get(endpoint, 3 * 1024 * 1024));
                    for (int i = 0; i < rows.length(); i++) {
                        JSONObject object = rows.optJSONObject(i);
                        if (object == null) continue;
                        RadioStation station = RadioStation.fromJson(object);
                        String originalCode = safe(station.countryCode).toUpperCase(Locale.ROOT);
                        if (originalCode.isEmpty() || isRegionalCountry(originalCode) || station.lastCheckOk != 1) continue;
                        if (!StreamResolver.isSafeHttp(station.url) && !StreamResolver.isSafeHttp(station.urlResolved)) continue;
                        station.sourceCountryCode = originalCode;
                        station.countryCode = DIASPORA_CODE;
                        station.tags = safe(station.tags).isEmpty() ? "dijaspora" : "dijaspora," + station.tags;
                        if (safe(station.country).isEmpty()) station.country = "Dijaspora";
                        station.refreshIndexes();
                        out.add(station);
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    throw e;
                } catch (Exception e) {
                    baseFailure = e;
                    AppLog.e(context, "diaspora-query-partial", e);
                }
            }
            out = dedupe(out);
            out.sort(Comparator.comparingInt((RadioStation station) -> station.votes).reversed()
                    .thenComparing(station -> RadioStation.fold(station.name)));
            if (out.size() > MAX_DIASPORA) out = new ArrayList<>(out.subList(0, MAX_DIASPORA));
            if (!out.isEmpty()) return out;
            if (baseFailure != null) last = baseFailure;
        }
        if (last != null) throw last;
        return Collections.emptyList();
    }

    private List<RadioStation> mergeMissingGroups(List<RadioStation> online, List<RadioStation> cached) {
        Set<String> present = new LinkedHashSet<>();
        CatalogAccumulator merged = new CatalogAccumulator();
        if (online != null) {
            for (RadioStation station : online) {
                if (!isUsable(station)) continue;
                present.add(safe(station.countryCode).toUpperCase(Locale.ROOT));
                merged.add(station);
            }
        }
        if (cached != null) {
            for (RadioStation station : cached) {
                if (!isUsable(station)) continue;
                String code = safe(station.countryCode).toUpperCase(Locale.ROOT);
                if (!present.contains(code)) merged.add(station);
            }
        }
        return sortCatalog(merged.values());
    }

    private static List<RadioStation> sortCatalog(List<RadioStation> input) {
        List<RadioStation> regional = new ArrayList<>();
        List<RadioStation> diaspora = new ArrayList<>();
        List<RadioStation> foreign = new ArrayList<>();
        for (RadioStation station : input) {
            if (!isUsable(station)) continue;
            if (FOREIGN_CODE.equalsIgnoreCase(station.countryCode)) foreign.add(station);
            else if (DIASPORA_CODE.equalsIgnoreCase(station.countryCode)) diaspora.add(station);
            else regional.add(station);
        }
        regional.sort(Comparator.comparingInt((RadioStation station) -> countryPriority(station.countryCode))
                .thenComparing(Comparator.comparingInt((RadioStation station) -> station.votes).reversed())
                .thenComparing(station -> RadioStation.fold(station.name)));
        diaspora.sort(Comparator.comparingInt((RadioStation station) -> station.votes).reversed()
                .thenComparing(station -> RadioStation.fold(station.name)));
        foreign.sort(Comparator.comparingInt((RadioStation station) -> station.votes).reversed()
                .thenComparing(station -> RadioStation.fold(station.name)));
        int regionalLimit = MAX_CATALOG - MAX_FOREIGN - MAX_DIASPORA;
        if (regional.size() > regionalLimit) regional = new ArrayList<>(regional.subList(0, regionalLimit));
        if (diaspora.size() > MAX_DIASPORA) diaspora = new ArrayList<>(diaspora.subList(0, MAX_DIASPORA));
        if (foreign.size() > MAX_FOREIGN) foreign = new ArrayList<>(foreign.subList(0, MAX_FOREIGN));
        regional.addAll(diaspora);
        regional.addAll(foreign);
        return regional;
    }

    private static int countRegional(List<RadioStation> stations) {
        int count = 0;
        for (RadioStation station : stations) if (station != null && isRegionalCountry(station.countryCode)) count++;
        return count;
    }

    private List<String> apiBases() {
        List<String> current = bases;
        if (current != null && !current.isEmpty()) return current;
        synchronized (this) {
            if (bases != null && !bases.isEmpty()) return bases;
            LinkedHashSet<String> found = new LinkedHashSet<>();
            try {
                JSONArray rows = new JSONArray(get("https://all.api.radio-browser.info/json/servers", 512 * 1024));
                for (int i = 0; i < rows.length() && found.size() < 8; i++) {
                    JSONObject object = rows.optJSONObject(i);
                    String name = object == null ? "" : object.optString("name", "").trim();
                    if (!name.isEmpty()) {
                        String candidate = "https://" + name;
                        if (isTrustedApiUrl(candidate)) found.add(candidate);
                    }
                }
            } catch (Throwable ignored) { }
            Collections.addAll(found, FALLBACK_BASES);
            bases = new ArrayList<>(found);
            return bases;
        }
    }

    private String get(String raw, int maxBytes) throws Exception {
        String current = raw;
        for (int hop = 0; hop <= MAX_REDIRECTS; hop++) {
            if (!isTrustedApiUrl(current)) throw new IllegalArgumentException("Nedopušten API URL");
            if (!StreamResolver.isSafeHttpForConnection(current)) throw new IllegalArgumentException("Nesigurno API mrežno odredište");
            HttpURLConnection connection = (HttpURLConnection) new URL(current).openConnection();
            connection.setConnectTimeout(7000);
            connection.setReadTimeout(14000);
            connection.setRequestProperty("User-Agent", AppInfo.USER_AGENT);
            connection.setRequestProperty("Accept", "application/json");
            connection.setInstanceFollowRedirects(false);
            connection.setUseCaches(false);
            try {
                int code = connection.getResponseCode();
                if (code >= 300 && code < 400) {
                    String location = connection.getHeaderField("Location");
                    if (location == null || location.trim().isEmpty()) throw new IllegalStateException("API preusmjeravanje bez lokacije");
                    current = new URL(connection.getURL(), location).toString();
                    if (!isTrustedApiUrl(current)) throw new IllegalStateException("Nedopušteno API preusmjeravanje");
                    continue;
                }
                if (code < 200 || code >= 300) throw new IllegalStateException("HTTP " + code);
                try (BufferedInputStream in = new BufferedInputStream(connection.getInputStream());
                     ByteArrayOutputStream out = new ByteArrayOutputStream()) {
                    byte[] buffer = new byte[16384];
                    int n;
                    while ((n = in.read(buffer)) > 0) {
                        int remaining = maxBytes - out.size();
                        if (remaining <= 0 || n > remaining) throw new IllegalStateException("Odgovor je prevelik");
                        out.write(buffer, 0, n);
                    }
                    return out.toString(StandardCharsets.UTF_8.name());
                }
            } finally {
                connection.disconnect();
            }
        }
        throw new IllegalStateException("Previše API preusmjeravanja");
    }

    private static boolean isTrustedApiUrl(String raw) {
        try {
            URL url = new URL(raw);
            if (!"https".equalsIgnoreCase(url.getProtocol())) return false;
            if (url.getUserInfo() != null) return false;
            String host = url.getHost() == null ? "" : url.getHost().toLowerCase(Locale.ROOT).replaceAll("\\.+$", "");
            return host.equals("api.radio-browser.info") || host.endsWith(".api.radio-browser.info");
        } catch (Throwable ignored) { return false; }
    }

    private File cacheFile() { return new File(context.getFilesDir(), "stations-cache.json"); }
    private File backupFile() { return new File(context.getFilesDir(), "stations-cache.json.bak"); }

    private void saveCache(List<RadioStation> list) {
        list = sortCatalog(dedupe(list));
        try {
            File target = cacheFile();
            File backup = backupFile();
            File tmp = new File(target.getAbsolutePath() + ".tmp");
            try (FileOutputStream raw = new FileOutputStream(tmp);
                 BufferedOutputStream out = new BufferedOutputStream(raw, 64 * 1024);
                 BufferedWriter writer = new BufferedWriter(new OutputStreamWriter(out, StandardCharsets.UTF_8), 64 * 1024)) {
                writer.write('[');
                boolean first = true;
                for (RadioStation station : list) {
                    if (station == null) continue;
                    if (!first) writer.write(',');
                    writer.write(station.toJson().toString());
                    first = false;
                }
                writer.write(']');
                writer.flush();
                out.flush();
                raw.getFD().sync();
            }
            if (backup.exists() && !backup.delete()) AppLog.e(context, "cache-backup-delete", new IllegalStateException("backup delete"));
            if (target.exists() && !target.renameTo(backup)) {
                tmp.delete();
                throw new IllegalStateException("Ne mogu pripremiti backup kataloga");
            }
            if (!tmp.renameTo(target)) {
                if (backup.exists()) backup.renameTo(target);
                throw new IllegalStateException("Ne mogu spremiti katalog");
            }
        } catch (Throwable t) {
            AppLog.e(context, "cache-save", t);
        }
    }

    private List<RadioStation> loadCache() {
        List<RadioStation> primary = readCache(cacheFile());
        return !primary.isEmpty() ? primary : readCache(backupFile());
    }

    private List<RadioStation> readCache(File file) {
        if (!file.exists() || file.length() > 16L * 1024L * 1024L) return Collections.emptyList();
        List<RadioStation> list = new ArrayList<>(Math.min(2048, MAX_CATALOG));
        try (BufferedInputStream in = new BufferedInputStream(new FileInputStream(file), 64 * 1024);
             JsonReader reader = new JsonReader(new InputStreamReader(in, StandardCharsets.UTF_8))) {
            reader.beginArray();
            while (reader.hasNext() && list.size() < MAX_CATALOG) {
                RadioStation station = readStation(reader);
                if (station != null && isUsable(station)) list.add(station);
            }
            while (reader.hasNext()) reader.skipValue();
            reader.endArray();
            return sortCatalog(dedupe(list));
        } catch (Throwable t) {
            AppLog.e(context, "cache-load", t);
            return Collections.emptyList();
        }
    }

    private static RadioStation readStation(JsonReader reader) throws Exception {
        if (reader.peek() == JsonToken.NULL) { reader.nextNull(); return null; }
        JSONObject object = new JSONObject();
        reader.beginObject();
        while (reader.hasNext()) {
            String name = reader.nextName();
            switch (name) {
                case "stationuuid": case "name": case "url": case "url_resolved": case "homepage":
                case "favicon": case "tags": case "country": case "countrycode": case "state": case "language": case "codec":
                    if (reader.peek() == JsonToken.NULL) reader.nextNull();
                    else object.put(name, reader.nextString());
                    break;
                case "votes": case "bitrate": case "lastcheckok":
                    if (reader.peek() == JsonToken.NULL) { reader.nextNull(); object.put(name, 0); }
                    else {
                        String raw = reader.nextString();
                        try { object.put(name, Integer.parseInt(raw)); }
                        catch (NumberFormatException ignored) { object.put(name, 0); }
                    }
                    break;
                default:
                    reader.skipValue();
            }
        }
        reader.endObject();
        return RadioStation.fromJson(object);
    }

    private static List<RadioStation> dedupe(List<RadioStation> list) {
        CatalogAccumulator unique = new CatalogAccumulator();
        if (list != null) for (RadioStation station : list) unique.add(station);
        return unique.values();
    }

    private static int countryPriority(String code) {
        for (int i = 1; i < COUNTRIES.length; i++) if (COUNTRIES[i][0].equalsIgnoreCase(code)) return i;
        return COUNTRIES.length + 1;
    }

    public static String countryName(String code) {
        for (String[] country : COUNTRIES) if (country[0].equalsIgnoreCase(code)) return country[1];
        return code;
    }

    private void safeCached(Listener listener, List<RadioStation> value) {
        try { listener.onCached(value); } catch (Throwable t) { AppLog.e(context, "listener-cache", t); }
    }
    private void safeLoaded(Listener listener, List<RadioStation> value) {
        try { listener.onLoaded(value); } catch (Throwable t) { AppLog.e(context, "listener-load", t); }
    }
    private void safeError(Listener listener, Throwable error, boolean hasCache) {
        try { listener.onError(error, hasCache); } catch (Throwable t) { AppLog.e(context, "listener-error", t); }
    }
}
