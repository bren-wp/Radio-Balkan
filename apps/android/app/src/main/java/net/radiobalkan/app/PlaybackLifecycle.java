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
}
