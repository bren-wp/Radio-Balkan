package net.radiobalkan.app;

import android.content.Context;
import java.io.File;
import java.io.FileWriter;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.Locale;

public final class AppLog {
    private static final long MAX_LOG_BYTES = 512L * 1024L;
    private static final int MAX_STACK_FRAMES = 160;

    private AppLog() { }

    public static synchronized void e(Context c, String scope, Throwable t) {
        if (c == null) return;
        try {
            File directory = c.getFilesDir();
            if (directory == null) return;
            File f = new File(directory, "app.log");
            if (f.length() > MAX_LOG_BYTES) rotateOrTruncate(f, new File(directory, "app.log.1"));

            Throwable error = t == null ? new IllegalStateException("Nepoznata pogreška") : t;
            try (FileWriter w = new FileWriter(f, true)) {
                String ts = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.ROOT).format(new Date());
                w.write(ts + " [" + bounded(scope, 256) + "] " + bounded(String.valueOf(error), 2048) + "\n");
                StackTraceElement[] stack = error.getStackTrace();
                int limit = Math.min(stack.length, MAX_STACK_FRAMES);
                for (int i = 0; i < limit; i++) w.write("  at " + stack[i] + "\n");
                if (stack.length > limit) w.write("  … " + (stack.length - limit) + " dodatnih frameova\n");
            }
        } catch (Exception ignored) { }
    }

    private static void rotateOrTruncate(File current, File previous) {
        try {
            if (previous.exists() && !previous.delete()) {
                try (FileWriter truncateOld = new FileWriter(previous, false)) {
                    truncateOld.write("");
                }
            }
            if (current.renameTo(previous)) return;
        } catch (Exception ignored) { }
        try (FileWriter truncateCurrent = new FileWriter(current, false)) {
            truncateCurrent.write("");
        } catch (Exception ignored) { }
    }

    private static String bounded(String value, int max) {
        String clean = value == null ? "" : value.replace('\r', ' ').replace('\n', ' ');
        return clean.length() <= max ? clean : clean.substring(0, max) + "…";
    }
}
