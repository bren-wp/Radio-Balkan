package net.radiobalkan.app;

import org.json.JSONArray;
import org.json.JSONObject;
import java.io.BufferedInputStream;
import java.io.ByteArrayOutputStream;
import java.net.HttpURLConnection;
import java.net.InetAddress;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Locale;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public final class StreamResolver {
    private static final String USER_AGENT = AppInfo.USER_AGENT;
    private static final int MAX_REDIRECTS = 4;
    private static final String[] BASES = {
            "https://all.api.radio-browser.info",
            "https://de1.api.radio-browser.info",
            "https://de2.api.radio-browser.info",
            "https://at1.api.radio-browser.info",
            "https://nl1.api.radio-browser.info"
    };
    private static final Pattern ABSOLUTE_URL = Pattern.compile("https?://[^\\s\\\"'<>]+", Pattern.CASE_INSENSITIVE);
    private static final Pattern ATTR_URL = Pattern.compile("(?i)(?:href|src)\\s*=\\s*[\\\"']([^\\\"']+)[\\\"']");

    public static final class Resolution {
        public final String url;
        public final String source;
        Resolution(String url, String source) { this.url = url; this.source = source; }
    }

    private StreamResolver() { }

    public static String resolveFirst(List<String> candidates) {
        Resolution r = resolveCandidates(candidates);
        return r == null ? null : r.url;
    }

    public static Resolution resolveCandidates(List<String> candidates) {
        Set<String> unique = new LinkedHashSet<>();
        if (candidates != null) for (String s : candidates) if (isSafeHttp(s)) unique.add(s.trim());
        for (String candidate : unique) {
            String resolved = probe(candidate, 0);
            if (resolved != null && !resolved.isEmpty()) return new Resolution(resolved, "candidate");
        }
        return null;
    }

    public static Resolution repair(List<String> candidates, String stationUuid, String homepage, String countryCode) {
        Resolution direct = resolveCandidates(candidates);
        if (direct != null) return direct;
        for (String refreshed : refreshByUuid(stationUuid, countryCode)) {
            String resolved = probe(refreshed, 0);
            if (resolved != null) return new Resolution(resolved, "catalog");
        }
        for (String discovered : discoverFromHomepage(homepage)) {
            String resolved = probe(discovered, 0);
            if (resolved != null) return new Resolution(resolved, "website");
        }
        return null;
    }

    private static String probe(String raw, int depth) {
        if (depth > MAX_REDIRECTS || !isSafeHttp(raw)) return null;
        HttpURLConnection c = null;
        try {
            URL original = new URL(raw);
            c = (HttpURLConnection) original.openConnection();
            c.setConnectTimeout(6500);
            c.setReadTimeout(6500);
            c.setInstanceFollowRedirects(false);
            c.setUseCaches(false);
            c.setRequestProperty("User-Agent", USER_AGENT);
            c.setRequestProperty("Icy-MetaData", "0");
            c.setRequestProperty("Accept", "audio/*,application/ogg,application/vnd.apple.mpegurl,application/x-mpegURL,*/*;q=0.5");
            int status = c.getResponseCode();
            if (isRedirect(status)) {
                String location = safe(c.getHeaderField("Location"));
                if (location.isEmpty()) return null;
                URL redirected = new URL(original, location);
                String next = redirected.toString();
                return isSafeHttp(next) ? probe(next, depth + 1) : null;
            }
            if (status < 200 || status >= 300) return null;
            String finalUrl = c.getURL().toString();
            if (!isSafeHttp(finalUrl)) return null;
            String contentType = safe(c.getContentType()).toLowerCase(Locale.ROOT);
            if (isHls(finalUrl, contentType)) return finalUrl;
            if (looksPlaylist(finalUrl, contentType)) {
                String body = readLimited(c, 192 * 1024);
                String next = firstPlaylistUrl(body, c.getURL());
                return next == null ? null : probe(next, depth + 1);
            }
            if (contentType.contains("text/html")) return null;
            try (BufferedInputStream in = new BufferedInputStream(c.getInputStream())) {
                byte[] probe = new byte[768];
                int n = in.read(probe);
                if (n <= 0) return null;
            }
            return finalUrl;
        } catch (Throwable ignored) {
            return null;
        } finally {
            if (c != null) c.disconnect();
        }
    }

    private static boolean isRedirect(int status) {
        return status == HttpURLConnection.HTTP_MULT_CHOICE
                || status == HttpURLConnection.HTTP_MOVED_PERM
                || status == HttpURLConnection.HTTP_MOVED_TEMP
                || status == HttpURLConnection.HTTP_SEE_OTHER
                || status == 307
                || status == 308;
    }

    private static boolean isHls(String u, String ct) {
        String x = safe(u).toLowerCase(Locale.ROOT);
        return x.contains(".m3u8") || ct.contains("vnd.apple.mpegurl") || ct.contains("x-mpegurl") && x.contains("m3u8");
    }

    private static boolean looksPlaylist(String u, String ct) {
        String x = safe(u).toLowerCase(Locale.ROOT).split("\\?", 2)[0];
        return x.endsWith(".m3u") || x.endsWith(".pls") || x.endsWith(".asx")
                || ct.contains("scpls") || ct.contains("asx") || (ct.contains("mpegurl") && !x.endsWith(".m3u8"));
    }

    private static String firstPlaylistUrl(String body, URL base) {
        if (body == null) return null;
        for (String line : body.replace("\r", "\n").split("\n")) {
            String s = line.trim();
            if (s.isEmpty() || s.startsWith("#")) continue;
            int eq = s.indexOf('=');
            if (eq > 0) {
                String left = s.substring(0, eq).toLowerCase(Locale.ROOT);
                if (left.startsWith("file") || left.contains("ref")) s = s.substring(eq + 1).trim();
            }
            s = s.replace("&amp;", "&").replace("\"", "").replace("'", "").trim();
            int http = s.toLowerCase(Locale.ROOT).indexOf("http");
            if (http >= 0) s = s.substring(http).split("[<> ]", 2)[0];
            try {
                URL resolved = isHttp(s) ? new URL(s) : new URL(base, s);
                if (isSafeHttp(resolved.toString())) return resolved.toString();
            } catch (Throwable ignored) { }
        }
        Matcher m = ABSOLUTE_URL.matcher(body);
        if (m.find()) {
            String candidate = m.group();
            return isSafeHttp(candidate) ? candidate : null;
        }
        return null;
    }

    public static List<String> candidates(RadioStation s, StateStore store) {
        List<String> out = new ArrayList<>();
        if (s == null) return out;
        String key = s.key();
        String manual = store.manualReplacement(key);
        String automatic = store.autoReplacement(key);
        if (!manual.isEmpty()) out.add(manual);
        if (!automatic.isEmpty()) out.add(automatic);
        out.addAll(store.backups(key));
        if (!s.activeUrl.isEmpty()) out.add(s.activeUrl);
        if (!s.urlResolved.isEmpty()) out.add(s.urlResolved);
        if (!s.url.isEmpty()) out.add(s.url);
        return unique(out, 16);
    }

    public static List<String> refreshByUuid(String stationUuid, String countryCode) {
        List<String> out = new ArrayList<>();
        stationUuid = safe(stationUuid);
        countryCode = safe(countryCode).toUpperCase(Locale.ROOT);
        if (stationUuid.isEmpty() || !RadioRepository.isSupportedCountry(countryCode)) return out;
        for (String base : BASES) {
            HttpURLConnection c = null;
            try {
                URL u = new URL(base + "/json/stations/byuuid/" + stationUuid);
                c = (HttpURLConnection) u.openConnection();
                c.setConnectTimeout(5000); c.setReadTimeout(7000); c.setUseCaches(false); c.setInstanceFollowRedirects(false);
                c.setRequestProperty("User-Agent", USER_AGENT); c.setRequestProperty("Accept", "application/json");
                int status = c.getResponseCode();
                if (status < 200 || status >= 300) continue;
                String json = readLimited(c, 512 * 1024);
                JSONArray a = new JSONArray(json);
                for (int i = 0; i < a.length(); i++) {
                    JSONObject o = a.optJSONObject(i);
                    if (o == null) continue;
                    String actualCountry = safe(o.optString("countrycode", "")).toUpperCase(Locale.ROOT);
                    if (!countryCode.equals(actualCountry)) continue;
                    String r = safe(o.optString("url_resolved", ""));
                    String raw = safe(o.optString("url", ""));
                    if (isSafeHttp(r)) out.add(r);
                    if (isSafeHttp(raw)) out.add(raw);
                }
                if (!out.isEmpty()) break;
            } catch (Throwable ignored) { }
            finally { if (c != null) c.disconnect(); }
        }
        return unique(out, 8);
    }

    public static List<String> discoverFromHomepage(String homepage) {
        return discoverFromHomepage(homepage, 0);
    }

    private static List<String> discoverFromHomepage(String homepage, int depth) {
        List<String> out = new ArrayList<>();
        if (depth > MAX_REDIRECTS || !isSafeHttp(homepage)) return out;
        HttpURLConnection c = null;
        try {
            URL requested = new URL(homepage);
            c = (HttpURLConnection) requested.openConnection();
            c.setConnectTimeout(6000); c.setReadTimeout(8000); c.setUseCaches(false); c.setInstanceFollowRedirects(false);
            c.setRequestProperty("User-Agent", USER_AGENT); c.setRequestProperty("Accept", "text/html,*/*;q=0.5");
            int status = c.getResponseCode();
            if (isRedirect(status)) {
                String location = safe(c.getHeaderField("Location"));
                if (location.isEmpty()) return out;
                String next = new URL(requested, location).toString();
                return isSafeHttp(next) ? discoverFromHomepage(next, depth + 1) : out;
            }
            if (status < 200 || status >= 300) return out;
            String body = readLimited(c, 1024 * 1024).replace("\\/", "/").replace("&amp;", "&");
            URL base = c.getURL();
            if (!isSafeHttp(base.toString())) return out;
            Matcher abs = ABSOLUTE_URL.matcher(body);
            while (abs.find() && out.size() < 48) addIfStreamLike(out, abs.group(), base);
            Matcher attr = ATTR_URL.matcher(body);
            while (attr.find() && out.size() < 48) addIfStreamLike(out, attr.group(1), base);
        } catch (Throwable ignored) { }
        finally { if (c != null) c.disconnect(); }
        return unique(out, 24);
    }

    private static void addIfStreamLike(List<String> out, String raw, URL base) {
        try {
            URL u = isHttp(raw) ? new URL(raw) : new URL(base, raw);
            String v = u.toString();
            if (!isSafeHttp(v)) return;
            String l = v.toLowerCase(Locale.ROOT);
            if (l.contains(".mp3") || l.contains(".aac") || l.contains(".ogg") || l.contains(".opus") || l.contains(".m3u")
                    || l.contains(".pls") || l.contains(".asx") || l.contains("stream") || l.contains("listen")
                    || l.contains("icecast") || l.contains("shoutcast")) out.add(v);
        } catch (Throwable ignored) { }
    }

    private static String readLimited(HttpURLConnection c, int limit) throws Exception {
        try (BufferedInputStream in = new BufferedInputStream(c.getInputStream()); ByteArrayOutputStream out = new ByteArrayOutputStream()) {
            byte[] buf = new byte[8192];
            int n;
            while ((n = in.read(buf)) > 0) {
                int remaining = limit - out.size();
                if (remaining <= 0) break;
                out.write(buf, 0, Math.min(n, remaining));
            }
            return out.toString(StandardCharsets.UTF_8.name());
        }
    }

    private static List<String> unique(List<String> values, int max) {
        LinkedHashSet<String> set = new LinkedHashSet<>();
        if (values != null) for (String v : values) if (isSafeHttp(v)) { set.add(v.trim()); if (set.size() >= max) break; }
        return new ArrayList<>(set);
    }

    public static boolean isHttp(String s) {
        if (s == null) return false;
        String v = s.trim().toLowerCase(Locale.ROOT);
        return v.startsWith("http://") || v.startsWith("https://");
    }

    /** Reject local/private literal targets while still allowing public HTTP radio streams. */
    public static boolean isSafeHttp(String s) {
        if (!isHttp(s) || s.length() > 4096) return false;
        try {
            URL u = new URL(s.trim());
            String host = canonicalHost(u.getHost());
            if (host.isEmpty() || host.equals("localhost") || host.endsWith(".localhost") || host.endsWith(".local")
                    || host.equals("metadata.google.internal") || host.equals("instance-data.ec2.internal") || host.equals("metadata.azure.internal")) return false;
            if (u.getUserInfo() != null) return false;
            // Avoid DNS lookups in the hot path. Check literal IP hosts only.
            boolean numeric = host.matches("^[0-9.]+$") || host.contains(":");
            if (numeric) {
                InetAddress ip = InetAddress.getByName(host);
                if (isUnsafeAddress(ip)) return false;
            }
            return true;
        } catch (Throwable ignored) { return false; }
    }

    private static String canonicalHost(String raw) {
        String host = safe(raw).toLowerCase(Locale.ROOT);
        while (host.endsWith(".")) host = host.substring(0, host.length() - 1);
        return host;
    }

    private static boolean isUnsafeAddress(InetAddress ip) {
        if (ip == null || ip.isAnyLocalAddress() || ip.isLoopbackAddress() || ip.isLinkLocalAddress() || ip.isSiteLocalAddress() || ip.isMulticastAddress()) return true;
        byte[] a = ip.getAddress();
        if (a == null) return true;
        if (a.length == 4) {
            int b0 = a[0] & 0xff, b1 = a[1] & 0xff;
            return b0 == 0 || b0 == 10 || b0 == 127 || b0 >= 224
                    || (b0 == 100 && b1 >= 64 && b1 <= 127)
                    || (b0 == 169 && b1 == 254)
                    || (b0 == 172 && b1 >= 16 && b1 <= 31)
                    || (b0 == 192 && b1 == 168);
        }
        if (a.length == 16) {
            int b0 = a[0] & 0xff, b1 = a[1] & 0xff;
            return (b0 & 0xfe) == 0xfc || (b0 == 0xfe && (b1 & 0xc0) == 0x80) || b0 == 0xff;
        }
        return true;
    }

    private static String safe(String v) { return v == null ? "" : v.trim(); }
}
