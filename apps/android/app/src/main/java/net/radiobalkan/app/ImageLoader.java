package net.radiobalkan.app;

import android.content.ComponentCallbacks2;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.os.Handler;
import android.os.Looper;
import android.util.LruCache;
import android.view.View;
import android.widget.ImageView;
import android.widget.TextView;
import java.io.BufferedInputStream;
import java.io.ByteArrayOutputStream;
import java.lang.ref.WeakReference;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.ThreadPoolExecutor;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.RejectedExecutionException;

/** Lightweight image loader tuned for station logos and ListView recycling. */
public final class ImageLoader {
    private static final int MAX_IMAGE_BYTES = 512 * 1024;
    private static final int MAX_CACHE_BYTES = 4 * 1024 * 1024;
    private static final int MIN_CACHE_BYTES = 1536 * 1024;
    private static final int TRIMMED_CACHE_BYTES = 768 * 1024;

    private static final class Request {
        final WeakReference<ImageView> image;
        final WeakReference<TextView> fallback;
        final String url;
        Request(String url, ImageView image, TextView fallback) {
            this.url = url;
            this.image = new WeakReference<>(image);
            this.fallback = new WeakReference<>(fallback);
        }
    }

    private final Handler main = new Handler(Looper.getMainLooper());
    private final ExecutorService pool = new ThreadPoolExecutor(
            2, 2, 20L, TimeUnit.SECONDS, new ArrayBlockingQueue<>(40), r -> {
                Thread t = new Thread(r, "radio-image");
                t.setPriority(Thread.MIN_PRIORITY);
                return t;
            }, new ThreadPoolExecutor.AbortPolicy());
    private final LruCache<String, Bitmap> cache = new LruCache<String, Bitmap>(cacheBudget()) {
        @Override protected int sizeOf(String key, Bitmap value) { return value == null ? 0 : value.getByteCount(); }
    };
    private final Object inFlightLock = new Object();
    private final Map<String, List<Request>> inFlight = new HashMap<>();
    private volatile boolean closed;

    private static int cacheBudget() {
        long heap = Runtime.getRuntime().maxMemory();
        long target = Math.max(MIN_CACHE_BYTES, Math.min(MAX_CACHE_BYTES, heap / 32L));
        return (int) target;
    }

    public void load(String rawUrl, ImageView image, TextView fallback) {
        if (image == null) return;
        String url = rawUrl == null ? "" : rawUrl.trim();
        preparePlaceholder(image, fallback, url);
        if (closed || !StreamResolver.isSafeHttp(url)) return;

        Bitmap cached;
        synchronized (cache) { cached = cache.get(url); }
        if (cached != null && !cached.isRecycled()) {
            showBitmap(image, fallback, cached);
            return;
        }

        boolean shouldStart;
        synchronized (inFlightLock) {
            List<Request> waiters = inFlight.get(url);
            shouldStart = waiters == null;
            if (waiters == null) {
                waiters = new ArrayList<>(2);
                inFlight.put(url, waiters);
            }
            boolean alreadyQueued = false;
            for (Request request : waiters) {
                if (request.image.get() == image) { alreadyQueued = true; break; }
            }
            if (!alreadyQueued) waiters.add(new Request(url, image, fallback));
        }
        if (!shouldStart) return;

        try {
            pool.execute(() -> complete(url, download(url)));
        } catch (RejectedExecutionException ignored) {
            synchronized (inFlightLock) { inFlight.remove(url); }
        }
    }

    private void complete(String url, Bitmap bitmap) {
        List<Request> waiters;
        synchronized (inFlightLock) { waiters = inFlight.remove(url); }
        if (closed) return;
        if (bitmap != null) {
            synchronized (cache) { cache.put(url, bitmap); }
        }
        if (waiters == null || waiters.isEmpty() || bitmap == null) return;
        main.post(() -> {
            if (closed || bitmap.isRecycled()) return;
            for (Request request : waiters) {
                ImageView image = request.image.get();
                if (image == null) continue;
                Object tag = image.getTag();
                if (tag == null || !request.url.equals(tag.toString())) continue;
                showBitmap(image, request.fallback.get(), bitmap);
            }
        });
    }

