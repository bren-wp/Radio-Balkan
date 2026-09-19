package net.radiobalkan.app;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;
import org.junit.Test;

public class AdminRateLimiterTest {
    @Test public void locksAfterFiveCompletedFailuresEvenWithoutDialogState() {
        AdminRateLimiter limiter = new AdminRateLimiter(5, 30_000L);
        long now = 10_000L;
        for (int i = 0; i < 4; i++) {
            limiter.recordFailure(now + i);
            assertFalse(limiter.isLocked(now + i));
        }
        limiter.recordFailure(now + 4);
        assertTrue(limiter.isLocked(now + 4));
        assertEquals(30L, limiter.remainingSeconds(now + 4));
    }

    @Test public void successClearsFailureAndLockoutState() {
        AdminRateLimiter limiter = new AdminRateLimiter(2, 30_000L);
        limiter.recordFailure(1_000L);
        limiter.recordFailure(1_001L);
        assertTrue(limiter.isLocked(1_001L));
        limiter.recordSuccess();
        assertFalse(limiter.isLocked(1_002L));
        limiter.recordFailure(1_003L);
        assertFalse(limiter.isLocked(1_003L));
    }

    @Test public void expiredLockoutAllowsFreshAttempts() {
        AdminRateLimiter limiter = new AdminRateLimiter(1, 30_000L);
        limiter.recordFailure(5_000L);
        assertTrue(limiter.isLocked(34_999L));
        assertFalse(limiter.isLocked(35_000L));
        limiter.recordFailure(35_001L);
        assertTrue(limiter.isLocked(35_001L));
    }
}
