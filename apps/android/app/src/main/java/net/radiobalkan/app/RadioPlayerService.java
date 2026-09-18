package net.radiobalkan.app;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.media.AudioAttributes;
import android.media.AudioFocusRequest;
import android.media.AudioManager;
import android.media.MediaMetadata;
import android.media.MediaPlayer;
import android.media.session.MediaSession;
import android.media.session.PlaybackState;
import android.os.Build;
import android.os.IBinder;
import android.os.Handler;
import android.os.Looper;
import android.os.PowerManager;
import java.io.BufferedInputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.RejectedExecutionException;

public final class RadioPlayerService extends Service {
    public static final String ACTION_PLAY = "net.radiobalkan.app.PLAY";
    public static final String ACTION_PAUSE = "net.radiobalkan.app.PAUSE";
    public static final String ACTION_RESUME = "net.radiobalkan.app.RESUME";
    public static final String ACTION_STOP = "net.radiobalkan.app.STOP";
    public static final String ACTION_QUERY = "net.radiobalkan.app.QUERY";
    public static final String ACTION_VOLUME = "net.radiobalkan.app.VOLUME";
    public static final String ACTION_STATE = "net.radiobalkan.app.STATE";
    public static final String INTERNAL_STATE_PERMISSION = "net.radiobalkan.app.permission.INTERNAL_STATE";
    public static final String EXTRA_KEY = "key";
    public static final String EXTRA_NAME = "name";
    public static final String EXTRA_META = "meta";
    public static final String EXTRA_URL = "url";
    public static final String EXTRA_RESOLVED = "resolved";
    public static final String EXTRA_UUID = "uuid";
    public static final String EXTRA_HOMEPAGE = "homepage";
    public static final String EXTRA_COUNTRY = "country";
    public static final String EXTRA_PLAYING = "playing";
    public static final String EXTRA_STATUS = "status";
    public static final String EXTRA_NOW_PLAYING = "now_playing";
    public static final String EXTRA_VOLUME = "volume";
    public static final String EXTRA_STOPPED = "stopped";
    private static final int NOTIFICATION_ID = 42;
    private static final String CHANNEL_ID = "radio_playback";

    private static final long PREPARE_TIMEOUT_MS = 15_000L;
    private final Object lock = new Object();
    private final Handler mainHandler = new Handler(Looper.getMainLooper());
    private final ExecutorService worker = Executors.newSingleThreadExecutor(r -> {
        Thread t = new Thread(r, "radio-player");
        t.setPriority(Thread.NORM_PRIORITY);
        return t;
    });
    private final ExecutorService metadataWorker = Executors.newSingleThreadExecutor(r -> {
        Thread t = new Thread(r, "radio-metadata");
        t.setPriority(Thread.MIN_PRIORITY);
        return t;
    });
    private MediaPlayer player;
    private MediaPlayer pendingPlayer;
    private int generation;
    private boolean playing;
    private boolean pausedByFocus;
    private boolean explicitlyStopped = true;
    private String currentKey = "";
    private String currentName = "Radio Balkan";
    private String currentMeta = "";
    private String currentUuid = "";
    private String currentHomepage = "";
    private String currentCountry = "";
    private String currentResolved = "";
    private String currentNowPlaying = "";
    private int metadataGeneration;
    private String status = "Spremno";
    private List<String> currentCandidates = new ArrayList<>();
    private int currentCandidate;
    private int reconnectRound;
    private StateStore state;
    private AudioManager audioManager;
    private AudioFocusRequest focusRequest;
    private MediaSession mediaSession;
    private boolean noisyRegistered;
    private Runnable prepareTimeout;

