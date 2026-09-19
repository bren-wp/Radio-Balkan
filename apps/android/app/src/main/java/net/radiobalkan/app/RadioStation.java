package net.radiobalkan.app;

import org.json.JSONObject;
import java.text.Normalizer;
import java.util.Locale;
import java.net.URL;
import java.util.regex.Pattern;

public final class RadioStation {
    public String stationUuid = "";
    public String name = "";
    public String url = "";
    public String urlResolved = "";
    public String homepage = "";
    public String favicon = "";
    public String tags = "";
    public String country = "";
    public String countryCode = "";
    public String sourceCountryCode = "";
    public String state = "";
    public String language = "";
    public int votes;
    public String codec = "";
    public int bitrate;
    public int lastCheckOk;
    public String activeUrl = "";
    public String health = "unknown";
    public boolean replaced;
    public String searchIndex = "";

    private static final Pattern MARKS = Pattern.compile("\\p{M}+");
    private static final Pattern WHITESPACE = Pattern.compile("\\s+");
    private static final Pattern CONTROL = Pattern.compile("[\\p{Cntrl}&&[^\\r\\n\\t]]");

    public static RadioStation fromJson(JSONObject o) {
        RadioStation s = new RadioStation();
        s.stationUuid = clean(o.optString("stationuuid", ""));
        s.name = clean(o.optString("name", ""));
        s.url = clean(o.optString("url", ""));
        s.urlResolved = clean(o.optString("url_resolved", ""));
        s.homepage = clean(o.optString("homepage", ""));
        s.favicon = clean(o.optString("favicon", ""));
        s.tags = clean(o.optString("tags", ""));
        s.country = clean(o.optString("country", ""));
        s.countryCode = clean(o.optString("countrycode", "")).toUpperCase(Locale.ROOT);
        s.sourceCountryCode = clean(o.optString("sourcecountrycode", "")).toUpperCase(Locale.ROOT);
        s.state = clean(o.optString("state", ""));
        s.language = clean(o.optString("language", ""));
        s.votes = Math.max(0, o.optInt("votes", 0));
        s.codec = clean(o.optString("codec", ""));
        s.bitrate = Math.max(0, o.optInt("bitrate", 0));
        s.lastCheckOk = o.optInt("lastcheckok", 0);
        s.activeUrl = !s.urlResolved.isEmpty() ? s.urlResolved : s.url;
        s.health = s.lastCheckOk == 1 ? "ok" : "unknown";
        if (s.favicon.isEmpty()) s.favicon = websiteIcon(s.homepage);
        s.refreshIndexes();
        return s;
    }

    public JSONObject toJson() {
        JSONObject o = new JSONObject();
        try {
            o.put("stationuuid", stationUuid);
            o.put("name", name);
            o.put("url", url);
            o.put("url_resolved", urlResolved);
            o.put("homepage", homepage);
            o.put("favicon", favicon);
            o.put("tags", tags);
            o.put("country", country);
            o.put("countrycode", countryCode);
            o.put("sourcecountrycode", sourceCountryCode);
            o.put("state", state);
            o.put("language", language);
            o.put("votes", votes);
            o.put("codec", codec);
            o.put("bitrate", bitrate);
            o.put("lastcheckok", lastCheckOk);
        } catch (Exception ignored) { }
        return o;
    }

    public void refreshIndexes() {
        searchIndex = fold(name + " " + tags + " " + state + " " + country + " " + language + " " + codec);
    }

    public String key() {
        if (!stationUuid.isEmpty()) return stationUuid;
        String base = countryCode + "|" + fold(name) + "|" + (!urlResolved.isEmpty() ? urlResolved : url);
        return base.length() > 512 ? base.substring(0, 512) : base;
    }

    public String flagCode() {
        return sourceCountryCode == null || sourceCountryCode.trim().isEmpty() ? countryCode : sourceCountryCode;
    }

    public String meta() {
        StringBuilder b = new StringBuilder();
        if (!country.isEmpty()) b.append(country);
        if (!state.isEmpty() && !state.equalsIgnoreCase(country)) append(b, state);
        if (!codec.isEmpty()) append(b, codec.toUpperCase(Locale.ROOT));
        if (bitrate > 0) append(b, bitrate + " kbps");
        return b.length() == 0 ? "Radio uživo" : b.toString();
    }

    private static void append(StringBuilder b, String v) {
        if (b.length() > 0) b.append(" · ");
        b.append(v);
    }

    public static String fold(String v) {
        if (v == null) return "";
        String n = Normalizer.normalize(v.toLowerCase(Locale.ROOT), Normalizer.Form.NFD)
                .replace('đ', 'd').replace('ð', 'd').replace('ı', 'i');
        n = MARKS.matcher(n).replaceAll("");
        return WHITESPACE.matcher(n).replaceAll(" ").trim();
    }

    private static String clean(String v) {
        if (v == null) return "";
        String s = CONTROL.matcher(v.replace('\u0000', ' ')).replaceAll(" ");
        s = WHITESPACE.matcher(s).replaceAll(" ").trim();
        return s.length() > 2048 ? s.substring(0, 2048) : s;
    }

    public static String websiteIcon(String homepage) {
        try {
            if (homepage == null || homepage.trim().isEmpty()) return "";
            URL u = new URL(homepage.trim());
            String protocol = u.getProtocol();
            if (!("http".equalsIgnoreCase(protocol) || "https".equalsIgnoreCase(protocol))) return "";
            if (u.getHost() == null || u.getHost().trim().isEmpty()) return "";
            int port = u.getPort();
            return protocol.toLowerCase(Locale.ROOT) + "://" + u.getHost() + (port > 0 ? ":" + port : "") + "/favicon.ico";
        } catch (Throwable ignored) { return ""; }
    }
}
