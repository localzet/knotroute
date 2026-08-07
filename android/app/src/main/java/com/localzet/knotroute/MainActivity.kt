package com.localzet.knotroute

import android.Manifest
import android.annotation.SuppressLint
import android.app.*
import android.content.*
import android.content.pm.PackageManager
import android.graphics.Color
import android.net.http.SslError
import android.os.*
import android.security.KeyChain
import android.view.*
import android.webkit.*
import android.widget.*
import androidx.webkit.ProxyConfig
import androidx.webkit.ProxyController
import androidx.webkit.WebViewFeature
import java.io.ByteArrayInputStream
import java.security.cert.CertificateFactory
import java.util.concurrent.Executor

class MainActivity : Activity() {
    private lateinit var web: WebView
    private lateinit var address: EditText
    private lateinit var status: TextView
    private val mainExecutor = Executor { runOnUiThread(it) }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 40)
        startForegroundService(Intent(this, KnotService::class.java))
        buildUi()
        waitForCore(0)
    }

    @SuppressLint("SetJavaScriptEnabled")
    private fun buildUi() {
        val root = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setBackgroundColor(Color.rgb(18,18,18)) }
        val bar = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL; setPadding(12,12,12,8) }
        address = EditText(this).apply { hint = "https://… .knot"; setSingleLine(); setTextColor(Color.WHITE); setHintTextColor(Color.GRAY); layoutParams = LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f) }
        fun button(label: String, action: () -> Unit) = Button(this).apply { text = label; setOnClickListener { action() } }
        bar.addView(address); bar.addView(button("Go") { navigate() }); bar.addView(button("CA") { installCa() }); bar.addView(button("⋮") { settings() })
        status = TextView(this).apply { setPadding(16,4,16,8); setTextColor(Color.LTGRAY); text = "Starting KnotRoute…" }
        web = WebView(this).apply {
            settings.javaScriptEnabled = true
            settings.domStorageEnabled = true
            settings.loadsImagesAutomatically = true
            webViewClient = object : WebViewClient() {
                override fun onReceivedSslError(view: WebView?, handler: SslErrorHandler?, error: SslError?) {
                    // Never bypass TLS errors. The local CA must be explicitly trusted by the user.
                    handler?.cancel()
                    Toast.makeText(this@MainActivity, "TLS trust failed. Install the KnotRoute CA from the CA button.", Toast.LENGTH_LONG).show()
                }
            }
        }
        root.addView(bar); root.addView(status); root.addView(web, LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f)); setContentView(root)
    }

    private fun waitForCore(attempt: Int) {
        if (KnotRuntime.ready()) {
            configureProxy()
            status.text = "Connected · ${KnotRuntime.nodeAddress()}"
            return
        }
        if (attempt > 100) { status.text = "Failed: ${KnotRuntime.lastError ?: "core did not start"}"; return }
        Handler(Looper.getMainLooper()).postDelayed({ waitForCore(attempt + 1) }, 100)
    }

    private fun configureProxy() {
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.PROXY_OVERRIDE)) {
            status.text = "This Android WebView does not support process proxy override"
            return
        }
        val endpoint = KnotRuntime.proxyUrl().removePrefix("http://")
        val proxy = ProxyConfig.Builder().addProxyRule(endpoint).build()
        ProxyController.getInstance().setProxyOverride(proxy, mainExecutor) { status.text = "Connected · ${KnotRuntime.nodeAddress()}" }
    }

    private fun navigate() {
        var value = address.text.toString().trim()
        if (value.isEmpty()) return
        if (!value.contains("://")) value = "https://$value"
        web.loadUrl(value)
    }

    private fun installCa() {
        if (!KnotRuntime.ready()) { Toast.makeText(this, "KnotRoute is still starting", Toast.LENGTH_SHORT).show(); return }
        try {
            val pem = KnotRuntime.rootCaPem().toByteArray()
            val cert = CertificateFactory.getInstance("X.509").generateCertificate(ByteArrayInputStream(pem))
            val intent = KeyChain.createInstallIntent().apply {
                putExtra(KeyChain.EXTRA_NAME, "KnotRoute Local Root CA")
                putExtra(KeyChain.EXTRA_CERTIFICATE, cert.encoded)
            }
            startActivity(intent)
        } catch (e: Exception) { Toast.makeText(this, e.message ?: "CA installation failed", Toast.LENGTH_LONG).show() }
    }

    private fun settings() {
        val prefs = getSharedPreferences("knotroute", Context.MODE_PRIVATE)
        val box = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setPadding(40,8,40,8) }
        val network = EditText(this).apply { hint = "Network ID (blank = default)"; setText(prefs.getString("network_id", "")) }
        val beacons = EditText(this).apply { hint = "Beacon URLs, comma separated"; setText(prefs.getString("beacons", "")) }
        val hops = EditText(this).apply { hint = "Circuit hops"; inputType = 2; setText(prefs.getInt("circuit_hops", 3).toString()) }
        box.addView(network); box.addView(beacons); box.addView(hops)
        AlertDialog.Builder(this).setTitle("KnotRoute settings").setView(box).setNegativeButton("Cancel", null).setPositiveButton("Save & restart") { _, _ ->
            prefs.edit().putString("network_id", network.text.toString().trim()).putString("beacons", beacons.text.toString().trim()).putInt("circuit_hops", hops.text.toString().toIntOrNull()?.coerceIn(1,8) ?: 3).apply()
            stopService(Intent(this, KnotService::class.java)); Handler(Looper.getMainLooper()).postDelayed({ startForegroundService(Intent(this, KnotService::class.java)); status.text = "Restarting…"; waitForCore(0) }, 400)
        }.show()
    }

    override fun onDestroy() { if (isFinishing && WebViewFeature.isFeatureSupported(WebViewFeature.PROXY_OVERRIDE)) ProxyController.getInstance().clearProxyOverride(mainExecutor) {}; super.onDestroy() }
}
