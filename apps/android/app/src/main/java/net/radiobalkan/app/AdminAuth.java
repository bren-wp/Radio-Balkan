package net.radiobalkan.app;

import java.security.GeneralSecurityException;
import java.security.MessageDigest;
import java.util.Locale;
import javax.crypto.SecretKeyFactory;
import javax.crypto.spec.PBEKeySpec;

final class AdminAuth {
    private static final String USERNAME = "brendigo";
    private static final int ITERATIONS = 120_000;
    private static final byte[] SALT = hex("c6d79acaafb52bb8bac278313e84ccf7");
    private static final byte[] EXPECTED_KEY = hex("6d319ade7c2c0f333d1d520eaf582034a80cabbc4e2f0317527b68576b631f80");

    private AdminAuth() { }

    static boolean matches(String username, String password) {
        String normalized = username == null ? "" : username.trim().toLowerCase(Locale.ROOT);
        if (!USERNAME.equals(normalized) || password == null) return false;
        PBEKeySpec spec = new PBEKeySpec(password.toCharArray(), SALT, ITERATIONS, 256);
        try {
            SecretKeyFactory factory = SecretKeyFactory.getInstance("PBKDF2WithHmacSHA256");
            byte[] actual = factory.generateSecret(spec).getEncoded();
            return MessageDigest.isEqual(EXPECTED_KEY, actual);
        } catch (GeneralSecurityException ignored) {
            return false;
        } finally {
            spec.clearPassword();
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
