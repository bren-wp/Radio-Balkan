package net.radiobalkan.app;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.Locale;

final class AdminAuth {
    private static final String USERNAME = "brendigo";
    private static final String SALT = "RadioBalkanAdmin:v1:";
    private static final byte[] EXPECTED_DIGEST = hex("79cf893dcfdb18ecc6eba591896f896c5dd3eab95354d4e7e4503d13292fe9a0");

    private AdminAuth() { }

    static boolean matches(String username, String password) {
        String normalized = username == null ? "" : username.trim().toLowerCase(Locale.ROOT);
        if (!USERNAME.equals(normalized) || password == null) return false;
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] actual = digest.digest((SALT + password).getBytes(StandardCharsets.UTF_8));
            return MessageDigest.isEqual(EXPECTED_DIGEST, actual);
        } catch (Exception ignored) {
            return false;
        }
    }

    private static byte[] hex(String value) {
        byte[] out = new byte[value.length() / 2];
        for (int i = 0; i < value.length(); i += 2) {
            int hi = Character.digit(value.charAt(i), 16);
            int lo = Character.digit(value.charAt(i + 1), 16);
            out[i / 2] = (byte) ((hi << 4) | lo);
        }
        return out;
    }
}
