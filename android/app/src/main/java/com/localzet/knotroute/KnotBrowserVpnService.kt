package com.localzet.knotroute

import android.app.*
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.ProxyInfo
import android.net.VpnService
import android.os.Build
import android.os.IBinder
import android.os.ParcelFileDescriptor
import java.io.FileInputStream

class KnotBrowserVpnService : VpnService() {
    private var tunnel: ParcelFileDescriptor? = null
    private var drainThread: Thread? = null

    override fun onCreate() {
        super.onCreate()
        val manager = getSystemService(NotificationManager::class.java)
        if (Build.VERSION.SDK_INT >= 26) {
            manager.createNotificationChannel(NotificationChannel(CHANNEL, getString(R.string.browser_channel_name), NotificationManager.IMPORTANCE_LOW))
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            stopTunnel()
            stopSelf()
            return START_NOT_STICKY
        }
        if (!KnotRuntime.ready()) {
            stopSelf()
            return START_NOT_STICKY
        }
        startNotification()
        if (tunnel == null) startTunnel()
        return START_STICKY
    }

    private fun startTunnel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) {
            stopSelf()
            return
        }
        val builder = Builder()
            .setSession(getString(R.string.app_name))
            .setMtu(1500)
            .addAddress("10.73.0.2", 32)
            .addRoute("0.0.0.0", 0)
        builder.setHttpProxy(ProxyInfo.buildDirectProxy("127.0.0.1", 19478))

        // Keep the beta integration scoped to installed browsers. Browsers that
        // honor Android's VPN HTTP proxy send HTTP(S) to KnotRoute; ordinary apps
        // are not captured by this compatibility layer.
        val browserPackages = listOf(
            "com.android.chrome", "com.chrome.beta", "com.chrome.dev",
            "com.brave.browser", "com.microsoft.emmx", "com.opera.browser",
            "com.vivaldi.browser", "org.chromium.chrome"
        )
        var allowed = 0
        browserPackages.forEach { pkg ->
            try { builder.addAllowedApplication(pkg); allowed++ } catch (_: Exception) { }
        }
        if (allowed == 0) {
            stopSelf()
            return
        }
        tunnel = builder.establish() ?: run { stopSelf(); return }
        drainThread = Thread {
            // Protocols that ignore the proxy (for example QUIC) are deliberately
            // discarded so compatible browsers fall back to proxied TCP/HTTPS.
            try {
                FileInputStream(tunnel!!.fileDescriptor).use { input ->
                    val buffer = ByteArray(32768)
                    while (!Thread.currentThread().isInterrupted && input.read(buffer) >= 0) { }
                }
            } catch (_: Throwable) { }
        }.also { it.name = "KnotRoute-browser-vpn"; it.start() }
    }

    private fun startNotification() {
        val open = PendingIntent.getActivity(this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        val notification = Notification.Builder(this, CHANNEL)
            .setSmallIcon(R.drawable.ic_knotroute_status)
            .setContentTitle(getString(R.string.browser_integration))
            .setContentText(getString(R.string.browser_integration_running))
            .setContentIntent(open)
            .setOngoing(true)
            .build()
        if (Build.VERSION.SDK_INT >= 34) startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        else startForeground(NOTIFICATION_ID, notification)
    }

    private fun stopTunnel() {
        drainThread?.interrupt(); drainThread = null
        tunnel?.close(); tunnel = null
    }

    override fun onDestroy() { stopTunnel(); super.onDestroy() }
    override fun onBind(intent: Intent?): IBinder? = super.onBind(intent)

    companion object {
        const val ACTION_START = "com.localzet.knotroute.BROWSER_VPN_START"
        const val ACTION_STOP = "com.localzet.knotroute.BROWSER_VPN_STOP"
        private const val CHANNEL = "knotroute-browser"
        private const val NOTIFICATION_ID = 8
    }
}
