package net.radiobalkan.app;

final class PlaybackLifecycle {
    enum UiCommand { PLAY, PAUSE, RESUME }

    private PlaybackLifecycle() { }

    static UiCommand uiCommand(boolean sameStation, boolean playing, boolean explicitlyStopped) {
        if (!sameStation || explicitlyStopped) return UiCommand.PLAY;
        return playing ? UiCommand.PAUSE : UiCommand.RESUME;
    }

    static boolean canResume(boolean explicitlyStopped, boolean hasPlayer, boolean hasCandidates) {
        return !explicitlyStopped && (hasPlayer || hasCandidates);
    }

    static boolean canNavigate(int size, boolean hasCurrent) {
        return hasCurrent && size > 1;
    }

    static int adjacentIndex(int size, int currentIndex, int delta) {
        if (size <= 0 || delta == 0) return -1;
        if (currentIndex < 0 || currentIndex >= size) return delta > 0 ? 0 : size - 1;
        return Math.floorMod(currentIndex + (delta > 0 ? 1 : -1), size);
    }
}
