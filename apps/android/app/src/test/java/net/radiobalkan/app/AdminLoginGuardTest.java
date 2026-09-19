package net.radiobalkan.app;

import org.junit.Test;

import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

public class AdminLoginGuardTest {
    private AdminLoginGuard guard() {
        return new AdminLoginGuard(new AdminRateLimiter(5, 30_000L));
    }

    @Test public void singleFlightRejectsSecondAttemptUntilCompletion() {
        AdminLoginGuard guard = guard();
        assertTrue(guard.tryBegin());
        assertTrue(guard.isRunning());
        assertFalse(guard.tryBegin());

        guard.complete(false, 1_000L);
        assertFalse(guard.isRunning());
        assertTrue(guard.tryBegin());
        guard.cancel();
    }

    @Test public void fiveFailedCompletionsTriggerLockout() {
        AdminLoginGuard guard = guard();
        for (int i = 0; i < 5; i++) {
            assertTrue(guard.tryBegin());
            guard.complete(false, 1_000L + i);
        }

        assertTrue(guard.isLocked(1_005L));
        assertTrue(guard.remainingSeconds(1_005L) > 0L);
        assertFalse(guard.isRunning());
    }

    @Test public void successfulCompletionClearsFailuresAndReleasesFlight() {
        AdminLoginGuard guard = guard();
        for (int i = 0; i < 4; i++) {
            assertTrue(guard.tryBegin());
            guard.complete(false, 2_000L + i);
        }

        assertTrue(guard.tryBegin());
        guard.complete(true, 2_010L);
        assertFalse(guard.isLocked(2_011L));
        assertFalse(guard.isRunning());

        for (int i = 0; i < 4; i++) {
            assertTrue(guard.tryBegin());
            guard.complete(false, 3_000L + i);
        }
        assertFalse(guard.isLocked(3_005L));
    }

    @Test public void cancelReleasesFlightWithoutCountingFailure() {
        AdminLoginGuard guard = guard();
        assertTrue(guard.tryBegin());
        guard.cancel();
        assertFalse(guard.isRunning());

        for (int i = 0; i < 4; i++) {
            assertTrue(guard.tryBegin());
            guard.complete(false, 4_000L + i);
        }
        assertFalse(guard.isLocked(4_005L));
    }
}
