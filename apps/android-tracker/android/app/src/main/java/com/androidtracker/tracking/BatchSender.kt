package com.androidtracker.tracking

import android.content.Context
import android.util.Log
import kotlinx.coroutines.*
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONArray
import org.json.JSONObject
import java.time.Instant
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

class BatchSender(
    private val context: Context,
    private val dao: LocationQueueDao,
    private val scope: CoroutineScope,
) {
    companion object {
        private const val TAG = "BatchSender"
        private const val BATCH_SIZE = 20
        private const val FLUSH_INTERVAL_MS = 15_000L
        private const val MAX_BACKOFF_MS = 30_000L
        // Cap consecutive token-refresh attempts so a server that rejects even
        // freshly-minted tokens cannot drive an infinite refresh→connect→401 loop.
        private const val MAX_REFRESH_ATTEMPTS = 3
    }

    private val client = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(60, TimeUnit.SECONDS)
        .writeTimeout(10, TimeUnit.SECONDS)
        .build()

    private var ws: WebSocket? = null
    private val sessionAccepted = AtomicBoolean(false)
    private var reconnectAttempt = 0
    private var flushJob: Job? = null
    private var inflight: List<LocationPoint> = emptyList()

    // Single-flight guard: concurrent failures must not fire parallel refreshes.
    private val refreshing = AtomicBoolean(false)
    private var refreshAttempt = 0

    @Volatile var wsConnected = false
        private set

    @Volatile var wsLastError: String? = null
        private set

    // Set when the refresh token itself is rejected — reconnect loop stops until
    // the user re-logs in (which calls reconnectNow with a fresh token in prefs).
    @Volatile var authExpired = false
        private set

    fun start() {
        connect()
        scheduleFlush()
    }

    fun stop() {
        flushJob?.cancel()
        ws?.close(1000, "tracking stopped")
        ws = null
        sessionAccepted.set(false)
        wsConnected = false
    }

    fun flush() {
        scope.launch { trySendBatch() }
    }

    fun reconnectNow() {
        ws?.close(1000, "force reconnect")
        ws = null
        sessionAccepted.set(false)
        wsConnected = false
        wsLastError = null
        reconnectAttempt = 0
        refreshAttempt = 0
        authExpired = false
        connect()
    }

    private fun connect() {
        if (authExpired) {
            Log.w(TAG, "Auth expired — not connecting until re-login")
            return
        }
        val token = readPrefs("access_token")
        if (token == null) {
            // No access token yet — mint one from the refresh token if we have it.
            if (readPrefs("refresh_token") != null && readPrefs("api_url") != null) {
                refreshAndReconnect()
            } else {
                Log.w(TAG, "No token, delaying reconnect")
                scheduleReconnect()
            }
            return
        }
        val wsUrl = readPrefs("ws_url") ?: "ws://10.0.2.2:8080/v1/ws/tracker"
        val request = Request.Builder()
            .url("$wsUrl?access_token=$token")
            .build()
        ws = client.newWebSocket(request, wsListener)
    }

    private val wsListener = object : WebSocketListener() {
        override fun onOpen(webSocket: WebSocket, response: Response) {
            Log.i(TAG, "WS open, sending session.start")
            reconnectAttempt = 0
            refreshAttempt = 0
            wsConnected = true
            val deviceId = readPrefs("device_id") ?: ""
            val msg = JSONObject().apply {
                put("type", "session.start")
                put("request_id", "req_session_${System.currentTimeMillis()}")
                put("payload", JSONObject().apply {
                    put("device_id", deviceId)
                    put("app_version", "0.1.0")
                    put("platform", "android")
                })
            }
            webSocket.send(msg.toString())
        }

        override fun onMessage(webSocket: WebSocket, text: String) {
            try {
                val msg = JSONObject(text)
                when (msg.optString("type")) {
                    "session.accepted" -> {
                        sessionAccepted.set(true)
                        Log.i(TAG, "Session accepted")
                        scope.launch { trySendBatch() }
                    }
                    "location.batch.ack" -> handleAck(msg.getJSONObject("payload"))
                    "ping" -> webSocket.send(JSONObject().put("type", "pong").toString())
                    "error" -> Log.w(TAG, "Server error: $text")
                }
            } catch (e: Exception) {
                Log.e(TAG, "Message parse error: ${e.message}")
            }
        }

        override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
            val reason = buildString {
                append(t.message ?: t.javaClass.simpleName)
                if (response != null) append(" (HTTP ${response.code})")
            }
            Log.w(TAG, "WS failure: $reason")
            wsLastError = reason
            wsConnected = false
            sessionAccepted.set(false)
            ws = null
            returnInflightToPending()
            // The gateway rejects an invalid/expired token at the HTTP upgrade
            // with 401 (403 if forbidden). Reconnecting with the same stale token
            // would loop forever — refresh it first.
            if (response?.code == 401 || response?.code == 403) {
                refreshAndReconnect()
            } else {
                scheduleReconnect()
            }
        }

        override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
            wsConnected = false
            sessionAccepted.set(false)
            returnInflightToPending()
        }

        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
            ws = null
        }
    }

    private fun scheduleReconnect() {
        val delayMs = minOf(1_000L shl reconnectAttempt.coerceAtMost(5), MAX_BACKOFF_MS)
        reconnectAttempt++
        Log.d(TAG, "Reconnect in ${delayMs}ms (attempt $reconnectAttempt)")
        scope.launch {
            delay(delayMs)
            connect()
        }
    }

    // Refresh the access token (server-side) then reconnect. Single-flight: a
    // second caller while a refresh is in progress is a no-op. On refresh failure
    // the connection is marked auth-expired and the reconnect loop stops.
    private fun refreshAndReconnect() {
        if (!refreshing.compareAndSet(false, true)) return
        scope.launch {
            try {
                if (doRefresh()) {
                    reconnectAttempt = 0
                    connect()
                } else {
                    markAuthExpired()
                }
            } finally {
                refreshing.set(false)
            }
        }
    }

    // POST /v1/auth/refresh with the stored refresh token; persists the new token
    // pair to prefs on success. Returns false on any failure (caller stops retrying).
    private fun doRefresh(): Boolean {
        if (refreshAttempt >= MAX_REFRESH_ATTEMPTS) {
            Log.w(TAG, "Max refresh attempts reached")
            return false
        }
        refreshAttempt++
        val rt = readPrefs("refresh_token")
        val apiUrl = readPrefs("api_url")
        if (rt == null || apiUrl == null) {
            Log.w(TAG, "Cannot refresh: missing refresh_token or api_url")
            return false
        }
        return try {
            val reqBody = JSONObject().put("refresh_token", rt).toString()
                .toRequestBody("application/json".toMediaType())
            val req = Request.Builder()
                .url("$apiUrl/v1/auth/refresh")
                .post(reqBody)
                .build()
            client.newCall(req).execute().use { resp ->
                if (!resp.isSuccessful) {
                    Log.w(TAG, "Refresh failed: HTTP ${resp.code}")
                    return false
                }
                val data = JSONObject(resp.body?.string() ?: "").getJSONObject("data")
                writePrefs(
                    "access_token" to data.getString("access_token"),
                    "refresh_token" to data.getString("refresh_token"),
                )
                Log.i(TAG, "Token refreshed")
                true
            }
        } catch (e: Exception) {
            Log.e(TAG, "Refresh error: ${e.message}")
            false
        }
    }

    private fun markAuthExpired() {
        authExpired = true
        wsLastError = "session expired — please re-login"
        Log.w(TAG, "Auth refresh failed — stopping reconnect until re-login")
    }

    private fun scheduleFlush() {
        flushJob = scope.launch {
            while (isActive) {
                delay(FLUSH_INTERVAL_MS)
                trySendBatch()
            }
        }
    }

    private suspend fun trySendBatch() {
        if (!sessionAccepted.get()) return
        if (inflight.isNotEmpty()) return
        val points = dao.getPending(BATCH_SIZE)
        if (points.isEmpty()) return
        inflight = points
        sendBatch(points)
    }

    private fun sendBatch(points: List<LocationPoint>) {
        val deviceId = readPrefs("device_id") ?: ""
        val events = JSONArray()
        for (p in points) {
            events.put(JSONObject().apply {
                put("event_id", p.id)
                put("recorded_at", p.recordedAt)
                put("lat", p.lat)
                put("lon", p.lon)
                put("accuracy_m", p.accuracyM)
                p.speedMps?.let { put("speed_mps", it) }
                p.bearingDeg?.let { put("bearing_deg", it) }
                p.activityType?.let { put("activity_type", it) }
                p.activityConfidence?.let { put("activity_confidence", it) }
                p.batteryLevel?.let { put("battery_level", it) }
            })
        }
        val msg = JSONObject().apply {
            put("type", "location.batch")
            put("request_id", "batch_${System.currentTimeMillis()}")
            put("payload", JSONObject().apply {
                put("device_id", deviceId)
                put("events", events)
            })
        }
        val sent = ws?.send(msg.toString()) ?: false
        if (!sent) {
            returnInflightToPending()
        } else {
            Log.d(TAG, "Sent batch of ${points.size} points")
        }
    }

    private fun handleAck(payload: JSONObject) {
        scope.launch {
            val now = Instant.now().toString()
            val accepted = payload.getJSONArray("accepted").let { a ->
                (0 until a.length()).map { a.getString(it) }
            }
            val duplicated = payload.getJSONArray("duplicated").let { a ->
                (0 until a.length()).map { a.getString(it) }
            }
            val rejected = payload.getJSONArray("rejected")

            val toAck = accepted + duplicated
            if (toAck.isNotEmpty()) dao.markAcked(toAck, now)

            for (i in 0 until rejected.length()) {
                val r = rejected.getJSONObject(i)
                dao.markFailed(r.getString("event_id"), r.getString("reason"), now)
            }

            inflight = emptyList()
            Log.d(TAG, "ACK: ${accepted.size} accepted, ${duplicated.size} dup, ${rejected.length()} rejected")

            // Immediately try next batch
            trySendBatch()
        }
    }

    private fun returnInflightToPending() {
        if (inflight.isEmpty()) return
        val ids = inflight.map { it.id }
        inflight = emptyList()
        scope.launch { dao.returnToPending(ids, Instant.now().toString()) }
    }

    private fun readPrefs(key: String): String? =
        context.getSharedPreferences("wrany_tracking", Context.MODE_PRIVATE)
            .getString(key, null)

    private fun writePrefs(vararg pairs: Pair<String, String>) {
        context.getSharedPreferences("wrany_tracking", Context.MODE_PRIVATE)
            .edit()
            .apply { pairs.forEach { (k, v) -> putString(k, v) } }
            .apply()
    }
}
