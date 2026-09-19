package net.radiobalkan.app;

final class AdminRateLimiter {
    private final int maxFailures;
    private final long lockoutMs;
    private int failures;
    private long lockedUntilMs;

    AdminRateLimiter(int maxFailures, long lockoutMs) {
        if (maxFailures < 1) throw new IllegalArgumentException("maxFailures");
        if (lockoutMs < 1) throw new IllegalArgumentException("lockoutMs");
        this.maxFailures = maxFailures;
        this.lockoutMs = lockoutMs;
    }

    synchronized boolean isLocked(long nowMs) {
        if (lockedUntilMs == 0L) return false;
        if (nowMs >= lockedUntilMs) {
            lockedUntilMs = 0L;
            return false;
        }
        return true;
    }

    synchronized long remainingSeconds(long nowMs) {
        if (!isLocked(nowMs)) return 0L;
        return Math.max(1L, (lockedUntilMs - nowMs + 999L) / 1000L);
    }

    synchronized void recordFailure(long nowMs) {
        if (isLocked(nowMs)) return;
        failures++;
        if (failures >= maxFailures) {
            failures = 0;
            lockedUntilMs = nowMs + lockoutMs;
        }
    }

    synchronized void recordSuccess() {
        failures = 0;
        lockedUntilMs = 0L;
    }
}
