package net.radiobalkan.app;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import org.junit.Test;

public final class PlaybackLifecycleTest {
    @Test public void uiCommandDistinguishesPauseResumeAndRestart() {
        assertEquals(PlaybackLifecycle.UiCommand.PLAY,
                PlaybackLifecycle.uiCommand(false, false, false));
        assertEquals(PlaybackLifecycle.UiCommand.PAUSE,
                PlaybackLifecycle.uiCommand(true, true, false));
        assertEquals(PlaybackLifecycle.UiCommand.RESUME,
                PlaybackLifecycle.uiCommand(true, false, false));
        assertEquals(PlaybackLifecycle.UiCommand.PLAY,
                PlaybackLifecycle.uiCommand(true, false, true));
    }

    @Test public void explicitStopCannotResumeAnOldServiceSession() {
        assertFalse(PlaybackLifecycle.canResume(true, true, true));
        assertFalse(PlaybackLifecycle.canResume(true, false, true));
    }

    @Test public void pausedOrRecoverablePlaybackCanResume() {
        assertTrue(PlaybackLifecycle.canResume(false, true, false));
        assertTrue(PlaybackLifecycle.canResume(false, false, true));
        assertFalse(PlaybackLifecycle.canResume(false, false, false));
    }
}
