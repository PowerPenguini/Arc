package com.arc.sshqr.ssh

import android.view.MotionEvent

internal object TerminalTouchPolicy {
    @Suppress("UNUSED_PARAMETER")
    fun shouldConsumeEvent(
        actionMasked: Int,
        pointerCount: Int,
        isSelectingText: Boolean,
        activeArrowGesture: Boolean,
        consumedByGestureDetector: Boolean,
        deltaX: Float,
        deltaY: Float,
        topRow: Int,
    ): Boolean {
        if (pointerCount >= 2 || isSelectingText) {
            return false
        }

        return when (actionMasked) {
            MotionEvent.ACTION_DOWN -> false
            MotionEvent.ACTION_MOVE -> true

            MotionEvent.ACTION_UP,
            MotionEvent.ACTION_CANCEL -> activeArrowGesture || consumedByGestureDetector
            else -> consumedByGestureDetector
        }
    }

    @Suppress("UNUSED_PARAMETER")
    fun shouldStartArrowGesture(deltaX: Float, deltaY: Float, topRow: Int): Boolean {
        val horizontal = kotlin.math.abs(deltaX) > kotlin.math.abs(deltaY)
        val primary = if (horizontal) kotlin.math.abs(deltaX) else kotlin.math.abs(deltaY)
        val secondary = if (horizontal) kotlin.math.abs(deltaY) else kotlin.math.abs(deltaX)
        if (primary < SWIPE_ARROW_MIN_DISTANCE_PX) {
            return false
        }
        return primary >= secondary * SWIPE_ARROW_DIRECTION_BIAS
    }

    fun shouldTriggerFlickHold(
        deltaX: Float,
        deltaY: Float,
        topRow: Int,
        gestureDurationMs: Long,
        maxFlickDurationMs: Long,
    ): Boolean {
        if (gestureDurationMs > maxFlickDurationMs) {
            return false
        }
        return shouldStartArrowGesture(
            deltaX = deltaX,
            deltaY = deltaY,
            topRow = topRow,
        )
    }

    fun shouldStartTwoFingerScroll(
        totalDeltaX1: Float,
        totalDeltaY1: Float,
        totalDeltaX2: Float,
        totalDeltaY2: Float,
    ): Boolean {
        val absY1 = kotlin.math.abs(totalDeltaY1)
        val absY2 = kotlin.math.abs(totalDeltaY2)
        val averageVerticalTravel = (absY1 + absY2) / 2f
        if (averageVerticalTravel < TWO_FINGER_SCROLL_MIN_DISTANCE_PX) {
            return false
        }
        if (totalDeltaY1 == 0f || totalDeltaY2 == 0f || totalDeltaY1 * totalDeltaY2 <= 0f) {
            return false
        }
        val averageHorizontalTravel = (kotlin.math.abs(totalDeltaX1) + kotlin.math.abs(totalDeltaX2)) / 2f
        if (averageVerticalTravel < averageHorizontalTravel * TWO_FINGER_SCROLL_DIRECTION_BIAS) {
            return false
        }
        val verticalTravelGap = kotlin.math.abs(absY1 - absY2)
        if (verticalTravelGap > averageVerticalTravel * TWO_FINGER_SCROLL_VERTICAL_ALIGNMENT_TOLERANCE_RATIO + TWO_FINGER_SCROLL_VERTICAL_ALIGNMENT_TOLERANCE_PX) {
            return false
        }
        return true
    }

    fun resolveArrowKeyCode(deltaX: Float, deltaY: Float): Int? {
        val horizontal = kotlin.math.abs(deltaX) > kotlin.math.abs(deltaY)
        val distance = if (horizontal) kotlin.math.abs(deltaX) else kotlin.math.abs(deltaY)
        if (distance < SWIPE_ARROW_MIN_DISTANCE_PX) {
            return null
        }
        return if (horizontal) {
            if (deltaX > 0) android.view.KeyEvent.KEYCODE_DPAD_RIGHT else android.view.KeyEvent.KEYCODE_DPAD_LEFT
        } else {
            if (deltaY > 0) android.view.KeyEvent.KEYCODE_DPAD_DOWN else android.view.KeyEvent.KEYCODE_DPAD_UP
        }
    }

    private const val SWIPE_ARROW_MIN_DISTANCE_PX = 96
    private const val SWIPE_ARROW_DIRECTION_BIAS = 1.4f
    private const val TWO_FINGER_SCROLL_MIN_DISTANCE_PX = 10f
    private const val TWO_FINGER_SCROLL_DIRECTION_BIAS = 1.05f
    private const val TWO_FINGER_SCROLL_VERTICAL_ALIGNMENT_TOLERANCE_RATIO = 0.7f
    private const val TWO_FINGER_SCROLL_VERTICAL_ALIGNMENT_TOLERANCE_PX = 18f
}
