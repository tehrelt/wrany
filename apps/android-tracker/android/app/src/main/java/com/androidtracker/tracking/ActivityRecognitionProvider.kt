package com.androidtracker.tracking

import android.annotation.SuppressLint
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import android.util.Log
import com.google.android.gms.location.ActivityRecognition
import com.google.android.gms.location.ActivityTransition
import com.google.android.gms.location.ActivityTransitionRequest
import com.google.android.gms.location.DetectedActivity

/**
 * Subscribes to Activity Recognition Transition updates and delivers them to the
 * foreground service via a getService PendingIntent. This is the PRIMARY wake-up
 * trigger out of IDLE; it runs on low-power sensors (no GPS), so it is cheap.
 *
 * Requires ACTIVITY_RECOGNITION runtime permission (API 29+). If missing, start()
 * fails gracefully and the system falls back to sparse-GPS escalation.
 */
class ActivityRecognitionProvider(private val context: Context) {

    private companion object {
        const val TAG = "ActivityRecognition"
        const val REQUEST_CODE = 0xAC

        val DETECTED_TYPES = listOf(
            DetectedActivity.STILL,
            DetectedActivity.WALKING,
            DetectedActivity.RUNNING,
            DetectedActivity.ON_BICYCLE,
            DetectedActivity.IN_VEHICLE,
        )
    }

    private val client = ActivityRecognition.getClient(context)

    private val pendingIntent: PendingIntent by lazy {
        val intent = Intent(context, TrackingForegroundService::class.java).apply {
            action = TrackingForegroundService.ACTION_ACTIVITY
        }
        val flags = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_MUTABLE
        } else {
            PendingIntent.FLAG_UPDATE_CURRENT
        }
        PendingIntent.getService(context, REQUEST_CODE, intent, flags)
    }

    private val request: ActivityTransitionRequest by lazy {
        val transitions = buildList {
            for (type in DETECTED_TYPES) {
                add(transition(type, ActivityTransition.ACTIVITY_TRANSITION_ENTER))
                add(transition(type, ActivityTransition.ACTIVITY_TRANSITION_EXIT))
            }
        }
        ActivityTransitionRequest(transitions)
    }

    @SuppressLint("MissingPermission")
    fun start() {
        try {
            client.requestActivityTransitionUpdates(request, pendingIntent)
                .addOnSuccessListener { Log.i(TAG, "transition updates registered") }
                .addOnFailureListener { e -> Log.w(TAG, "register failed: ${e.message}") }
        } catch (e: SecurityException) {
            Log.w(TAG, "ACTIVITY_RECOGNITION permission missing; GPS fallback only")
        }
    }

    @SuppressLint("MissingPermission")
    fun stop() {
        try {
            client.removeActivityTransitionUpdates(pendingIntent)
        } catch (e: SecurityException) {
            Log.w(TAG, "remove failed: ${e.message}")
        }
    }

    private fun transition(activityType: Int, transitionType: Int): ActivityTransition =
        ActivityTransition.Builder()
            .setActivityType(activityType)
            .setActivityTransition(transitionType)
            .build()
}