    private final AudioManager.OnAudioFocusChangeListener focusListener = focus -> {
        if (focus == AudioManager.AUDIOFOCUS_GAIN) {
            boolean shouldResume;
            synchronized (lock) { shouldResume = pausedByFocus; pausedByFocus = false; }
            if (shouldResume) resume();
        } else if (focus == AudioManager.AUDIOFOCUS_LOSS_TRANSIENT || focus == AudioManager.AUDIOFOCUS_LOSS_TRANSIENT_CAN_DUCK) {
            synchronized (lock) { pausedByFocus = playing; }
            pauseInternal("Pauzirano", false);
        } else if (focus == AudioManager.AUDIOFOCUS_LOSS) {
            synchronized (lock) { pausedByFocus = false; }
            pauseInternal("Pauzirano", false);
        }
    };

    private final BroadcastReceiver noisyReceiver = new BroadcastReceiver() {
        @Override public void onReceive(Context context, Intent intent) {
            if (AudioManager.ACTION_AUDIO_BECOMING_NOISY.equals(intent.getAction())) pause();
        }
    };

    @Override public void onCreate() {
        super.onCreate();
        state = new StateStore(this);
        audioManager = (AudioManager) getSystemService(Context.AUDIO_SERVICE);
        try {
            AudioAttributes attrs = audioAttributes();
            focusRequest = new AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
                    .setAudioAttributes(attrs)
                    .setWillPauseWhenDucked(true)
                    .setOnAudioFocusChangeListener(focusListener)
                    .build();
        } catch (Throwable t) {
            focusRequest = null;
            AppLog.e(this, "audio-focus-init", t);
        }
        try {
            mediaSession = new MediaSession(this, "RadioBalkan");
            mediaSession.setCallback(new MediaSession.Callback() {
                @Override public void onPlay() { resume(); }
                @Override public void onPause() { pause(); }
                @Override public void onStop() { stopPlayback(true); }
            });
            mediaSession.setActive(true);
        } catch (Throwable t) {
            AppLog.e(this, "media-session-init", t);
            try { if (mediaSession != null) mediaSession.release(); } catch (Throwable ignored) { }
            mediaSession = null;
        }
        try { createChannel(); } catch (Throwable t) { AppLog.e(this, "notification-channel", t); }
        registerNoisyReceiver();
        updateMediaSession();
    }