    private static void preparePlaceholder(ImageView image, TextView fallback, String url) {
        image.setTag(url);
        if (fallback != null) {
            image.setImageDrawable(null);
            image.setVisibility(View.GONE);
            fallback.setVisibility(View.VISIBLE);
        } else {
            image.setVisibility(View.VISIBLE);
        }
    }

    private static void showBitmap(ImageView image, TextView fallback, Bitmap bitmap) {
        image.setImageBitmap(bitmap);
        image.setVisibility(View.VISIBLE);
        if (fallback != null) fallback.setVisibility(View.GONE);
    }

    private Bitmap download(String raw) {
        HttpURLConnection c = null;
        try {
            if (!StreamResolver.isSafeHttp(raw)) return null;
            c = (HttpURLConnection) new URL(raw).openConnection();
            c.setConnectTimeout(4500);
            c.setReadTimeout(5500);
            c.setInstanceFollowRedirects(true);
            c.setUseCaches(true);
            c.setRequestProperty("User-Agent", AppInfo.USER_AGENT);
            c.setRequestProperty("Accept", "image/*,*/*;q=0.2");
            int code = c.getResponseCode();
            if (code < 200 || code >= 400) return null;
            if (!StreamResolver.isSafeHttp(c.getURL().toString())) return null;
            String type = c.getContentType();
            if (type != null && !type.toLowerCase(Locale.ROOT).startsWith("image/")) return null;
            int length = c.getContentLength();
            if (length > MAX_IMAGE_BYTES) return null;
            try (BufferedInputStream in = new BufferedInputStream(c.getInputStream()); ByteArrayOutputStream out = new ByteArrayOutputStream(Math.max(16 * 1024, Math.min(MAX_IMAGE_BYTES, Math.max(0, length))))) {
                byte[] buffer = new byte[8192];
                int n;
                while ((n = in.read(buffer)) > 0) {
                    if (out.size() + n > MAX_IMAGE_BYTES) return null;
                    out.write(buffer, 0, n);
                }
                byte[] bytes = out.toByteArray();
                return bytes.length == 0 ? null : decodeLogo(bytes);
            }
        } catch (Throwable ignored) {
            return null;
        } finally {
            if (c != null) c.disconnect();
        }
    }

    private static Bitmap decodeLogo(byte[] bytes) {
        BitmapFactory.Options bounds = new BitmapFactory.Options();
        bounds.inJustDecodeBounds = true;
        BitmapFactory.decodeByteArray(bytes, 0, bytes.length, bounds);
        if (bounds.outWidth <= 0 || bounds.outHeight <= 0) return null;
        if ((long) bounds.outWidth * (long) bounds.outHeight > 64L * 1024L * 1024L) return null;
        int sample = 1;
        while (bounds.outWidth / sample > 192 || bounds.outHeight / sample > 192) sample <<= 1;
        BitmapFactory.Options options = new BitmapFactory.Options();
        options.inPreferredConfig = Bitmap.Config.ARGB_8888;
        options.inSampleSize = Math.max(1, sample);
        try { return BitmapFactory.decodeByteArray(bytes, 0, bytes.length, options); }
        catch (OutOfMemoryError ignored) { return null; }
    }

    public void trimMemory(int level) {
        if (closed) return;
        synchronized (cache) {
            if (level >= ComponentCallbacks2.TRIM_MEMORY_COMPLETE) cache.evictAll();
            else if (level >= ComponentCallbacks2.TRIM_MEMORY_RUNNING_LOW) cache.trimToSize(TRIMMED_CACHE_BYTES);
        }
    }

    public void close() {
        closed = true;
        pool.shutdownNow();
        synchronized (inFlightLock) { inFlight.clear(); }
        synchronized (cache) { cache.evictAll(); }
        main.removeCallbacksAndMessages(null);
    }
}
