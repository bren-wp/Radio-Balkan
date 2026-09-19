package net.radiobalkan.app;

import android.content.Context;
import android.content.Intent;
import android.os.Build;

/** Single internal path for starting a station playback session. */
public final class PlaybackStarter {
    private PlaybackStarter() { }

    public static boolean start(Context context, RadioStation station, StateStore state) {
        if (context == null || station == null || state == null) return false;
        String key = station.key();
        if (key.isEmpty()) return false;

        String resolved = state.manualReplacement(key);
        if (resolved.isEmpty()) resolved = state.autoReplacement(key);
        if (resolved.isEmpty()) resolved = safe(station.activeUrl);
        if (resolved.isEmpty()) resolved = safe(station.urlResolved);
        if (resolved.isEmpty()) resolved = safe(station.url);

        Intent intent = new Intent(context, RadioPlayerService.class).setAction(RadioPlayerService.ACTION_PLAY);
        intent.putExtra(RadioPlayerService.EXTRA_KEY, key);
        intent.putExtra(RadioPlayerService.EXTRA_NAME, safe(station.name));
        intent.putExtra(RadioPlayerService.EXTRA_META, station.meta());
        intent.putExtra(RadioPlayerService.EXTRA_URL, safe(station.url));
        intent.putExtra(RadioPlayerService.EXTRA_RESOLVED, resolved);
        intent.putExtra(RadioPlayerService.EXTRA_UUID, safe(station.stationUuid));
        intent.putExtra(RadioPlayerService.EXTRA_HOMEPAGE, safe(station.homepage));
        intent.putExtra(RadioPlayerService.EXTRA_COUNTRY, safe(station.countryCode));
        try {
            if (Build.VERSION.SDK_INT >= 26) context.startForegroundService(intent);
            else context.startService(intent);
        } catch (Throwable error) {
            AppLog.e(context, "player-play", error);
            return false;
        }

        state.addRecent(key);
        state.setLastStation(key, safe(station.name), station.meta());
        return true;
    }

    private static String safe(String value) {
        return value == null ? "" : value.trim();
    }
}
