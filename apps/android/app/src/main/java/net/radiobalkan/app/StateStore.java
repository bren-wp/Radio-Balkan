package net.radiobalkan.app;

import android.content.Context;
import android.content.SharedPreferences;
import org.json.JSONArray;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashSet;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;

public final class StateStore {
    private static final String PREFS = "radio_balkan_state";
    private static final int MAX_RECENT = 50;
    private static final int MAX_BACKUPS = 8;
    private final SharedPreferences prefs;

    public StateStore(Context context) {
        prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE);
    }

    public synchronized boolean isFavorite(String key) {
        return favorites().contains(cleanKey(key));
    }

    public synchronized boolean toggleFavorite(String key) {
        key = cleanKey(key);
        if (key.isEmpty()) return false;
        Set<String> f = favorites();
        boolean on;
        if (f.remove(key)) on = false;
        else { f.add(key); on = true; }
        prefs.edit().putStringSet("favorites", f).apply();
        return on;
    }

    public synchronized Set<String> favorites() {
        Set<String> raw = prefs.getStringSet("favorites", Collections.emptySet());
        return new HashSet<>(raw == null ? Collections.emptySet() : raw);
    }

    public synchronized void addRecent(String key) {
        key = cleanKey(key);
        if (key.isEmpty()) return;
        List<String> list = recent();
        list.remove(key);
        list.add(0, key);
        while (list.size() > MAX_RECENT) list.remove(list.size() - 1);
        prefs.edit().putString("recent", toJson(list)).apply();
    }

    public synchronized List<String> recent() {
        return readJsonList(prefs.getString("recent", "[]"), MAX_RECENT);
    }

    public synchronized String manualReplacement(String key) {
        key = cleanKey(key);
        if (key.isEmpty()) return "";
        String current = safe(prefs.getString("manual:" + key, ""));
        if (!current.isEmpty()) return current;
        // Migracija iz 2.4 i starijih verzija.
        String legacy = safe(prefs.getString("replacement:" + key, ""));
        if (!legacy.isEmpty()) {
            prefs.edit().putString("manual:" + key, legacy).remove("replacement:" + key).apply();
            return legacy;
        }
        return "";
    }

    public synchronized void setManualReplacement(String key, String url) {
        key = cleanKey(key);
        if (key.isEmpty()) return;
        SharedPreferences.Editor e = prefs.edit().remove("replacement:" + key);
        url = safe(url);
        if (url.isEmpty()) e.remove("manual:" + key);
        else if (StreamResolver.isSafeHttp(url)) e.putString("manual:" + key, url);
        e.apply();
    }

    public synchronized String autoReplacement(String key) {
        key = cleanKey(key);
        return key.isEmpty() ? "" : safe(prefs.getString("auto:" + key, ""));
    }

    public synchronized void setAutoReplacement(String key, String url) {
        key = cleanKey(key);
        if (key.isEmpty()) return;
        url = safe(url);
        SharedPreferences.Editor e = prefs.edit();
        if (url.isEmpty()) e.remove("auto:" + key);
        else if (StreamResolver.isSafeHttp(url)) e.putString("auto:" + key, url);
        e.apply();
    }

    public synchronized void clearAutomaticSources(String key) {
        key = cleanKey(key);
        if (key.isEmpty()) return;
        prefs.edit().remove("auto:" + key).remove("backups:" + key).apply();
    }

    public synchronized List<String> backups(String key) {
        key = cleanKey(key);
        if (key.isEmpty()) return Collections.emptyList();
        return readJsonList(prefs.getString("backups:" + key, "[]"), MAX_BACKUPS);
    }

    public synchronized void addBackup(String key, String url) {
        key = cleanKey(key);
        url = safe(url);
        if (key.isEmpty() || url.isEmpty() || !StreamResolver.isSafeHttp(url)) return;
        LinkedHashSet<String> unique = new LinkedHashSet<>();
        unique.add(url);
        unique.addAll(backups(key));
        List<String> list = new ArrayList<>(unique);
        while (list.size() > MAX_BACKUPS) list.remove(list.size() - 1);
        prefs.edit().putString("backups:" + key, toJson(list)).apply();
    }

    public int volume() { return clamp(prefs.getInt("volume", 80), 0, 100); }
    public void setVolume(int value) { prefs.edit().putInt("volume", clamp(value, 0, 100)).apply(); }
    public String country() { return safe(prefs.getString("country", "")); }
    public void setCountry(String value) { prefs.edit().putString("country", safe(value)).apply(); }
    public String genre() { return safe(prefs.getString("genre", "")); }
    public void setGenre(String value) { prefs.edit().putString("genre", safe(value)).apply(); }
    public String tab() { return safe(prefs.getString("tab", "all")); }
    public void setTab(String value) { prefs.edit().putString("tab", safe(value).isEmpty() ? "all" : safe(value)).apply(); }
    public String lastStationKey() { return safe(prefs.getString("last_key", "")); }
    public String lastStationName() { return safe(prefs.getString("last_name", "")); }
    public String lastStationMeta() { return safe(prefs.getString("last_meta", "")); }
    public void setLastStation(String key, String name, String meta) {
        prefs.edit().putString("last_key", safe(key)).putString("last_name", safe(name)).putString("last_meta", safe(meta)).apply();
    }

    private static List<String> readJsonList(String raw, int max) {
        List<String> out = new ArrayList<>();
        try {
            JSONArray a = new JSONArray(raw == null ? "[]" : raw);
            for (int i = 0; i < a.length() && out.size() < max; i++) {
                String v = safe(a.optString(i, ""));
                if (!v.isEmpty() && !out.contains(v)) out.add(v);
            }
        } catch (Exception ignored) { }
        return out;
    }

    private static String toJson(List<String> values) {
        JSONArray a = new JSONArray();
        for (String v : values) if (!safe(v).isEmpty()) a.put(v);
        return a.toString();
    }

    private static int clamp(int v, int min, int max) { return Math.max(min, Math.min(max, v)); }
    private static String safe(String v) { return v == null ? "" : v.trim(); }
    private static String cleanKey(String key) {
        key = safe(key);
        return key.length() > 512 ? key.substring(0, 512) : key;
    }
}
