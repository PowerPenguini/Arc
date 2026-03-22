package com.arc.sshqr.ssh

import android.view.KeyEvent
import android.view.MotionEvent
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class TerminalTouchPolicyTest {

    @Test
    fun `down event is not consumed`() {
        assertFalse(
            TerminalTouchPolicy.shouldConsumeEvent(
                actionMasked = MotionEvent.ACTION_DOWN,
                pointerCount = 1,
                isSelectingText = false,
                activeArrowGesture = false,
                consumedByGestureDetector = false,
                deltaX = 0f,
                deltaY = 0f,
                topRow = 0,
            ),
        )
    }

    @Test
    fun `single tap up is consumed through gesture detector`() {
        assertTrue(
            TerminalTouchPolicy.shouldConsumeEvent(
                actionMasked = MotionEvent.ACTION_UP,
                pointerCount = 1,
                isSelectingText = false,
                activeArrowGesture = false,
                consumedByGestureDetector = true,
                deltaX = 0f,
                deltaY = 0f,
                topRow = 0,
            ),
        )
    }

    @Test
    fun `ordinary one finger drag is consumed so terminal does not scroll`() {
        assertTrue(
            TerminalTouchPolicy.shouldConsumeEvent(
                actionMasked = MotionEvent.ACTION_MOVE,
                pointerCount = 1,
                isSelectingText = false,
                activeArrowGesture = false,
                consumedByGestureDetector = false,
                deltaX = 0f,
                deltaY = 48f,
                topRow = 0,
            ),
        )
    }

    @Test
    fun `directional swipe starts arrow gesture at live bottom`() {
        assertTrue(
            TerminalTouchPolicy.shouldStartArrowGesture(
                deltaX = 0f,
                deltaY = 120f,
                topRow = 0,
            ),
        )
        assertEquals(
            KeyEvent.KEYCODE_DPAD_DOWN,
            TerminalTouchPolicy.resolveArrowKeyCode(
                deltaX = 0f,
                deltaY = 120f,
            ),
        )
    }

    @Test
    fun `directional swipe becomes arrow gesture in scrollback too`() {
        assertTrue(
            TerminalTouchPolicy.shouldStartArrowGesture(
                deltaX = 120f,
                deltaY = 0f,
                topRow = -1,
            ),
        )
    }

    @Test
    fun `active arrow gesture consumes move and up`() {
        assertTrue(
            TerminalTouchPolicy.shouldConsumeEvent(
                actionMasked = MotionEvent.ACTION_MOVE,
                pointerCount = 1,
                isSelectingText = false,
                activeArrowGesture = true,
                consumedByGestureDetector = false,
                deltaX = 140f,
                deltaY = 0f,
                topRow = 0,
            ),
        )
        assertTrue(
            TerminalTouchPolicy.shouldConsumeEvent(
                actionMasked = MotionEvent.ACTION_UP,
                pointerCount = 1,
                isSelectingText = false,
                activeArrowGesture = true,
                consumedByGestureDetector = false,
                deltaX = 140f,
                deltaY = 0f,
                topRow = 0,
            ),
        )
    }

    @Test
    fun `multi touch is passed through to allow pinch handling`() {
        assertFalse(
            TerminalTouchPolicy.shouldConsumeEvent(
                actionMasked = MotionEvent.ACTION_MOVE,
                pointerCount = 2,
                isSelectingText = false,
                activeArrowGesture = false,
                consumedByGestureDetector = false,
                deltaX = 100f,
                deltaY = 0f,
                topRow = 0,
            ),
        )
    }

    @Test
    fun `two finger parallel vertical move starts two finger scroll`() {
        assertTrue(
            TerminalTouchPolicy.shouldStartTwoFingerScroll(
                deltaX1 = 3f,
                deltaY1 = 28f,
                deltaX2 = -2f,
                deltaY2 = 24f,
                spanDelta = 4f,
            ),
        )
    }

    @Test
    fun `pinch does not get treated as two finger scroll`() {
        assertFalse(
            TerminalTouchPolicy.shouldStartTwoFingerScroll(
                deltaX1 = -20f,
                deltaY1 = 8f,
                deltaX2 = 22f,
                deltaY2 = -6f,
                spanDelta = 40f,
            ),
        )
    }
}
