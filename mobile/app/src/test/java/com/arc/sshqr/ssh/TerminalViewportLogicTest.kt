package com.arc.sshqr.ssh

import org.junit.Assert.assertEquals
import org.junit.Test

class TerminalViewportLogicTest {

    @Test
    fun `syncState follows output when top row is live bottom`() {
        val state = TerminalViewportLogic.syncState(topRow = 0, transcriptRows = 40)

        assertEquals(TerminalViewportState(), state)
    }

    @Test
    fun `syncState enters scrollback when top row is negative`() {
        val state = TerminalViewportLogic.syncState(topRow = -12, transcriptRows = 40)

        assertEquals(
            TerminalViewportState(
                mode = TerminalViewportMode.SCROLLBACK,
                anchorTopIndex = 28,
            ),
            state,
        )
    }

    @Test
    fun `restoreTopRow keeps live bottom when following output`() {
        val (restoredState, restoredTopRow) = TerminalViewportLogic.restoreTopRow(
            state = TerminalViewportState(),
            transcriptRows = 50,
            currentTopRow = -7,
        )

        assertEquals(TerminalViewportState(), restoredState)
        assertEquals(0, restoredTopRow)
    }

    @Test
    fun `restoreTopRow preserves anchor across transcript growth`() {
        val (restoredState, restoredTopRow) = TerminalViewportLogic.restoreTopRow(
            state = TerminalViewportState(
                mode = TerminalViewportMode.SCROLLBACK,
                anchorTopIndex = 28,
            ),
            transcriptRows = 60,
            currentTopRow = -5,
        )

        assertEquals(-32, restoredTopRow)
        assertEquals(
            TerminalViewportState(
                mode = TerminalViewportMode.SCROLLBACK,
                anchorTopIndex = 28,
            ),
            restoredState,
        )
    }

    @Test
    fun `restoreTopRow clamps invalid anchor into transcript`() {
        val (restoredState, restoredTopRow) = TerminalViewportLogic.restoreTopRow(
            state = TerminalViewportState(
                mode = TerminalViewportMode.SCROLLBACK,
                anchorTopIndex = 999,
            ),
            transcriptRows = 10,
            currentTopRow = -3,
        )

        assertEquals(-1, restoredTopRow)
        assertEquals(
            TerminalViewportState(
                mode = TerminalViewportMode.SCROLLBACK,
                anchorTopIndex = 9,
            ),
            restoredState,
        )
    }
}
