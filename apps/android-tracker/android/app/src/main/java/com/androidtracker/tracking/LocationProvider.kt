package com.androidtracker.tracking

import android.annotation.SuppressLint
import android.content.Context
import android.location.Location
import android.os.Looper
import android.util.Log
import com.google.android.gms.location.*

/**
 * Thin GPS source. It does NOT decide cadence by itself — the MotionStateMachine
 * drives that via [setActive]. The client never drops points for data quality;
 * every fix is forwarded raw and the backend filters noise.
 *
 *  - ACTIVE: high-accuracy GPS, dense (~1-2 s) while moving.
 *  - IDLE:   balanced-power, sparse (~20 s) as a fallback while stationary.
 */
class LocationProvider(
    private val context: Context,
    private val onLocation: (Location) -> Unit,
) {
    private companion object {
        const val TAG = "LocationProvider"
        const val ACTIVE_INTERVAL_MS = 2_000L
        const val ACTIVE_MIN_INTERVAL_MS = 1_000L
        const val IDLE_INTERVAL_MS = 20_000L
        const val IDLE_MIN_INTERVAL_MS = 10_000L
    }

    private val fusedClient = LocationServices.getFusedLocationProviderClient(context)

    private val activeRequest = LocationRequest.Builder(
        Priority.PRIORITY_HIGH_ACCURACY, ACTIVE_INTERVAL_MS
    ).apply {
        setMinUpdateIntervalMillis(ACTIVE_MIN_INTERVAL_MS)
        setMinUpdateDistanceMeters(0f)
    }.build()

    private val idleRequest = LocationRequest.Builder(
        Priority.PRIORITY_BALANCED_POWER_ACCURACY, IDLE_INTERVAL_MS
    ).apply {
        setMinUpdateIntervalMillis(IDLE_MIN_INTERVAL_MS)
        setMinUpdateDistanceMeters(0f)
    }.build()

    private val callback = object : LocationCallback() {
        override fun onLocationResult(result: LocationResult) {
            // Keep every location in the batch, not just the last one.
            for (loc in result.locations) onLocation(loc)
        }
    }

    @Volatile private var active = true

    @SuppressLint("MissingPermission")
    fun start() {
        active = true
        fusedClient.requestLocationUpdates(activeRequest, callback, Looper.getMainLooper())
    }

    fun stop() {
        fusedClient.removeLocationUpdates(callback)
    }

    /** Switch GPS cadence. Called by the motion state machine. */
    @SuppressLint("MissingPermission")
    fun setActive(value: Boolean) {
        if (value == active) return
        active = value
        fusedClient.removeLocationUpdates(callback)
        val request = if (value) activeRequest else idleRequest
        fusedClient.requestLocationUpdates(request, callback, Looper.getMainLooper())
        Log.i(TAG, "cadence -> ${if (value) "ACTIVE" else "IDLE"}")
    }
}
