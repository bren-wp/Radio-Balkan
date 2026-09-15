package net.radiobalkan.app;

import android.app.Application;
import android.content.Context;

public final class RadioBalkanApplication extends Application {
    @Override public void onCreate() {
        super.onCreate();
        final Context appContext = getApplicationContext();
        final Thread.UncaughtExceptionHandler previous = Thread.getDefaultUncaughtExceptionHandler();
        Thread.setDefaultUncaughtExceptionHandler((thread, error) -> {
            try { AppLog.e(appContext, "uncaught-" + thread.getName(), error); } catch (Throwable ignored) { }
            if (previous != null) previous.uncaughtException(thread, error);
        });
    }
}
