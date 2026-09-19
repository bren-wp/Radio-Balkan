package net.radiobalkan.app;

import java.util.concurrent.atomic.AtomicBoolean;

/**
 * Process-lifetime administrator login guard.
 * It intentionally survives Activity recreation, while adminMode itself remains Activity-local.
 */
final class AdminLoginGuard {
    private static final AdminLoginGuard SHARED =
            new AdminLoginGuard(new AdminRateLimiter(5, 30_000L));

    private final AdminRateLimiter limiter;
    private final AtomicBoolean running = new AtomicBoolean();

    AdminLoginGuard(AdminRateLimiter limiter) {
        if (limiter == null) throw new IllegalArgumentException("limiter");
        this.limiter = limiter;
    }

    static AdminLoginGuard shared() {
        return SHARED;
    }

    boolean isLocked(long nowMs) {
        return limiter.isLocked(nowMs);
    }

    long remainingSeconds(long nowMs) {
        return limiter.remainingSeconds(nowMs);
    }

    boolean tryBegin() {
        return running.compareAndSet(false, true);
    }

    void complete(boolean accepted, long nowMs) {
        if (accepted) limiter.recordSuccess();
        else limiter.recordFailure(nowMs);
        running.set(false);
    }

    void cancel() {
        running.set(false);
    }

    boolean isRunning() {
        return running.get();
    }
}
