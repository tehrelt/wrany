package com.androidtracker.tracking

import android.app.Service
import android.content.Context
import android.content.Intent
import android.location.Location
import android.os.IBinder
import android.util.Log
import com.google.android.gms.location.ActivityTransitionResult
import kotlinx.coroutines.*
import java.time.Instant
import java.util.UUID

class TrackingForegroundService : Service() {

    companion object {
        private const val TAG = "TrackingService"
        const val ACTION_START = "com.androidtracker.tracking.START"
        const val ACTION_STOP = "com.androidtracker.tracking.STOP"
        const val ACTION_RECONNECT = "com.androidtracker.tracking.RECONNECT"
        const val ACTION_ACTIVITY = "com.androidtracker.tracking.ACTIVITY"

        private const val MOTION_TICK_MS = 15_000L

        @Volatile var isRunning = false
        @Volatile var lastLocationTime: String? = null
        @Volatile var motionState: String = MotionState.MOVING.name
        @Volatile private var senderRef: BatchSender? = null

        val wsConnected: Boolean get() = senderRef?.wsConnected ?: false
        val wsLastError: String? get() = senderRef?.wsLastError
        val authExpired: Boolean get() = senderRef?.authExpired ?: false
    }

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private lateinit var db: LocationQueueDatabase
    private lateinit var locationProvider: LocationProvider
    private lateinit var activityProvider: ActivityRecognitionProvider
    private lateinit var motion: MotionStateMachine
    private lateinit var batchSender: BatchSender
    private var deviceId: String = ""
    private var notificationUpdateJob: Job? = null
    private var motionTickJob: Job? = null

    override fun onCreate() {
        super.onCreate()
        db = LocationQueueDatabase.getInstance(this)

        motion = MotionStateMachine(
            onCadence = { active -> locationProvider.setActive(active) },
            onStateEvent = { state, activity, confidence ->
                motionState = state.name
                Log.i(TAG, "state=$state activity=${activity.wire} conf=$confidence")
            },
        )

        locationProvider = LocationProvider(this) { loc ->
            lastLocationTime = Instant.now().toString()
            motion.onLocation(loc, System.currentTimeMillis())
            val point = toPoint(loc)
            scope.launch { db.dao().insert(point) }
        }

        activityProvider = ActivityRecognitionProvider(this)
        batchSender = BatchSender(this, db.dao(), scope)
        senderRef = batchSender
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> stopSelf()
            ACTION_RECONNECT -> senderRef?.reconnectNow()
            ACTION_ACTIVITY -> handleActivityResult(intent)
            else -> if (!isRunning) startTracking()
        }
        return START_STICKY
    }

    private fun startTracking() {
        isRunning = true
        startForeground(TrackingNotification.NOTIFICATION_ID, TrackingNotification.build(this))

        val prefs = getSharedPreferences("wrany_tracking", Context.MODE_PRIVATE)
        deviceId = prefs.getString("device_id", "") ?: ""

        locationProvider.start()
        activityProvider.start()
        batchSender.start()
        startNotificationUpdater()
        startMotionTicker()

        Log.i(TAG, "Tracking started (deviceId=$deviceId)")
    }

    private fun handleActivityResult(intent: Intent) {
        if (!ActivityTransitionResult.hasResult(intent)) return
        val result = ActivityTransitionResult.extractResult(intent) ?: return
        val now = System.currentTimeMillis()
        for (event in result.transitionEvents) {
            val activity = MotionActivity.fromDetected(event.activityType)
            val isEnter =
                event.transitionType == com.google.android.gms.location.ActivityTransition.ACTIVITY_TRANSITION_ENTER
            Log.i(TAG, "AR ${activity.wire} ${if (isEnter) "ENTER" else "EXIT"}")
            // Transition API does not expose confidence; use a fixed high value.
            motion.onActivityTransition(activity, isEnter, now, conf = 90)
        }
    }

    private fun toPoint(loc: Location): LocationPoint {
        val now = Instant.now().toString()
        return LocationPoint(
            id = UUID.randomUUID().toString(),
            deviceId = deviceId,
            recordedAt = Instant.ofEpochMilli(loc.time).toString(),
            lat = loc.latitude,
            lon = loc.longitude,
            accuracyM = loc.accuracy.toDouble(),
            speedMps = if (loc.hasSpeed()) loc.speed.toDouble() else null,
            bearingDeg = if (loc.hasBearing()) loc.bearing.toDouble() else null,
            activityType = motion.activity.wire,
            activityConfidence = motion.confidence / 100.0,
            createdAt = now,
            updatedAt = now,
        )
    }

    private fun startMotionTicker() {
        motionTickJob = scope.launch {
            while (isActive) {
                delay(MOTION_TICK_MS)
                motion.onTick(System.currentTimeMillis())
            }
        }
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
        motionTickJob?.cancel()
        locationProvider.stop()
        activityProvider.stop()
        batchSender.stop()
        senderRef = null
        scope.cancel()
        stopForeground(STOP_FOREGROUND_REMOVE)
        Log.i(TAG, "Tracking stopped")
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null
}
