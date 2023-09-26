package io.github.asutorufa.tailscaled

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat
import androidx.core.content.getSystemService
import appctr.Appctr
import kotlinx.coroutines.DelicateCoroutinesApi


class TailscaledService : Service() {
    private val notification by lazy { application.getSystemService<NotificationManager>()!! }

    private fun startNotification() {
        // Notifications on Oreo and above need a channel
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O)
            notification.createNotificationChannel(
                NotificationChannel(
                    packageName,
                    "tailscaled",
                    NotificationManager.IMPORTANCE_MIN
                )
            )

        startForeground(
            1,
            NotificationCompat.Builder(this, packageName)
                .setContentTitle("tailscaled")
                .setContentText("tailscaled")
                .setContentIntent(
                    PendingIntent.getActivity(
                        this,
                        0,
                        Intent(this, MainActivity::class.java),
                        PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
                    )
                )
                .build()
        )
    }

    @OptIn(DelicateCoroutinesApi::class)
    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        Log.d(TAG, "starting")
        if (Appctr.isRunning()) return START_STICKY
        start()
        startNotification()
        return START_STICKY
    }

    override fun onBind(p0: Intent?): IBinder? {
        return null
    }

    override fun onDestroy() {
        super.onDestroy()
        stopMe()
    }

    private fun stopMe() {
        stopForeground(STOP_FOREGROUND_REMOVE)
        Appctr.stop()
        stopSelf()
    }

    private fun start() {
        Appctr.start(
            "0.0.0.0:1056",
            "${applicationInfo.nativeLibraryDir}/libtailscaled.so",
            "${applicationInfo.dataDir}/tailscaled.sock",
            "${applicationInfo.dataDir}/state",
        ) {
            stopMe()
        }
    }

    companion object {
        private val TAG = TailscaledService::class.java.simpleName
    }
}
