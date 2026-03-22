package com.arc.sshqr.ssh

internal enum class TerminalViewportMode {
    FOLLOW_OUTPUT,
    SCROLLBACK,
}

internal data class TerminalViewportState(
    val mode: TerminalViewportMode = TerminalViewportMode.FOLLOW_OUTPUT,
    val anchorTopIndex: Int? = null,
)

internal object TerminalViewportLogic {
    fun syncState(topRow: Int, transcriptRows: Int): TerminalViewportState {
        if (transcriptRows <= 0 || topRow >= 0) {
            return TerminalViewportState()
        }
        return TerminalViewportState(
            mode = TerminalViewportMode.SCROLLBACK,
            anchorTopIndex = (transcriptRows + topRow).coerceAtLeast(0),
        )
    }

    fun restoreTopRow(
        state: TerminalViewportState,
        transcriptRows: Int,
        currentTopRow: Int,
    ): Pair<TerminalViewportState, Int> {
        if (transcriptRows <= 0) {
            return TerminalViewportState() to currentTopRow.coerceAtMost(0)
        }

        if (state.mode != TerminalViewportMode.SCROLLBACK) {
            return TerminalViewportState() to 0
        }

        val anchorTopIndex = state.anchorTopIndex?.coerceIn(0, transcriptRows - 1)
            ?: return TerminalViewportState() to 0
        val boundedTopRow = (anchorTopIndex - transcriptRows).coerceIn(-transcriptRows, -1)
        return TerminalViewportState(
            mode = TerminalViewportMode.SCROLLBACK,
            anchorTopIndex = transcriptRows + boundedTopRow,
        ) to boundedTopRow
    }
}
