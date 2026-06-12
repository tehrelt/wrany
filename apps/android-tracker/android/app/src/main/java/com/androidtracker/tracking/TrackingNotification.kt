package com.androidtracker.tracking

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat
import com.androidtracker.MainActivity

object TrackingNotification {
    const val CHANNEL_ID = "wrany_tracking"
    const val NOTIFICATION_ID = 1001

    fun createChannel(context: Context) {
        val channel = NotificationChannel(
            CHANNEL_ID,
            "Location Tracking",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "Visible while WR any% is collecting route data"
            setShowBadge(false)
        }
        context.getSystemService(NotificationManager::class.java)
            .createNotificationChannel(channel)
    }

    fun build(context: Context, pendingCount: Int = 0, wsConnected: Boolean = false): Notification {
        val stopIntent = Intent(context, TrackingForegroundService::class.java).apply {
            action = TrackingForegroundService.ACTION_STOP
        }
        val stopPending = PendingIntent.getService(
            context, 0, stopIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val openIntent = Intent(context, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP
        }
        val openPending = PendingIntent.getActivity(
            context, 1, openIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val statusText = if (wsConnected) {
            "Syncing to server"
        } else {
            "Offline — $pendingCount point(s) queued"
        }

        return NotificationCompat.Builder(context, CHANNEL_ID)
            .setContentTitle("WR any% — Tracking active")
            .setContentText(statusText)
            .setSmallIcon(android.R.drawable.ic_menu_mylocation)
            .setOngoing(true)
            .setSilent(true)
            .setContentIntent(openPending)
            .addAction(android.R.drawable.ic_media_pause, "Stop tracking", stopPending)
            .build()
    }
}
