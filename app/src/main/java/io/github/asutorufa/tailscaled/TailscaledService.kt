package io.github.asutorufa.tailscaled

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.os.Message
import android.os.Messenger
import android.util.Log
import androidx.core.app.NotificationCompat
import androidx.core.content.getSystemService
import appctr.Appctr


class TailscaledService : Service() {
    private val notification by lazy { application.getSystemService<NotificationManager>()!! }

    override fun onCreate() {
        super.onCreate()
        mMessenger = Messenger(IncomingHandler(this))
    }

    private fun startNotification() {
        // Notifications on Oreo and above need a channel
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O)
            notification.createNotificationChannel(
                NotificationChannel(
                    packageName,
                    "Tailscaled",
                    NotificationManager.IMPORTANCE_MIN
                )
            )

        startForeground(
            1,
            NotificationCompat.Builder(this, packageName)
                .setContentTitle("Tailscaled は実行中です")
                .setSmallIcon(R.drawable.ic_launcher_foreground)
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

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        Log.d(TAG, "starting")
        if (Appctr.isRunning()) return START_STICKY
        start()
        startNotification()
        applicationContext.sendBroadcast(Intent("START"))
        return START_STICKY
    }

    override fun onBind(p0: Intent?): IBinder? {
        return mMessenger.binder
    }

    override fun onDestroy() {
        super.onDestroy()
        Log.d(TAG, "try to stopMe")
        stopMe()
    }

    private fun stopMe() {
        if (!Appctr.isRunning()) return
        stopForeground(STOP_FOREGROUND_REMOVE)
        Appctr.stop()
        stopSelf()
        applicationContext.sendBroadcast(Intent("STOP"))
    }

    /**
     * Target we publish for clients to send messages to IncomingHandler.
     */
    private lateinit var mMessenger: Messenger

    /**
     * Handler of incoming messages from clients.
     */
    private class IncomingHandler(
        var context: TailscaledService,
        private val applicationContext: Context = context.applicationContext
    ) : Handler(Looper.getMainLooper()) {
        override fun handleMessage(msg: Message) {
            Log.d(TAG,"receive message: ${msg.what}")
            when (msg.what) {
                MSG_SAY_HELLO -> {
                    applicationContext.sendBroadcast(Intent(if (Appctr.isRunning()) "START" else "STOP"))
                }

                MSG_STOP -> {
                    context.stopMe()
                }

                MSG_START -> context.onStartCommand(null, 0, 0)

                else -> super.handleMessage(msg)
            }
        }
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
        const val MSG_SAY_HELLO = 0
        const val MSG_STOP = 1
        const val MSG_START = 2
    }
}
