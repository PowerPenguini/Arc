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
    fun `quick directional swipe can trigger flick hold`() {
        assertTrue(
            TerminalTouchPolicy.shouldTriggerFlickHold(
                deltaX = 0f,
                deltaY = -120f,
                topRow = 0,
                gestureDurationMs = 180L,
                maxFlickDurationMs = 250L,
            ),
        )
    }

    @Test
    fun `slow swipe does not trigger flick hold`() {
        assertFalse(
            TerminalTouchPolicy.shouldTriggerFlickHold(
                deltaX = 0f,
                deltaY = -120f,
                topRow = 0,
                gestureDurationMs = 320L,
                maxFlickDurationMs = 250L,
            ),
        )
    }

    @Test
    fun `short quick move does not trigger flick hold`() {
        assertFalse(
            TerminalTouchPolicy.shouldTriggerFlickHold(
                deltaX = 40f,
                deltaY = 0f,
                topRow = 0,
                gestureDurationMs = 100L,
                maxFlickDurationMs = 250L,
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
    fun `multi touch is passed through to custom two finger handler`() {
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
                totalDeltaX1 = 3f,
                totalDeltaY1 = 28f,
                totalDeltaX2 = -2f,
                totalDeltaY2 = 24f,
            ),
        )
    }

    @Test
    fun `slow parallel vertical move still starts two finger scroll`() {
        assertTrue(
            TerminalTouchPolicy.shouldStartTwoFingerScroll(
                totalDeltaX1 = 2f,
                totalDeltaY1 = 11f,
                totalDeltaX2 = -1f,
                totalDeltaY2 = 10f,
            ),
        )
    }

    @Test
    fun `opposing two finger motion does not get treated as scroll`() {
        assertFalse(
            TerminalTouchPolicy.shouldStartTwoFingerScroll(
                totalDeltaX1 = -20f,
                totalDeltaY1 = 14f,
                totalDeltaX2 = 22f,
                totalDeltaY2 = -12f,
            ),
        )
    }

    @Test
    fun `mostly horizontal two finger motion does not get treated as scroll`() {
        assertFalse(
            TerminalTouchPolicy.shouldStartTwoFingerScroll(
                totalDeltaX1 = 26f,
                totalDeltaY1 = 12f,
                totalDeltaX2 = 24f,
                totalDeltaY2 = 10f,
            ),
        )
    }
}
