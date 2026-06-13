package com.androidtracker.tracking

import com.google.android.gms.location.DetectedActivity

/** Coarse local motion state. Backend remains the source of truth for trips. */
enum class MotionState { IDLE, MOTION_CANDIDATE, MOVING, STOP_CANDIDATE }

/** Activity classes we react to, mapped from Google's DetectedActivity. */
enum class MotionActivity(val wire: String) {
    STILL("still"),
    WALKING("walking"),
    RUNNING("running"),
    ON_BICYCLE("on_bicycle"),
    IN_VEHICLE("in_vehicle"),
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
