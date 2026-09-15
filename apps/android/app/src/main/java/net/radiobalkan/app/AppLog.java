package net.radiobalkan.app;

import android.content.Context;
import java.io.File;
import java.io.FileWriter;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.Locale;

public final class AppLog {
    private AppLog() { }

    public static synchronized void e(Context c, String scope, Throwable t) {
        try {
            File f = new File(c.getFilesDir(), "app.log");
            if (f.length() > 512 * 1024) {
                File old = new File(c.getFilesDir(), "app.log.1");
                //noinspection ResultOfMethodCallIgnored
                old.delete();
                //noinspection ResultOfMethodCallIgnored
                f.renameTo(old);
            }
            try (FileWriter w = new FileWriter(f, true)) {
                String ts = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.ROOT).format(new Date());
                w.write(ts + " [" + scope + "] " + String.valueOf(t) + "\n");
                for (StackTraceElement x : t.getStackTrace()) w.write("  at " + x + "\n");
            }
        } catch (Exception ignored) { }
    }
}
