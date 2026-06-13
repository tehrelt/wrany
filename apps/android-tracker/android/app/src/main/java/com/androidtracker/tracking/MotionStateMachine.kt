package com.androidtracker.tracking

import android.location.Location
import android.util.Log

/**
 * Client-side motion state machine.
 *
 * Purpose: wake up dense GPS quickly (driven by Activity Recognition) and tag
 * outgoing location events with the current activity. It is deliberately NOT a
 * trip detector — the backend (tracking-worker) remains the source of truth for
 * trip start/finish. Here we only decide:
 *
 *   - the GPS cadence (dense vs sparse), and
 *   - a coarse local state used for tagging and for emitting tracker state hints.
 *
 * States:
 *   IDLE             — stationary, sparse GPS (battery saver). Wake-up is normally
 *                      an Activity Recognition ENTER; sparse GPS is only a fallback.
 *   MOTION_CANDIDATE — AR (or GPS fallback) reported motion. Dense GPS is on, but
 *                      we have NOT confirmed real movement yet.
 *   MOVING           — GPS window confirmed motion (duration + distance + accuracy
 *                      + plausible speed). This is the "trip is likely active" hint.
 *   STOP_CANDIDATE   — STILL ENTER while moving. Dense GPS stays on; we only settle
 *                      to IDLE after the user is stationary for [STOP_DURATION_MS].
 */
