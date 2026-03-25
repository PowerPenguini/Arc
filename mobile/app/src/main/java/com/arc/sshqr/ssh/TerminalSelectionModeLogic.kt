package com.arc.sshqr.ssh

internal object TerminalSelectionModeLogic {
    fun isSelectionActive(copyModeActive: Boolean, viewSelectingText: Boolean): Boolean {
        return copyModeActive || viewSelectingText
    }
}
