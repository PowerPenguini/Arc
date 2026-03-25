package com.arc.sshqr.ssh

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class TerminalSelectionModeLogicTest {

    @Test
    fun `selection is active when copy mode callback is active`() {
        assertTrue(
            TerminalSelectionModeLogic.isSelectionActive(
                copyModeActive = true,
                viewSelectingText = false,
            ),
        )
    }

    @Test
    fun `selection is active when terminal view is selecting`() {
        assertTrue(
            TerminalSelectionModeLogic.isSelectionActive(
                copyModeActive = false,
                viewSelectingText = true,
            ),
        )
    }

    @Test
    fun `selection is inactive when neither source reports selection`() {
        assertFalse(
            TerminalSelectionModeLogic.isSelectionActive(
                copyModeActive = false,
                viewSelectingText = false,
            ),
        )
    }
}
