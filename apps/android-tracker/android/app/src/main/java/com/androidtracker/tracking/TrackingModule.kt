package com.androidtracker.tracking

import android.content.Context
import android.content.Intent
import android.os.Build
import com.facebook.react.bridge.*
import kotlinx.coroutines.*
import java.time.Instant

class TrackingModule(reactContext: ReactApplicationContext) :
    ReactContextBaseJavaModule(reactContext) {

    override fun getName(): String = "TrackingModule"

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    @ReactMethod
    fun enableTracking(deviceId: String, token: String, wsUrl: String, promise: Promise) {
        try {
            reactApplicationContext
                .getSharedPreferences("wrany_tracking", Context.MODE_PRIVATE)
                .edit()
                .putString("device_id", deviceId)
                .putString("access_token", token)
                .putString("ws_url", wsUrl)
                .apply()

            val intent = Intent(reactApplicationContext, TrackingForegroundService::class.java).apply {
                action = TrackingForegroundService.ACTION_START
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                reactApplicationContext.startForegroundService(intent)
            } else {
                reactApplicationContext.startService(intent)
            }
            promise.resolve(null)
        } catch (e: Exception) {
            promise.reject("TRACKING_START_ERROR", e.message ?: "unknown error", e)
        }
    }

    @ReactMethod
    fun disableTracking(promise: Promise) {
        try {
            val intent = Intent(reactApplicationContext, TrackingForegroundService::class.java).apply {
                action = TrackingForegroundService.ACTION_STOP
            }
            reactApplicationContext.startService(intent)
            promise.resolve(null)
        } catch (e: Exception) {
            promise.reject("TRACKING_STOP_ERROR", e.message ?: "unknown error", e)
        }
    }

    @ReactMethod
    fun getTrackingStatus(promise: Promise) {
        scope.launch {
            try {
                val dao = LocationQueueDatabase.getInstance(reactApplicationContext).dao()
                val pending = dao.pendingCount()
                val failed = dao.failedCount()
                val lastSync = dao.lastSyncTime()
                val result = Arguments.createMap().apply {
                    putBoolean("serviceRunning", TrackingForegroundService.isRunning)
                    putInt("pendingCount", pending)
                    putInt("failedCount", failed)
                    putString("lastLocationTime", TrackingForegroundService.lastLocationTime)
                    putString("lastSyncTime", lastSync)
                }
                promise.resolve(result)
            } catch (e: Exception) {
                promise.reject("STATUS_ERROR", e.message ?: "unknown error", e)
            }
        }
    }

    @ReactMethod
    fun flushNow(promise: Promise) {
        promise.resolve(null)
    }

    @ReactMethod
    fun clearFailed(promise: Promise) {
        scope.launch {
            try {
                LocationQueueDatabase.getInstance(reactApplicationContext).dao().clearFailed()
                promise.resolve(null)
            } catch (e: Exception) {
                promise.reject("CLEAR_ERROR", e.message ?: "unknown error", e)
            }
        }
    }

    @ReactMethod
    fun updateToken(token: String, promise: Promise) {
        reactApplicationContext
            .getSharedPreferences("wrany_tracking", Context.MODE_PRIVATE)
            .edit()
            .putString("access_token", token)
            .apply()
        promise.resolve(null)
    }

    @ReactMethod
    fun cleanupOldPoints(promise: Promise) {
        scope.launch {
            try {
                val cutoff = Instant.now().minusSeconds(7 * 24 * 3600).toString()
                LocationQueueDatabase.getInstance(reactApplicationContext).dao().cleanupOld(cutoff)
                promise.resolve(null)
            } catch (e: Exception) {
                promise.reject("CLEANUP_ERROR", e.message ?: "unknown error", e)
            }
        }
    }
}
