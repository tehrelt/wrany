package com.androidtracker.tracking

import android.annotation.SuppressLint
import android.content.Context
import android.location.Location
import android.os.Looper
import com.google.android.gms.location.*
import java.time.Instant
import java.util.UUID

class LocationProvider(
    private val context: Context,
    private val onPoint: (LocationPoint) -> Unit,
) {
    private val fusedClient = LocationServices.getFusedLocationProviderClient(context)

    private val request = LocationRequest.Builder(
        Priority.PRIORITY_HIGH_ACCURACY, 12_000L
    ).apply {
        setMinUpdateIntervalMillis(5_000L)
        setMinUpdateDistanceMeters(15f)
    }.build()

    private val callback = object : LocationCallback() {
        override fun onLocationResult(result: LocationResult) {
            result.lastLocation?.let { handleLocation(it) }
        }
    }

    var deviceId: String = ""

    @SuppressLint("MissingPermission")
    fun start() {
        fusedClient.requestLocationUpdates(request, callback, Looper.getMainLooper())
    }

    fun stop() {
        fusedClient.removeLocationUpdates(callback)
    }

    private fun handleLocation(loc: Location) {
        if (deviceId.isEmpty()) return
        val now = Instant.now().toString()
        val point = LocationPoint(
            id = UUID.randomUUID().toString(),
            deviceId = deviceId,
            recordedAt = Instant.ofEpochMilli(loc.time).toString(),
            lat = loc.latitude,
            lon = loc.longitude,
            accuracyM = loc.accuracy.toDouble(),
            speedMps = if (loc.hasSpeed()) loc.speed.toDouble() else null,
            bearingDeg = if (loc.hasBearing()) loc.bearing.toDouble() else null,
            createdAt = now,
            updatedAt = now,
        )
        onPoint(point)
    }
}