class MotionStateMachine(
    private val onCadence: (active: Boolean) -> Unit,
    private val onStateEvent: (state: MotionState, activity: MotionActivity, confidence: Int) -> Unit,
) {
    private companion object {
        const val TAG = "MotionSM"

        // GPS confirmation window (MOTION_CANDIDATE -> MOVING).
        const val MIN_MOTION_DURATION_MS = 8_000L
        const val MIN_MOTION_DISTANCE_M = 30.0
        const val MAX_ACCURACY_M = 50.0          // ignore worse fixes for decisions

        // False start: AR said "moving" but GPS never confirmed.
        const val CANDIDATE_TIMEOUT_MS = 60_000L

        // Stop confirmation (STOP_CANDIDATE -> IDLE). Spec: 3-5 minutes.
        const val STOP_DURATION_MS = 3 * 60_000L
        const val RESUME_SPEED_MPS = 1.2f        // movement that cancels a stop
        const val RESUME_DISTANCE_M = 40.0

        // IDLE GPS fallback: escalate without AR if sparse GPS shows real motion.
        const val FALLBACK_SPEED_MPS = 1.5f
        const val FALLBACK_DISTANCE_M = 35.0
    }

    var state: MotionState = MotionState.MOVING // assume moving until proven still
        private set
    var activity: MotionActivity = MotionActivity.UNKNOWN
        private set
    var confidence: Int = 0
        private set

    private var candidateStartMs = 0L
    private var candidateStartLoc: Location? = null
    private var stopStartMs = 0L
    private var movingRefLoc: Location? = null
    private var idleAnchor: Location? = null

    // --- Inputs -------------------------------------------------------------

    fun onActivityTransition(act: MotionActivity, isEnter: Boolean, nowMs: Long, conf: Int) {
        if (act == MotionActivity.UNKNOWN) return
        confidence = conf
        if (act != MotionActivity.STILL) activity = act

        when {
            isEnter && act == MotionActivity.STILL -> handleStillEnter(nowMs)
            isEnter && act.isMotion -> handleMotionEnter(act, nowMs)
            // EXIT transitions carry no decision on their own; ENTER drives state.
        }
    }

    fun onLocation(loc: Location, nowMs: Long) {
        val usable = loc.accuracy <= MAX_ACCURACY_M
        when (state) {
            MotionState.IDLE -> if (usable) idleFallback(loc, nowMs)
            MotionState.MOTION_CANDIDATE -> if (usable) tryConfirmMotion(loc, nowMs)
            MotionState.STOP_CANDIDATE -> evaluateStop(loc, nowMs, usable)
            MotionState.MOVING -> movingRefLoc = loc
        }
    }

    /** Periodic safety tick (timeouts that should fire even without new fixes). */
    fun onTick(nowMs: Long) {
        when (state) {
            MotionState.MOTION_CANDIDATE ->
                if (nowMs - candidateStartMs >= CANDIDATE_TIMEOUT_MS) {
                    Log.i(TAG, "MOTION_CANDIDATE timed out -> IDLE (false start)")
                    transition(MotionState.IDLE, active = false)
                }
            MotionState.STOP_CANDIDATE ->
                if (nowMs - stopStartMs >= STOP_DURATION_MS) {
                    Log.i(TAG, "stop confirmed by tick -> IDLE")
                    transition(MotionState.IDLE, active = false)
                }
            else -> {}
        }
    }

    // --- Transitions --------------------------------------------------------

    private fun handleMotionEnter(act: MotionActivity, nowMs: Long) {
        when (state) {
            MotionState.IDLE, MotionState.MOTION_CANDIDATE -> beginMotionCandidate(nowMs)
            MotionState.STOP_CANDIDATE -> {
                Log.i(TAG, "motion during stop window -> MOVING (resume)")
                transition(MotionState.MOVING, active = true)
            }
            MotionState.MOVING -> { /* stay moving, activity tag already updated */ }
        }
    }

    private fun handleStillEnter(nowMs: Long) {
        when (state) {
            MotionState.MOVING -> {
                stopStartMs = nowMs
                transition(MotionState.STOP_CANDIDATE, active = true)
            }
            MotionState.MOTION_CANDIDATE -> {
                Log.i(TAG, "STILL during candidate -> IDLE (false start)")
                transition(MotionState.IDLE, active = false)
            }
            else -> {}
        }
    }

    private fun beginMotionCandidate(nowMs: Long) {
        candidateStartMs = nowMs
        candidateStartLoc = null
        transition(MotionState.MOTION_CANDIDATE, active = true)
    }

    private fun tryConfirmMotion(loc: Location, nowMs: Long) {
        val start = candidateStartLoc ?: loc.also { candidateStartLoc = it }
        val elapsed = nowMs - candidateStartMs
        val distance = loc.distanceTo(start)
        if (elapsed >= MIN_MOTION_DURATION_MS &&
            distance >= MIN_MOTION_DISTANCE_M &&
            speedPlausible(loc)
        ) {
            movingRefLoc = loc
            Log.i(TAG, "motion confirmed (${distance.toInt()}m/${elapsed}ms) -> MOVING")
            transition(MotionState.MOVING, active = true)
        }
    }

    private fun evaluateStop(loc: Location, nowMs: Long, usable: Boolean) {
        if (usable && movementResumed(loc)) {
            Log.i(TAG, "movement resumed -> MOVING")
            transition(MotionState.MOVING, active = true)
            return
        }
        if (nowMs - stopStartMs >= STOP_DURATION_MS) {
            transition(MotionState.IDLE, active = false)
        }
    }

    private fun idleFallback(loc: Location, nowMs: Long) {
        val anchor = idleAnchor
        if (anchor == null) {
            idleAnchor = loc
            return
        }
        val moved = loc.distanceTo(anchor) >= FALLBACK_DISTANCE_M
        val fast = loc.hasSpeed() && loc.speed >= FALLBACK_SPEED_MPS
        if (moved || fast) {
            Log.i(TAG, "GPS fallback detected motion -> MOTION_CANDIDATE")
            idleAnchor = null
            beginMotionCandidate(nowMs)
        }
    }

    private fun movementResumed(loc: Location): Boolean {
        if (loc.hasSpeed() && loc.speed >= RESUME_SPEED_MPS) return true
        val ref = movingRefLoc ?: return false
        return loc.distanceTo(ref) >= RESUME_DISTANCE_M
    }

    private fun speedPlausible(loc: Location): Boolean {
        if (!loc.hasSpeed()) return true // don't block confirmation on missing speed
        val s = loc.speed
        return when (activity) {
            MotionActivity.WALKING -> s in 0.3f..3.5f
            MotionActivity.RUNNING -> s in 1.8f..7.0f
            MotionActivity.ON_BICYCLE -> s in 1.5f..15.0f
            MotionActivity.IN_VEHICLE -> s in 2.0f..70.0f
            else -> s >= 0.3f
        }
    }

    private fun transition(next: MotionState, active: Boolean) {
        if (next == MotionState.IDLE) {
            idleAnchor = null
            activity = MotionActivity.STILL
        }
        val changed = next != state
        state = next
        onCadence(active)
        if (changed) onStateEvent(next, activity, confidence)
    }
}
