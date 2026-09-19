package net.radiobalkan.app;

import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

/** Public, non-sensitive station copy shared by list/detail surfaces. */
public final class StationPresentation {
    private StationPresentation() { }

    public static String area(RadioStation s) {
        if (s == null) return "Radio uživo";
        String country = safe(s.country);
        if (RadioRepository.DIASPORA_CODE.equalsIgnoreCase(s.countryCode)) {
            return country.isEmpty() ? "Dijaspora" : "Dijaspora · " + country;
        }
        if (RadioRepository.FOREIGN_CODE.equalsIgnoreCase(s.countryCode)) {
            return country.isEmpty() ? "Strano" : "Strano · " + country;
        }
        if (!country.isEmpty()) return country;
        return safe(s.countryCode).isEmpty() ? "Radio uživo" : safe(s.countryCode);
    }

    public static String firstUsefulTag(String tags) {
        if (tags == null) return "";
        for (String raw : tags.split(",")) {
            String value = raw.trim();
            if (value.isEmpty() || "dijaspora".equalsIgnoreCase(value)) continue;
            if (value.length() <= 32) return value;
        }
        return "";
    }

    public static String description(RadioStation s) {
        if (s == null) return "Radio uživo.";
        StringBuilder out = new StringBuilder();
        out.append(safe(s.name).isEmpty() ? "Ova radio stanica" : safe(s.name))
                .append(" je radio stanica iz područja ").append(area(s)).append(". ");
        String genre = firstUsefulTag(s.tags);
        if (!genre.isEmpty()) out.append("Istaknuta kategorija: ").append(genre).append(". ");
        if (!safe(s.language).isEmpty()) out.append("Jezik programa: ").append(safe(s.language)).append(". ");
        out.append("Slušanje koristi sigurni Radio Balkan player s ograničenim timeoutom te fallback i recovery postupkom kada je dostupan.");
        return out.toString();
    }

    public static String publicDetails(RadioStation s) {
        if (s == null) return "Radio uživo";
        List<String> parts = new ArrayList<>();
        if (!safe(s.state).isEmpty() && !safe(s.state).equalsIgnoreCase(s.country)) parts.add(safe(s.state));
        if (!safe(s.codec).isEmpty()) parts.add(safe(s.codec).toUpperCase(Locale.ROOT));
        if (s.bitrate > 0) parts.add(s.bitrate + " kbps");
        List<String> tags = new ArrayList<>();
        if (s.tags != null) {
            for (String raw : s.tags.split(",")) {
                String value = raw.trim();
                if (value.isEmpty() || "dijaspora".equalsIgnoreCase(value) || tags.contains(value)) continue;
                tags.add(value);
                if (tags.size() >= 5) break;
            }
        }
        if (!tags.isEmpty()) parts.add("Kategorije: " + join(tags));
        return parts.isEmpty() ? "Radio uživo" : join(parts);
    }

    private static String join(List<String> values) {
        StringBuilder out = new StringBuilder();
        for (String value : values) {
            if (out.length() > 0) out.append(" · ");
            out.append(value);
        }
        return out.toString();
    }

    private static String safe(String value) {
        return value == null ? "" : value.trim();
    }
}
