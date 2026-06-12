package com.androidtracker.tracking

import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.IBinder
import android.util.Log
import kotlinx.coroutines.*
import java.time.Instant

class TrackingForegroundService : Service() {

    companion object {
        private const val TAG = "TrackingService"
        const val ACTION_START = "com.androidtracker.tracking.START"
        const val ACTION_STOP = "com.androidtracker.tracking.STOP"

        @Volatile var isRunning = false
        @Volatile var lastLocationTime: String? = null
    }

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private lateinit var db: LocationQueueDatabase
    private lateinit var locationProvider: LocationProvider
    private lateinit var batchSender: BatchSender
    private var notificationUpdateJob: Job? = null

    override fun onCreate() {
        super.onCreate()
        db = LocationQueueDatabase.getInstance(this)
        locationProvider = LocationProvider(this) { point ->
            lastLocationTime = Instant.now().toString()
            scope.launch { db.dao().insert(point) }
        }
        batchSender = BatchSender(this, db.dao(), scope)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> stopSelf()
            else -> if (!isRunning) startTracking()
        }
        return START_STICKY
    }

    private fun startTracking() {
        isRunning = true
        startForeground(TrackingNotification.NOTIFICATION_ID, TrackingNotification.build(this))

        val prefs = getSharedPreferences("wrany_tracking", Context.MODE_PRIVATE)
        locationProvider.deviceId = prefs.getString("device_id", "") ?: ""

        locationProvider.start()
        batchSender.start()
        startNotificationUpdater()

        Log.i(TAG, "Tracking started (deviceId=${locationProvider.deviceId})")
    }

    private fun startNotificationUpdater() {
        notificationUpdateJob = scope.launch {
            while (isActive) {
                delay(10_000)
                val pending = db.dao().pendingCount()
                val nm = getSystemService(android.app.NotificationManager::class.java)
                nm.notify(
                    TrackingNotification.NOTIFICATION_ID,
                    TrackingNotification.build(this@TrackingForegroundService, pending, batchSender.wsConnected)
                )
            }
        }
    }

    override fun onDestroy() {
        isRunning = false
        notificationUpdateJob?.cancel()
        locationProvider.stop()
        batchSender.stop()
        scope.cancel()
        stopForeground(STOP_FOREGROUND_REMOVE)
        Log.i(TAG, "Tracking stopped")
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null
}