    @Override public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent == null) return START_NOT_STICKY;
        String action = safe(intent.getAction());
        try {
            if (ACTION_PLAY.equals(action)) {
                String key = safe(intent.getStringExtra(EXTRA_KEY));
                String name = safe(intent.getStringExtra(EXTRA_NAME));
                String meta = safe(intent.getStringExtra(EXTRA_META));
                String url = safe(intent.getStringExtra(EXTRA_URL));
                String resolved = safe(intent.getStringExtra(EXTRA_RESOLVED));
                String uuid = safe(intent.getStringExtra(EXTRA_UUID));
                String homepage = safe(intent.getStringExtra(EXTRA_HOMEPAGE));
                String country = safe(intent.getStringExtra(EXTRA_COUNTRY));
                play(key, name, meta, uuid, homepage, country, url, resolved);
            } else if (ACTION_PAUSE.equals(action)) {
                pause();
            } else if (ACTION_RESUME.equals(action)) {
                resume();
            } else if (ACTION_STOP.equals(action)) {
                stopPlayback(true);
            } else if (ACTION_VOLUME.equals(action)) {
                setVolume(intent.getIntExtra(EXTRA_VOLUME, state.volume()));
            } else if (ACTION_QUERY.equals(action)) {
                notifyState(status, playing);
                if (player == null && pendingPlayer == null && currentKey.isEmpty()) stopSelf(startId);
            }
        } catch (Throwable t) {
            AppLog.e(this, "service-action", t);
            notifyState("Dogodila se pogreška reprodukcije", false);
        }
        return START_NOT_STICKY;
    }

    private void play(String key, String name, String meta, String uuid, String homepage, String country, String url, String resolved) {
        country = safe(country).toUpperCase(java.util.Locale.ROOT);
        if (!RadioRepository.isSupportedCountry(country)) {
            notifyState("Stanica nije iz podržane regije", false);
            return;
        }
        final int gen;
        List<String> candidates = new ArrayList<>();
        String manual = state.manualReplacement(key);
        String automatic = state.autoReplacement(key);
        if (!manual.isEmpty()) candidates.add(manual);
        if (!automatic.isEmpty()) candidates.add(automatic);
        candidates.addAll(state.backups(key));
        if (!resolved.isEmpty()) candidates.add(resolved);
        if (!url.isEmpty()) candidates.add(url);
        candidates = dedupe(candidates);
        synchronized (lock) {
            generation++;
            gen = generation;
            releasePendingLocked();
            releasePlayerLocked();
            currentKey = key;
            currentName = name.isEmpty() ? "Radio Balkan" : name;
            currentMeta = meta;
            currentUuid = uuid;
            currentHomepage = homepage;
            currentCountry = country;
            currentResolved = "";
            currentNowPlaying = "";
            metadataGeneration++;
            currentCandidates = candidates;
            currentCandidate = 0;
            reconnectRound = 0;
            playing = false;
            pausedByFocus = false;
            explicitlyStopped = false;
            status = "Povezujem…";
        }
        state.setLastStation(key, currentName, meta);
        notifyState("Povezujem…", false);
        if (!updateForeground(false, "Povezujem…")) {
            synchronized (lock) {
                if (gen == generation) {
                    generation++;
                    currentCandidates = new ArrayList<>();
                    currentCandidate = 0;
                    currentKey = "";
                    playing = false;
                    status = "Reprodukcija nije dostupna";
                    updateMediaSessionLocked();
                }
            }
            notifyState("Reprodukcija nije dostupna", false);
            stopSelf();
            return;
        }
        updateMediaSession();
        executeWorker(() -> tryCurrentCandidate(gen));
    }

    private void tryCurrentCandidate(int gen) {
        for (;;) {
            String candidate;
            synchronized (lock) {
                if (gen != generation) return;
                if (currentCandidate >= currentCandidates.size()) break;
                candidate = currentCandidates.get(currentCandidate);
            }
            String resolved = StreamResolver.resolveFirst(Collections.singletonList(candidate));
            if (resolved == null) {
                synchronized (lock) { if (gen == generation) currentCandidate++; }
                continue;
            }
            if (prepareAsync(gen, resolved)) return;
            synchronized (lock) { if (gen == generation) currentCandidate++; }
        }
        discoverOrReconnect(gen);
    }

    private void discoverOrReconnect(int gen) {
        String uuid, homepage, key, country;
        synchronized (lock) {
            if (gen != generation) return;
            uuid = currentUuid;
            homepage = currentHomepage;
            key = currentKey;
            country = currentCountry;
        }
        StreamResolver.Resolution repaired = StreamResolver.repair(Collections.emptyList(), uuid, homepage, country);
        if (repaired != null && StreamResolver.isSafeHttp(repaired.url)) {
            synchronized (lock) {
                if (gen != generation) return;
                if (!currentCandidates.contains(repaired.url)) currentCandidates.add(repaired.url);
                currentCandidate = currentCandidates.indexOf(repaired.url);
            }
            state.setAutoReplacement(key, repaired.url);
            state.addBackup(key, repaired.url);
            notifyState("Pronađen je novi izvor · povezujem…", false);
            if (prepareAsync(gen, repaired.url)) return;
        }
        boolean unavailable;
        int round;
        synchronized (lock) {
            if (gen != generation) return;
            unavailable = reconnectRound >= 2 || currentCandidates.isEmpty();
            if (unavailable) {
                status = "Stanica trenutno nije dostupna";
                playing = false;
                updateMediaSessionLocked();
                round = reconnectRound;
            } else {
                reconnectRound++;
                currentCandidate = 0;
                status = "Ponovno povezujem…";
                round = reconnectRound;
            }
        }
        if (unavailable) {
            notifyState("Stanica trenutno nije dostupna", false);
            stopForeground(false);
            abandonAudioFocus();
            return;
        }
        notifyState("Ponovno povezujem…", false);
        try {
            Thread.sleep(1200L * round);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            return;
        }
        tryCurrentCandidate(gen);
    }

    private boolean prepareAsync(int gen, String stream) {
        if (!StreamResolver.isSafeHttpForConnection(stream)) return false;
        final MediaPlayer next = new MediaPlayer();
        try {
            next.setAudioAttributes(audioAttributes());
            next.setWakeMode(this, PowerManager.PARTIAL_WAKE_LOCK);
            float v = state.volume() / 100f;
            next.setVolume(v, v);
            next.setDataSource(stream);
            next.setOnPreparedListener(mp -> onPrepared(gen, stream, mp));
            next.setOnErrorListener((mp, what, extra) -> {
                AppLog.e(this, "player-error", new IllegalStateException("MediaPlayer " + what + "/" + extra));
                onAsyncPlayerError(gen, mp);
                return true;
            });
            next.setOnCompletionListener(mp -> onAsyncPlayerError(gen, mp));
            synchronized (lock) {
                if (gen != generation) { release(next); return false; }
                releasePendingLocked();
                pendingPlayer = next;
                schedulePrepareTimeoutLocked(gen, next);
            }
            next.prepareAsync();
            return true;
        } catch (Throwable t) {
            AppLog.e(this, "player-prepare", t);
            synchronized (lock) {
                if (pendingPlayer == next) {
                    cancelPrepareTimeoutLocked();
                    pendingPlayer = null;
                }
            }
            release(next);
            return false;
        }
    }

    private void onPrepared(int gen, String stream, MediaPlayer mp) {
        synchronized (lock) {
            if (gen != generation || pendingPlayer != mp) {
                release(mp);
                return;
            }
            cancelPrepareTimeoutLocked();
            pendingPlayer = null;
        }
        if (!requestAudioFocus()) {
            release(mp);
            synchronized (lock) {
                if (gen != generation) return;
                status = "Audio trenutno koristi druga aplikacija";
                playing = false;
                updateMediaSessionLocked();
            }
            notifyState("Audio trenutno koristi druga aplikacija", false);
            updateForeground(false, "Pauzirano");
            return;
        }
        try {
            mp.start();
        } catch (Throwable t) {
            AppLog.e(this, "player-start", t);
            release(mp);
            synchronized (lock) { if (gen == generation) currentCandidate++; }
            executeWorker(() -> tryCurrentCandidate(gen));
            return;
        }
        synchronized (lock) {
            if (gen != generation) { release(mp); return; }
            releasePlayerLocked();
            player = mp;
            currentResolved = stream;
            currentNowPlaying = "";
            metadataGeneration++;
            playing = true;
            pausedByFocus = false;
            reconnectRound = 0;
            status = "Uživo";
            updateMediaSessionLocked();
        }
        state.addRecent(currentKey);
        state.addBackup(currentKey, stream);
        notifyState("Uživo", true);
        updateForeground(true, "Uživo");
        startMetadataLoop(gen, stream);
    }

    private void onAsyncPlayerError(int gen, MediaPlayer mp) {
        boolean retry = false;
        synchronized (lock) {
            if (gen == generation && (pendingPlayer == mp || player == mp)) {
                if (pendingPlayer == mp) {
                    cancelPrepareTimeoutLocked();
                    pendingPlayer = null;
                }
                if (player == mp) player = null;
                playing = false;
                currentNowPlaying = "";
                metadataGeneration++;
                currentCandidate++;
                status = "Ponovno povezujem…";
                updateMediaSessionLocked();
                retry = true;
            }
        }
        release(mp);
        if (!retry) return;
        notifyState("Ponovno povezujem…", false);
        executeWorker(() -> tryCurrentCandidate(gen));
    }

    private void pause() { pauseInternal("Pauzirano", true); }

    private void pauseInternal(String newStatus, boolean abandonFocus) {
        synchronized (lock) {
            if (player == null || !playing) return;
            try { player.pause(); playing = false; metadataGeneration++; status = newStatus; updateMediaSessionLocked(); }
            catch (Throwable t) { AppLog.e(this, "pause", t); return; }
        }
        if (abandonFocus) abandonAudioFocus();
        notifyState(newStatus, false);
        updateForeground(false, newStatus);
    }

    private void resume() {
        final int gen;
        synchronized (lock) {
            gen = generation;
            if (!PlaybackLifecycle.canResume(explicitlyStopped, player != null, !currentCandidates.isEmpty())) return;
            if (player == null) {
                currentCandidate = Math.min(currentCandidate, Math.max(0, currentCandidates.size() - 1));
                status = "Ponovno povezujem…";
                executeWorker(() -> tryCurrentCandidate(gen));
                notifyState(status, false);
                return;
            }
        }
        if (!requestAudioFocus()) {
            notifyState("Audio trenutno koristi druga aplikacija", false);
            return;
        }
        MediaPlayer failed = null;
        synchronized (lock) {
            if (gen != generation || player == null) return;
            try {
                player.start();
                playing = true;
                pausedByFocus = false;
                status = "Uživo";
                updateMediaSessionLocked();
            } catch (Throwable t) {
                AppLog.e(this, "resume", t);
                failed = player;
                player = null;
                playing = false;
                currentNowPlaying = "";
                metadataGeneration++;
                currentCandidate++;
                status = "Ponovno povezujem…";
                updateMediaSessionLocked();
            }
        }
        if (failed != null) {
            release(failed);
            notifyState("Ponovno povezujem…", false);
            executeWorker(() -> tryCurrentCandidate(gen));
            return;
        }
        notifyState("Uživo", true);
        updateForeground(true, "Uživo");
        String stream;
        synchronized (lock) { stream = currentResolved; }
        if (!stream.isEmpty()) startMetadataLoop(gen, stream);
    }

    private void setVolume(int value) {
        value = Math.max(0, Math.min(100, value));
        state.setVolume(value);
        float v = value / 100f;
        synchronized (lock) {
            try { if (player != null) player.setVolume(v, v); } catch (Throwable t) { AppLog.e(this, "volume", t); }
            try { if (pendingPlayer != null) pendingPlayer.setVolume(v, v); } catch (Throwable ignored) { }
        }
    }

    private void stopPlayback(boolean stopSelfToo) {
        synchronized (lock) {
            generation++;
            playing = false;
            pausedByFocus = false;
            explicitlyStopped = true;
            currentResolved = "";
            currentNowPlaying = "";
            metadataGeneration++;
            status = "Zaustavljeno";
            releasePendingLocked();
            releasePlayerLocked();
            updateMediaSessionLocked();
        }
        abandonAudioFocus();
        notifyState("Zaustavljeno", false);
        stopForeground(true);
        if (stopSelfToo) stopSelf();
    }

    private void releasePlayerLocked() {
        if (player != null) { release(player); player = null; }
    }

    private void releasePendingLocked() {
        cancelPrepareTimeoutLocked();
        if (pendingPlayer != null) { release(pendingPlayer); pendingPlayer = null; }
    }

    private void schedulePrepareTimeoutLocked(int gen, MediaPlayer target) {
        cancelPrepareTimeoutLocked();
        prepareTimeout = () -> {
            boolean retry = false;
            synchronized (lock) {
                if (gen != generation || pendingPlayer != target) return;
                pendingPlayer = null;
                prepareTimeout = null;
                playing = false;
                currentCandidate++;
                status = "Izvor ne odgovara · pokušavam sljedeći…";
                updateMediaSessionLocked();
                retry = true;
            }
            release(target);
            notifyState("Izvor ne odgovara · pokušavam sljedeći…", false);
            if (retry) executeWorker(() -> tryCurrentCandidate(gen));
        };
        mainHandler.postDelayed(prepareTimeout, PREPARE_TIMEOUT_MS);
    }

    private void cancelPrepareTimeoutLocked() {
        if (prepareTimeout != null) {
            mainHandler.removeCallbacks(prepareTimeout);
            prepareTimeout = null;
        }
    }

    private static void release(MediaPlayer p) {
        try { p.setOnPreparedListener(null); p.setOnErrorListener(null); p.setOnCompletionListener(null); } catch (Throwable ignored) { }
        try { p.reset(); } catch (Throwable ignored) { }
        try { p.release(); } catch (Throwable ignored) { }
    }

    private AudioAttributes audioAttributes() {
        return new AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_MEDIA).setContentType(AudioAttributes.CONTENT_TYPE_MUSIC).build();
    }

    @SuppressWarnings("deprecation")
    private boolean requestAudioFocus() {
        if (audioManager == null) return true;
        try {
            if (focusRequest != null) {
                return audioManager.requestAudioFocus(focusRequest) == AudioManager.AUDIOFOCUS_REQUEST_GRANTED;
            }
            return audioManager.requestAudioFocus(focusListener, AudioManager.STREAM_MUSIC, AudioManager.AUDIOFOCUS_GAIN)
                    == AudioManager.AUDIOFOCUS_REQUEST_GRANTED;
        } catch (Throwable t) {
            AppLog.e(this, "audio-focus", t);
            return false;
        }
    }

    @SuppressWarnings("deprecation")
    private void abandonAudioFocus() {
        if (audioManager == null) return;
        try {
            if (focusRequest != null) audioManager.abandonAudioFocusRequest(focusRequest);
            else audioManager.abandonAudioFocus(focusListener);
        } catch (Throwable ignored) { }
    }

    private Notification buildNotification(boolean isPlaying, String notificationStatus) {
        final String name;
        final String meta;
        final String nowPlaying;
        synchronized (lock) {
            name = currentName;
            meta = currentMeta;
            nowPlaying = currentNowPlaying;
        }
        Intent open = new Intent(this, MainActivity.class).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_CLEAR_TOP);
        PendingIntent content = PendingIntent.getActivity(this, 1, open, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        Intent toggle = new Intent(this, RadioPlayerService.class).setAction(isPlaying ? ACTION_PAUSE : ACTION_RESUME);
        Intent stop = new Intent(this, RadioPlayerService.class).setAction(ACTION_STOP);
        PendingIntent togglePi = PendingIntent.getService(this, 2, toggle, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        PendingIntent stopPi = PendingIntent.getService(this, 3, stop, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        Notification.Builder b = new Notification.Builder(this, CHANNEL_ID);
        b.setContentTitle(name)
                .setContentText(notificationStatus + " · " + (!nowPlaying.isEmpty() ? nowPlaying : (meta.isEmpty() ? "Radio Balkan" : meta)))
                .setSmallIcon(R.drawable.ic_radio_balkan)
                .setContentIntent(content)
                .setOnlyAlertOnce(true)
                .setOngoing(true)
                .setCategory(Notification.CATEGORY_SERVICE)
                .setVisibility(Notification.VISIBILITY_PUBLIC)
                .addAction(new Notification.Action.Builder(android.R.drawable.ic_media_play, isPlaying ? "Pauza" : "Nastavi", togglePi).build())
                .addAction(new Notification.Action.Builder(android.R.drawable.ic_menu_close_clear_cancel, "Stop", stopPi).build());
        if (mediaSession != null) b.setStyle(new Notification.MediaStyle().setMediaSession(mediaSession.getSessionToken()).setShowActionsInCompactView(0, 1));
        return b.build();
    }

    private boolean updateForeground(boolean isPlaying, String notificationStatus) {
        try {
            startForeground(NOTIFICATION_ID, buildNotification(isPlaying, notificationStatus));
            return true;
        } catch (Throwable t) {
            AppLog.e(this, "foreground-notification", t);
            return false;
        }
    }

    private void notifyState(String newStatus, boolean isPlaying) {
        final String key;
        final String name;
        final String meta;
        final String resolved;
        final String nowPlaying;
        final String country;
        final boolean stopped;
        synchronized (lock) {
            status = newStatus;
            key = currentKey;
            name = currentName;
            meta = currentMeta;
            resolved = currentResolved;
            nowPlaying = currentNowPlaying;
            country = currentCountry;
            stopped = explicitlyStopped;
        }
        Intent i = new Intent(ACTION_STATE).setPackage(getPackageName());
        i.putExtra(EXTRA_KEY, key);
        i.putExtra(EXTRA_NAME, name);
        i.putExtra(EXTRA_META, meta);
        i.putExtra(EXTRA_COUNTRY, country);
        i.putExtra(EXTRA_PLAYING, isPlaying);
        i.putExtra(EXTRA_STOPPED, stopped);
        i.putExtra(EXTRA_STATUS, newStatus);
        i.putExtra(EXTRA_URL, resolved);
        i.putExtra(EXTRA_NOW_PLAYING, nowPlaying);
        try {
            sendBroadcast(i, INTERNAL_STATE_PERMISSION);
        } catch (Throwable t) {
            AppLog.e(this, "state-broadcast", t);
        }
    }

    private void updateMediaSession() {
        synchronized (lock) { updateMediaSessionLocked(); }
    }

    private void updateMediaSessionLocked() {
        if (mediaSession == null) return;
        long actions = PlaybackState.ACTION_PLAY | PlaybackState.ACTION_PAUSE | PlaybackState.ACTION_PLAY_PAUSE | PlaybackState.ACTION_STOP;
        int stateValue = playing ? PlaybackState.STATE_PLAYING : (player != null ? PlaybackState.STATE_PAUSED : PlaybackState.STATE_STOPPED);
        mediaSession.setPlaybackState(new PlaybackState.Builder().setActions(actions).setState(stateValue, PlaybackState.PLAYBACK_POSITION_UNKNOWN, playing ? 1f : 0f).build());
        mediaSession.setMetadata(new MediaMetadata.Builder()
                .putString(MediaMetadata.METADATA_KEY_TITLE, currentName)
                .putString(MediaMetadata.METADATA_KEY_ARTIST, currentNowPlaying.isEmpty() ? (currentMeta.isEmpty() ? "Radio Balkan" : currentMeta) : currentNowPlaying)
                .build());
    }

    private void startMetadataLoop(int playerGeneration, String stream) {
        if (stream == null || stream.isEmpty() || stream.toLowerCase().contains(".m3u8")) return;
        final int metaGen;
        synchronized (lock) { metaGen = ++metadataGeneration; }
        executeMetadata(() -> {
            while (!Thread.currentThread().isInterrupted()) {
                synchronized (lock) {
                    if (playerGeneration != generation || metaGen != metadataGeneration || !playing) return;
                }
                String title = fetchIcyTitle(stream);
                if (!title.isEmpty()) {
                    synchronized (lock) {
                        if (playerGeneration != generation || metaGen != metadataGeneration || !playing) return;
                        if (!title.equals(currentNowPlaying)) {
                            currentNowPlaying = title;
                            updateMediaSessionLocked();
                        }
                    }
                    notifyState("Uživo", true);
                    updateForeground(true, "Uživo");
                }
                for (int i = 0; i < 24; i++) {
                    try { Thread.sleep(5000); } catch (InterruptedException e) { Thread.currentThread().interrupt(); return; }
                    synchronized (lock) {
                        if (playerGeneration != generation || metaGen != metadataGeneration || !playing) return;
                    }
                }
            }
        });
    }

    private String fetchIcyTitle(String raw) {
        if (!StreamResolver.isSafeHttpForConnection(raw)) return "";
        HttpURLConnection c = null;
        try {
            c = (HttpURLConnection) new URL(raw).openConnection();
            c.setConnectTimeout(5000); c.setReadTimeout(7500); c.setInstanceFollowRedirects(false); c.setUseCaches(false);
            c.setRequestProperty("User-Agent", AppInfo.USER_AGENT);
            c.setRequestProperty("Icy-MetaData", "1");
            int statusCode = c.getResponseCode();
            if (statusCode < 200 || statusCode >= 300) return "";
            if (!StreamResolver.isSafeHttp(c.getURL().toString())) return "";
            int interval;
            try { interval = Integer.parseInt(c.getHeaderField("icy-metaint")); } catch (Throwable ignored) { return ""; }
            if (interval <= 0 || interval > 4 * 1024 * 1024) return "";
            try (BufferedInputStream in = new BufferedInputStream(c.getInputStream())) {
                long remaining = interval;
                byte[] skip = new byte[8192];
                while (remaining > 0) {
                    int n = in.read(skip, 0, (int) Math.min(skip.length, remaining));
                    if (n < 0) return "";
                    remaining -= n;
                }
                int lenByte = in.read();
                if (lenByte <= 0) return "";
                int metaLen = lenByte * 16;
                byte[] data = new byte[metaLen];
                int off = 0;
                while (off < metaLen) {
                    int n = in.read(data, off, metaLen - off);
                    if (n < 0) break;
                    off += n;
                }
                String meta = new String(data, 0, off, StandardCharsets.UTF_8).replace("\u0000", "").trim();
                int p = meta.indexOf("StreamTitle='");
                if (p < 0) return "";
                p += "StreamTitle='".length();
                int end = meta.indexOf("';", p);
                if (end < 0) end = meta.indexOf('\'', p);
                if (end <= p) return "";
                String title = meta.substring(p, end).trim();
                return title.length() > 180 ? title.substring(0, 180) : title;
            }
        } catch (Throwable ignored) {
            return "";
        } finally {
            if (c != null) c.disconnect();
        }
    }

    private void createChannel() {
        NotificationChannel channel = new NotificationChannel(CHANNEL_ID, getString(R.string.channel_name), NotificationManager.IMPORTANCE_LOW);
        channel.setDescription(getString(R.string.channel_description));
        channel.setShowBadge(false);
        NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        if (manager != null) manager.createNotificationChannel(channel);
    }

    private void registerNoisyReceiver() {
        try {
            IntentFilter f = new IntentFilter(AudioManager.ACTION_AUDIO_BECOMING_NOISY);
            if (Build.VERSION.SDK_INT >= 33) registerReceiver(noisyReceiver, f, Context.RECEIVER_NOT_EXPORTED);
            else registerReceiver(noisyReceiver, f);
            noisyRegistered = true;
        } catch (Throwable t) { AppLog.e(this, "noisy-receiver", t); }
    }

    private void executeWorker(Runnable task) {
        try { worker.execute(task); }
        catch (RejectedExecutionException ignored) { }
    }

    private void executeMetadata(Runnable task) {
        try { metadataWorker.execute(task); }
        catch (RejectedExecutionException ignored) { }
    }

    private static List<String> dedupe(List<String> in) {
        LinkedHashSet<String> out = new LinkedHashSet<>();
        if (in != null) for (String v : in) if (StreamResolver.isSafeHttp(v)) { out.add(v.trim()); if (out.size() >= 16) break; }
        return new ArrayList<>(out);
    }

    private static String safe(String v) { return v == null ? "" : v.trim(); }

    @Override public void onDestroy() {
        try { stopPlayback(false); } catch (Throwable ignored) { }
        mainHandler.removeCallbacksAndMessages(null);
        worker.shutdownNow();
        metadataWorker.shutdownNow();
        if (noisyRegistered) { try { unregisterReceiver(noisyReceiver); } catch (Throwable ignored) { } }
        abandonAudioFocus();
        try { if (mediaSession != null) { mediaSession.setActive(false); mediaSession.release(); } } catch (Throwable ignored) { }
        super.onDestroy();
    }

    @Override public IBinder onBind(Intent intent) { return null; }
}
