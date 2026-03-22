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

    fun shouldStartTwoFingerScroll(
        deltaX1: Float,
        deltaY1: Float,
        deltaX2: Float,
        deltaY2: Float,
        spanDelta: Float,
    ): Boolean {
        val absY1 = kotlin.math.abs(deltaY1)
        val absY2 = kotlin.math.abs(deltaY2)
        val maxTravel = maxOf(absY1, absY2)
        if (maxTravel < TWO_FINGER_SCROLL_MIN_DISTANCE_PX) {
            return false
        }
        val movingTogetherVertically =
            (absY1 >= TWO_FINGER_SCROLL_MIN_DISTANCE_PX && absY2 >= TWO_FINGER_SCROLL_MIN_DISTANCE_PX && deltaY1 * deltaY2 > 0f) ||
                absY1 >= TWO_FINGER_SCROLL_MIN_DISTANCE_PX * 2f ||
                absY2 >= TWO_FINGER_SCROLL_MIN_DISTANCE_PX * 2f
        if (!movingTogetherVertically) {
            return false
        }
        if (absY1 >= TWO_FINGER_SCROLL_MIN_DISTANCE_PX && absY1 < kotlin.math.abs(deltaX1) * TWO_FINGER_SCROLL_DIRECTION_BIAS) {
            return false
        }
        if (absY2 >= TWO_FINGER_SCROLL_MIN_DISTANCE_PX && absY2 < kotlin.math.abs(deltaX2) * TWO_FINGER_SCROLL_DIRECTION_BIAS) {
            return false
        }
        return kotlin.math.abs(spanDelta) <= maxTravel * TWO_FINGER_PINCH_TOLERANCE_RATIO + TWO_FINGER_PINCH_TOLERANCE_PX
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
    private const val TWO_FINGER_SCROLL_MIN_DISTANCE_PX = 12f
    private const val TWO_FINGER_SCROLL_DIRECTION_BIAS = 1.2f
    private const val TWO_FINGER_PINCH_TOLERANCE_RATIO = 0.6f
    private const val TWO_FINGER_PINCH_TOLERANCE_PX = 12f
}
