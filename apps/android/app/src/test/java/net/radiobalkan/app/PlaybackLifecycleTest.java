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

    @Test public void stopThenPlayUsesFreshSessionInsteadOfResume() {
        assertFalse(PlaybackLifecycle.canResume(true, true, true));
        assertEquals(PlaybackLifecycle.UiCommand.PLAY,
                PlaybackLifecycle.uiCommand(true, false, true));
        assertEquals(PlaybackLifecycle.UiCommand.PAUSE,
                PlaybackLifecycle.uiCommand(true, true, false));
        assertEquals(PlaybackLifecycle.UiCommand.RESUME,
                PlaybackLifecycle.uiCommand(true, false, false));
    }

    @Test public void adjacentNavigationWrapsAndHandlesMissingSelection() {
        assertEquals(1, PlaybackLifecycle.adjacentIndex(3, 0, 1));
        assertEquals(0, PlaybackLifecycle.adjacentIndex(3, 2, 1));
        assertEquals(2, PlaybackLifecycle.adjacentIndex(3, 0, -1));
        assertEquals(0, PlaybackLifecycle.adjacentIndex(3, -1, 1));
        assertEquals(2, PlaybackLifecycle.adjacentIndex(3, -1, -1));
        assertEquals(-1, PlaybackLifecycle.adjacentIndex(0, -1, 1));
        assertEquals(-1, PlaybackLifecycle.adjacentIndex(3, 1, 0));
    }
}
