package com.androidtracker.tracking

import com.google.android.gms.location.DetectedActivity

/** Coarse local motion state. Backend remains the source of truth for trips. */
enum class MotionState { IDLE, MOTION_CANDIDATE, MOVING, STOP_CANDIDATE }

/**
 * Activity classes we react to, mapped from Google's DetectedActivity.
 *
 * `wire` MUST match the backend's accepted activity_type contract
 * (libs/events + tracking-gateway domain.ValidActivityTypes):
 * walking / running / bicycle / vehicle / stationary / unknown.
 * Any other value is rejected server-side as invalid_activity_type.
 */
enum class MotionActivity(val wire: String) {
    STILL("stationary"),
    WALKING("walking"),
    RUNNING("running"),
    ON_BICYCLE("bicycle"),
    IN_VEHICLE("vehicle"),
    UNKNOWN("unknown");

    val isMotion: Boolean
        get() = this == WALKING || this == RUNNING || this == ON_BICYCLE || this == IN_VEHICLE

    companion object {
        fun fromDetected(type: Int): MotionActivity = when (type) {
            DetectedActivity.STILL -> STILL
            DetectedActivity.WALKING, DetectedActivity.ON_FOOT -> WALKING
            DetectedActivity.RUNNING -> RUNNING
            DetectedActivity.ON_BICYCLE -> ON_BICYCLE
            DetectedActivity.IN_VEHICLE -> IN_VEHICLE
            else -> UNKNOWN
        }
    }
}
