package net.radiobalkan.app;

import android.content.Context;
import android.util.JsonReader;
import android.util.JsonToken;
import org.json.JSONArray;
import org.json.JSONObject;
import java.io.BufferedInputStream;
import java.io.BufferedOutputStream;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.OutputStreamWriter;
import java.io.InputStreamReader;
import java.io.BufferedWriter;
import java.net.HttpURLConnection;
import java.net.URL;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.CompletionService;
import java.util.concurrent.ExecutorCompletionService;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public final class RadioRepository {
    public interface Listener {
        void onCached(List<RadioStation> stations);
        void onLoaded(List<RadioStation> stations);
        void onError(Throwable error, boolean hasCache);
    }

    public static final String[][] COUNTRIES = {
            {"", "Sve podržane zemlje"}, {"HR", "Hrvatska"}, {"BA", "Bosna i Hercegovina"},
            {"RS", "Srbija"}, {"SI", "Slovenija"}, {"MK", "Sjeverna Makedonija"},
            {"AL", "Albanija"}, {"ME", "Crna Gora"}
    };

    private static final String[] FALLBACK_BASES = {
            "https://de1.api.radio-browser.info", "https://de2.api.radio-browser.info",
            "https://at1.api.radio-browser.info", "https://nl1.api.radio-browser.info"
    };
    private static final int PAGE = 250;
    private static final int MAX_PER_COUNTRY = 1800;
    private static final int MAX_CATALOG = 7000;
    private static final int PRODUCTION_MIN_STATIONS = 600;
    private final Context context;
    private final ExecutorService worker = Executors.newSingleThreadExecutor(r -> { Thread t = new Thread(r, "radio-catalog"); t.setPriority(Thread.NORM_PRIORITY - 1); return t; });
    private final ExecutorService countryPool = Executors.newFixedThreadPool(2, r -> { Thread t = new Thread(r, "radio-country"); t.setPriority(Thread.MIN_PRIORITY); return t; });
    private volatile boolean closed;
    private volatile List<String> bases;

    public RadioRepository(Context context) {
        this.context = context.getApplicationContext();
    }

    public void load(Listener listener) {
        worker.execute(() -> {
            List<RadioStation> cached = loadCache();
            if (closed) return;
            if (!cached.isEmpty()) safeCached(listener, cached);
            try {
                List<RadioStation> online = mergeMissingCountries(fetchAll(), cached);
                if (online.size() < PRODUCTION_MIN_STATIONS && !cached.isEmpty()) {
                    List<RadioStation> combined = new ArrayList<>(online);
                    combined.addAll(cached);
                    online = trimCatalog(dedupe(combined));
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
    }

    public void shutdown() {
        closed = true;
        worker.shutdownNow();
        countryPool.shutdownNow();
    }

    private List<RadioStation> fetchAll() throws Exception {
        // CompletionService consumes each country as soon as it finishes, avoiding a
        // large pile of completed Future results when thousands of stations are loaded.
        CompletionService<List<RadioStation>> completion = new ExecutorCompletionService<>(countryPool);
        int tasks = 0;
        for (String[] c : COUNTRIES) {
            if (c[0].isEmpty()) continue;
            String code = c[0];
            completion.submit(() -> fetchCountry(code));
            tasks++;
        }
        CatalogAccumulator unique = new CatalogAccumulator();
        Throwable first = null;
        for (int n = 0; n < tasks; n++) {
            if (closed || Thread.currentThread().isInterrupted()) break;
            try {
                List<RadioStation> batch = completion.take().get();
                for (RadioStation st : batch) unique.add(st);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                throw e;
            } catch (Throwable t) {
                if (first == null) first = t;
                AppLog.e(context, "catalog-partial", t);
            }
        }
        if (unique.size() == 0 && first != null) throw new Exception(first);
        List<RadioStation> out = unique.values();
        out.sort(Comparator.comparingInt((RadioStation st) -> countryPriority(st.countryCode))
                .thenComparing(Comparator.comparingInt((RadioStation st) -> st.votes).reversed())
                .thenComparing(st -> RadioStation.fold(st.name)));
        if (out.size() > MAX_CATALOG) out = new ArrayList<>(out.subList(0, MAX_CATALOG));
        return out;
    }

    private static final class CatalogAccumulator {
        private final Map<String, Integer> byUuid = new LinkedHashMap<>();
        private final Map<String, Integer> byIdentity = new LinkedHashMap<>();
        private final List<RadioStation> items = new ArrayList<>();

        int size() { return items.size(); }
        List<RadioStation> values() { return new ArrayList<>(items); }

        void add(RadioStation st) {
            if (!isUsable(st)) return;
            String uuid = safe(st.stationUuid);
            String identity = identity(st);
            Integer pos = uuid.isEmpty() ? null : byUuid.get(uuid);
            if (pos == null && !identity.isEmpty()) pos = byIdentity.get(identity);
            if (pos != null) {
                RadioStation merged = merge(items.get(pos), st);
                items.set(pos, merged);
                if (!safe(merged.stationUuid).isEmpty()) byUuid.put(merged.stationUuid, pos);
                String mergedIdentity = identity(merged);
                if (!mergedIdentity.isEmpty()) byIdentity.put(mergedIdentity, pos);
                return;
            }
            int next = items.size();
            items.add(st);
            if (!uuid.isEmpty()) byUuid.put(uuid, next);
            if (!identity.isEmpty()) byIdentity.put(identity, next);
        }
    }

    private static boolean isUsable(RadioStation st) {
        return st != null && !safe(st.name).isEmpty()
                && isSupportedCountry(st.countryCode)
                && (StreamResolver.isSafeHttp(st.url) || StreamResolver.isSafeHttp(st.urlResolved));
    }

    public static boolean isSupportedCountry(String code) {
        if (code == null) return false;
        String normalized = code.trim().toUpperCase(java.util.Locale.ROOT);
        if (normalized.isEmpty()) return false;
        for (int i = 1; i < COUNTRIES.length; i++) {
            if (COUNTRIES[i][0].equals(normalized)) return true;
        }
        return false;
    }

    private static String safe(String v) { return v == null ? "" : v.trim(); }

    private static int quality(RadioStation s) {
        int q = Math.max(0, s.votes);
        if (s.lastCheckOk == 1) q += 1_000_000;
        if (StreamResolver.isSafeHttp(s.favicon)) q += 20_000;
        if (StreamResolver.isSafeHttp(s.homepage)) q += 10_000;
        if (StreamResolver.isSafeHttp(s.urlResolved)) q += 5_000;
        q += Math.min(512, Math.max(0, s.bitrate));
        return q;
    }

    private static RadioStation merge(RadioStation a, RadioStation b) {
        RadioStation p = quality(b) > quality(a) ? b : a;
        RadioStation o = p == a ? b : a;
        if (safe(p.stationUuid).isEmpty()) p.stationUuid = o.stationUuid;
        if (safe(p.name).isEmpty()) p.name = o.name;
        if (safe(p.url).isEmpty()) p.url = o.url;
        if (safe(p.urlResolved).isEmpty()) p.urlResolved = o.urlResolved;
        if (safe(p.homepage).isEmpty()) p.homepage = o.homepage;
        if (safe(p.favicon).isEmpty()) p.favicon = !safe(o.favicon).isEmpty() ? o.favicon : RadioStation.websiteIcon(p.homepage);
        if (safe(p.tags).isEmpty()) p.tags = o.tags;
        if (safe(p.country).isEmpty()) p.country = o.country;
        if (safe(p.countryCode).isEmpty()) p.countryCode = o.countryCode;
        if (safe(p.state).isEmpty()) p.state = o.state;
        if (safe(p.language).isEmpty()) p.language = o.language;
        if (safe(p.codec).isEmpty()) p.codec = o.codec;
        if (p.bitrate <= 0) p.bitrate = o.bitrate;
        p.votes = Math.max(p.votes, o.votes);
        p.lastCheckOk = Math.max(p.lastCheckOk, o.lastCheckOk);
        p.refreshIndexes();
        return p;
    }

    private static String identity(RadioStation s) {
        String name = RadioStation.fold(safe(s.name)).replaceAll("[^\\p{L}\\p{Nd}]", "");
        String country = safe(s.countryCode).toUpperCase();
        if (name.isEmpty()) return "";
        String home = host(s.homepage);
        if (!home.isEmpty()) return country + "|" + name + "|home:" + home;
        String stream = !safe(s.urlResolved).isEmpty() ? s.urlResolved : s.url;
        try {
            URI u = URI.create(stream);
            String h = u.getHost() == null ? "" : u.getHost().toLowerCase();
            String path = u.getPath() == null ? "" : u.getPath().toLowerCase();
            if (!h.isEmpty()) return country + "|" + name + "|stream:" + h + path;
        } catch (Throwable ignored) { }
        return country + "|" + name;
    }

    private static String host(String raw) {
        try {
            URI u = URI.create(safe(raw));
            String h = u.getHost();
            if (h == null) return "";
            h = h.toLowerCase();
            return h.startsWith("www.") ? h.substring(4) : h;
        } catch (Throwable ignored) { return ""; }
    }

    private List<RadioStation> fetchCountry(String rawCode) throws Exception {
        String code = rawCode == null ? "" : rawCode.trim().toUpperCase(java.util.Locale.ROOT);
        if (!isSupportedCountry(code)) throw new IllegalArgumentException("Nepodržana država");
        Exception last = null;
        for (String base : apiBases()) {
            if (closed) throw new InterruptedException("zatvaranje");
            try {
                List<RadioStation> out = new ArrayList<>();
                for (int offset = 0; offset < MAX_PER_COUNTRY; offset += PAGE) {
                    if (closed || Thread.currentThread().isInterrupted()) throw new InterruptedException("zatvaranje");
                    String u = base + "/json/stations/search?countrycode=" + code
                            + "&hidebroken=true&order=votes&reverse=true&limit=" + PAGE + "&offset=" + offset;
                    JSONArray arr = new JSONArray(get(u, 6 * 1024 * 1024));
                    if (arr.length() == 0) break;
                    int accepted = 0;
                    for (int i = 0; i < arr.length(); i++) {
                        JSONObject o = arr.optJSONObject(i);
                        if (o == null) continue;
                        RadioStation station = RadioStation.fromJson(o);
                        String actual = safe(station.countryCode).toUpperCase(java.util.Locale.ROOT);
                        if (actual.isEmpty()) actual = code;
                        if (!code.equals(actual)) continue;
                        station.countryCode = code;
                        if (safe(station.country).isEmpty()) station.country = countryName(code);
                        station.refreshIndexes();
                        out.add(station);
                        accepted++;
                    }
                    if (arr.length() < PAGE || accepted == 0) break;
                }
                if (out.isEmpty()) {
                    String u = base + "/json/stations/bycountrycodeexact/" + code
                            + "?hidebroken=true&order=votes&reverse=true&limit=" + MAX_PER_COUNTRY;
                    JSONArray arr = new JSONArray(get(u, 10 * 1024 * 1024));
                    for (int i = 0; i < arr.length(); i++) {
                        JSONObject o = arr.optJSONObject(i);
                        if (o == null) continue;
                        RadioStation station = RadioStation.fromJson(o);
                        String actual = safe(station.countryCode).toUpperCase(java.util.Locale.ROOT);
                        if (actual.isEmpty()) actual = code;
                        if (!code.equals(actual)) continue;
                        station.countryCode = code;
                        if (safe(station.country).isEmpty()) station.country = countryName(code);
                        station.refreshIndexes();
                        out.add(station);
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

    private List<RadioStation> mergeMissingCountries(List<RadioStation> online, List<RadioStation> cached) {
        LinkedHashSet<String> present = new LinkedHashSet<>();
        CatalogAccumulator merged = new CatalogAccumulator();
        if (online != null) {
            for (RadioStation station : online) {
                if (!isUsable(station)) continue;
                present.add(station.countryCode.toUpperCase(java.util.Locale.ROOT));
                merged.add(station);
            }
        }
        if (cached != null) {
            for (RadioStation station : cached) {
                if (!isUsable(station)) continue;
                String code = station.countryCode.toUpperCase(java.util.Locale.ROOT);
                if (!present.contains(code)) merged.add(station);
            }
        }
        List<RadioStation> out = merged.values();
        out.sort(Comparator.comparingInt((RadioStation station) -> countryPriority(station.countryCode))
                .thenComparing(Comparator.comparingInt((RadioStation station) -> station.votes).reversed())
                .thenComparing(station -> RadioStation.fold(station.name)));
        return trimCatalog(out);
    }

    private List<String> apiBases() {
        List<String> current = bases;
        if (current != null && !current.isEmpty()) return current;
        synchronized (this) {
            if (bases != null && !bases.isEmpty()) return bases;
            LinkedHashSet<String> found = new LinkedHashSet<>();
            try {
                JSONArray a = new JSONArray(get("https://all.api.radio-browser.info/json/servers", 512 * 1024));
                for (int i = 0; i < a.length() && found.size() < 8; i++) {
                    JSONObject o = a.optJSONObject(i);
                    String name = o == null ? "" : o.optString("name", "").trim();
                    if (!name.isEmpty()) found.add("https://" + name);
                }
            } catch (Throwable ignored) { }
            Collections.addAll(found, FALLBACK_BASES);
            bases = new ArrayList<>(found);
            return bases;
        }
    }

    private String get(String raw, int maxBytes) throws Exception {
        if (!isTrustedApiUrl(raw)) throw new IllegalArgumentException("Nedopušten API URL");
        HttpURLConnection c = (HttpURLConnection) new URL(raw).openConnection();
        c.setConnectTimeout(7000);
        c.setReadTimeout(14000);
        c.setRequestProperty("User-Agent", AppInfo.USER_AGENT);
        c.setRequestProperty("Accept", "application/json");
        c.setInstanceFollowRedirects(true);
        c.setUseCaches(false);
        try {
            int code = c.getResponseCode();
            if (code < 200 || code >= 310) throw new IllegalStateException("HTTP " + code);
            if (!isTrustedApiUrl(c.getURL().toString())) throw new IllegalStateException("Nedopušteno API preusmjeravanje");
            try (BufferedInputStream in = new BufferedInputStream(c.getInputStream()); ByteArrayOutputStream out = new ByteArrayOutputStream()) {
                byte[] buf = new byte[16384];
                int n;
                while ((n = in.read(buf)) > 0) {
                    int remaining = maxBytes - out.size();
                    if (remaining <= 0) throw new IllegalStateException("Odgovor je prevelik");
                    out.write(buf, 0, Math.min(n, remaining));
                    if (n > remaining) throw new IllegalStateException("Odgovor je prevelik");
                }
                return out.toString(StandardCharsets.UTF_8.name());
            }
        } finally {
            c.disconnect();
        }
    }

    private static boolean isTrustedApiUrl(String raw) {
        try {
            URL u = new URL(raw);
            if (!"https".equalsIgnoreCase(u.getProtocol())) return false;
            String host = u.getHost() == null ? "" : u.getHost().toLowerCase();
            return host.equals("api.radio-browser.info") || host.endsWith(".api.radio-browser.info");
        } catch (Throwable ignored) { return false; }
    }

    private File cacheFile() { return new File(context.getFilesDir(), "stations-cache.json"); }
    private File backupFile() { return new File(context.getFilesDir(), "stations-cache.json.bak"); }

    private void saveCache(List<RadioStation> list) {
        list = trimCatalog(list);
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
        if (!primary.isEmpty()) return primary;
        return readCache(backupFile());
    }

    private List<RadioStation> readCache(File f) {
        if (!f.exists() || f.length() > 16L * 1024L * 1024L) return Collections.emptyList();
        List<RadioStation> list = new ArrayList<>(Math.min(2048, MAX_CATALOG));
        try (BufferedInputStream in = new BufferedInputStream(new FileInputStream(f), 64 * 1024);
             JsonReader reader = new JsonReader(new InputStreamReader(in, StandardCharsets.UTF_8))) {
            reader.beginArray();
            while (reader.hasNext() && list.size() < MAX_CATALOG) {
                RadioStation station = readStation(reader);
                if (station != null && isUsable(station)) list.add(station);
            }
            while (reader.hasNext()) reader.skipValue();
            reader.endArray();
            return trimCatalog(dedupe(list));
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
                case "favicon": case "tags": case "country": case "countrycode": case "state":
                case "language": case "codec":
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

    private static List<RadioStation> trimCatalog(List<RadioStation> list) {
        if (list == null || list.isEmpty()) return new ArrayList<>();
        List<RadioStation> filtered = new ArrayList<>(Math.min(list.size(), MAX_CATALOG));
        for (RadioStation station : list) {
            if (!isUsable(station)) continue;
            filtered.add(station);
            if (filtered.size() >= MAX_CATALOG) break;
        }
        return filtered;
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
        for (String[] c : COUNTRIES) if (c[0].equalsIgnoreCase(code)) return c[1];
        return code;
    }

    private void safeCached(Listener l, List<RadioStation> v) { try { l.onCached(v); } catch (Throwable t) { AppLog.e(context, "listener-cache", t); } }
    private void safeLoaded(Listener l, List<RadioStation> v) { try { l.onLoaded(v); } catch (Throwable t) { AppLog.e(context, "listener-load", t); } }
    private void safeError(Listener l, Throwable t, boolean hasCache) { try { l.onError(t, hasCache); } catch (Throwable e) { AppLog.e(context, "listener-error", e); } }
}
