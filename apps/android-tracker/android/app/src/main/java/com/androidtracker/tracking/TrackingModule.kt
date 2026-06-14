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
    fun enableTracking(
        deviceId: String,
        token: String,
        refreshToken: String,
        wsUrl: String,
        apiUrl: String,
        promise: Promise,
    ) {
        try {
            reactApplicationContext
                .getSharedPreferences("wrany_tracking", Context.MODE_PRIVATE)
                .edit()
                .putString("device_id", deviceId)
                .putString("access_token", token)
                .putString("refresh_token", refreshToken)
                .putString("ws_url", wsUrl)
                .putString("api_url", apiUrl)
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
                val wsStatus = when {
                    !TrackingForegroundService.isRunning -> "disconnected"
                    TrackingForegroundService.wsConnected -> "connected"
                    else -> "connecting"
                }
                val result = Arguments.createMap().apply {
                    putBoolean("serviceRunning", TrackingForegroundService.isRunning)
                    putInt("pendingCount", pending)
                    putInt("failedCount", failed)
                    putString("lastLocationTime", TrackingForegroundService.lastLocationTime)
                    putString("lastSyncTime", lastSync)
                    putString("wsStatus", wsStatus)
                    putString("wsLastError", TrackingForegroundService.wsLastError)
                    putBoolean("authExpired", TrackingForegroundService.authExpired)
                }
                promise.resolve(result)
            } catch (e: Exception) {
                promise.reject("STATUS_ERROR", e.message ?: "unknown error", e)
            }
        }
    }

    @ReactMethod
    fun reconnectWs(promise: Promise) {
        try {
            val intent = Intent(reactApplicationContext, TrackingForegroundService::class.java).apply {
                action = TrackingForegroundService.ACTION_RECONNECT
            }
            reactApplicationContext.startService(intent)
            promise.resolve(null)
        } catch (e: Exception) {
            promise.reject("RECONNECT_ERROR", e.message ?: "unknown error", e)
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

    // Pushes a freshly-issued token pair from JS into native prefs. Refresh tokens
    // rotate on the server, so after a JS-side refresh the service must receive the
    // new pair or its own next refresh would use a revoked token.
    @ReactMethod
    fun updateTokens(accessToken: String, refreshToken: String, promise: Promise) {
        reactApplicationContext
            .getSharedPreferences("wrany_tracking", Context.MODE_PRIVATE)
            .edit()
            .putString("access_token", accessToken)
            .putString("refresh_token", refreshToken)
            .apply()
        promise.resolve(null)
    }

    // Exposes the token pair currently held by the service so JS can absorb a
    // background refresh (the service may have rotated the refresh token while
    // the app was closed).
    @ReactMethod
    fun getStoredTokens(promise: Promise) {
        val prefs = reactApplicationContext
            .getSharedPreferences("wrany_tracking", Context.MODE_PRIVATE)
        val result = Arguments.createMap().apply {
            putString("accessToken", prefs.getString("access_token", null))
            putString("refreshToken", prefs.getString("refresh_token", null))
        }
        promise.resolve(result)
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
