package com.arc.sshqr.ssh

import android.view.KeyEvent
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class TerminalKeyPolicyTest {

    @Test
    fun `system back key is not handled by terminal`() {
        assertTrue(TerminalKeyPolicy.shouldLetSystemHandle(KeyEvent.KEYCODE_BACK))
    }

    @Test
    fun `terminal navigation keys stay handled in terminal`() {
        assertFalse(TerminalKeyPolicy.shouldLetSystemHandle(KeyEvent.KEYCODE_DPAD_LEFT))
        assertFalse(TerminalKeyPolicy.shouldLetSystemHandle(KeyEvent.KEYCODE_ESCAPE))
    }
}
