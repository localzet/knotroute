package com.localzet.knotroute

import android.app.*
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder

class KnotService : Service() {
    private val operationLock = Any()

    override fun onCreate() {
        super.onCreate()
        val manager = getSystemService(NotificationManager::class.java)
        if (Build.VERSION.SDK_INT >= 26) {
            manager.createNotificationChannel(NotificationChannel(CHANNEL, getString(R.string.channel_name), NotificationManager.IMPORTANCE_LOW))
        }
        startForegroundNotification(getString(R.string.notification_starting), false)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val restart = intent?.action == ACTION_RESTART
        Thread {
            synchronized(operationLock) {
                if (restart) KnotRuntime.stop()
                try {
                    KnotRuntime.start(applicationContext)
                    startForegroundNotification(getString(R.string.notification_running), false)
                } catch (error: Throwable) {
                    startForegroundNotification(getString(R.string.notification_failed, KnotRuntime.lastError ?: error.javaClass.simpleName), true)
                }
            }
        }.start()
        return START_STICKY
    }

    private fun startForegroundNotification(message: String, failed: Boolean) {
        val open = PendingIntent.getActivity(this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        val notification = Notification.Builder(this, CHANNEL)
            .setSmallIcon(if (failed) android.R.drawable.stat_notify_error else android.R.drawable.stat_sys_download_done)
            .setContentTitle(getString(R.string.app_name))
            .setContentText(message)
            .setContentIntent(open)
            .setOngoing(!failed)
            .build()
        if (Build.VERSION.SDK_INT >= 34) startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        else startForeground(NOTIFICATION_ID, notification)
    }

    override fun onDestroy() {
        synchronized(operationLock) { KnotRuntime.stop() }
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    companion object {
        const val CHANNEL = "knotroute-overlay"
        const val ACTION_RESTART = "com.localzet.knotroute.RESTART"
        private const val NOTIFICATION_ID = 7
    }
}
