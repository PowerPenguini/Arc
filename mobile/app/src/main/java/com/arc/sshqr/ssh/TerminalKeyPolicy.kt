package com.arc.sshqr.ssh

import android.view.KeyEvent

internal object TerminalKeyPolicy {
    fun shouldLetSystemHandle(keyCode: Int): Boolean {
        return keyCode == KeyEvent.KEYCODE_BACK
    }
}
