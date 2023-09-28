package io.github.asutorufa.tailscaled

import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.content.ServiceConnection
import android.os.Build
import android.os.Bundle
import android.os.IBinder
import android.os.Message
import android.os.Messenger
import android.util.Log
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import androidx.annotation.RequiresApi
import androidx.fragment.app.Fragment
import io.github.asutorufa.tailscaled.databinding.FragmentFirstBinding

/**
 * A simple [Fragment] subclass as the default destination in the navigation.
 */
class FirstFragment : Fragment() {
    private lateinit var binding: FragmentFirstBinding

    private var isRunning = false
    private val bReceiver: BroadcastReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: Context, intent: Intent) {
            Log.d(tag, "onReceive: ${intent.action}")

            when (intent.action) {
                "START" -> {
                    binding.buttonFirst.text = "Stop"
                    isRunning = true
                }

                "STOP" -> {
                    binding.buttonFirst.text = "Start"
                    isRunning = false
                }
            }
        }
    }

    /** Messenger for communicating with the service.  */
    private var mService: Messenger? = null

    /** Flag indicating whether we have called bind on the service.  */
    private var bound: Boolean = false

    /**
     * Class for interacting with the main interface of the service.
     */
    private val mConnection = object : ServiceConnection {

        override fun onServiceConnected(className: ComponentName, service: IBinder) {
            // This is called when the connection with the service has been
            // established, giving us the object we can use to
            // interact with the service.  We are communicating with the
            // service using a Messenger, so here we get a client-side
            // representation of that from the raw IBinder object.
            mService = Messenger(service)
            bound = true
            mService?.send(
                Message.obtain(
                    null,
                    TailscaledService.MSG_SAY_HELLO,
                    0,
                    0
                )
            )
        }

        override fun onServiceDisconnected(className: ComponentName) {
            // This is called when the connection with the service has been
            // unexpectedly disconnected -- that is, its process crashed.
            mService = null
            bound = false
        }
    }

    override fun onCreateView(
        inflater: LayoutInflater, container: ViewGroup?,
        savedInstanceState: Bundle?
    ): View {
        binding = FragmentFirstBinding.inflate(inflater, container, false)
        return binding.root
    }

    @RequiresApi(Build.VERSION_CODES.O)
    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)

        val intentFilter =  IntentFilter().apply {
            addAction("START")
            addAction("STOP")
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU)
            activity?.registerReceiver(bReceiver,intentFilter, Context.RECEIVER_EXPORTED)
        else activity?.registerReceiver(bReceiver, intentFilter)

        // Bind to the service
        Intent(activity, TailscaledService::class.java).also { intent ->
            activity?.bindService(intent, mConnection, Context.BIND_AUTO_CREATE)
        }

        binding.buttonFirst.setOnClickListener {
            Log.d("activity", "is running: $isRunning")
            if (isRunning) mService?.send(
                Message.obtain(
                    null,
                    TailscaledService.MSG_STOP,
                    0,
                    0
                )
            )
            else
                activity?.startService(
                    Intent(
                        activity,
                        TailscaledService::class.java
                    )
                )
        }
    }
}