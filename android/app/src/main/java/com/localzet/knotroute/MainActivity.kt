package com.localzet.knotroute

import android.Manifest
import android.annotation.SuppressLint
import android.app.*
import android.content.*
import android.content.pm.PackageManager
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.net.Uri
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
    private lateinit var root: LinearLayout
    private lateinit var content: FrameLayout
    private lateinit var statusText: TextView
    private lateinit var statusDot: TextView
    private lateinit var web: WebView
    private lateinit var address: EditText
    private val mainExecutor = Executor { runOnUiThread(it) }
    private var currentPage = "home"
    private var waitGeneration = 0

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(arrayOf(Manifest.permission.POST_NOTIFICATIONS), 40)
        }
        handleJoinIntent(intent)
        startForegroundService(Intent(this, KnotService::class.java))
        buildUi()
        waitGeneration++
        waitForCore(0, waitGeneration, null)
    }

    override fun onNewIntent(intent: Intent?) {
        super.onNewIntent(intent)
        setIntent(intent)
        if (intent != null && handleJoinIntent(intent)) {
            restartCore()
            showPage("network")
        }
    }

    private fun handleJoinIntent(intent: Intent): Boolean {
        val uri = intent.data ?: return false
        val profile = NetworkProfiles.importJoinUri(this, uri) ?: return false
        val profiles = NetworkProfiles.all(this)
        val existing = profiles.indexOfFirst { it.networkId == profile.networkId }
        val index = if (existing >= 0) {
            profiles[existing] = profile
            existing
        } else {
            profiles.add(profile)
            profiles.lastIndex
        }
        NetworkProfiles.save(this, profiles, index)
        Toast.makeText(this, getString(R.string.profile_imported), Toast.LENGTH_LONG).show()
        return true
    }

    private fun buildUi() {
        root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setBackgroundColor(Color.rgb(12, 14, 18))
        }
        val header = buildHeader()
        val bottomNav = buildBottomNav()
        root.addView(header)
        content = FrameLayout(this)
        root.addView(content, LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f))
        root.addView(bottomNav)
        setContentView(root)
        applySystemInsets(header, bottomNav)
        showPage("home")
    }

    @Suppress("DEPRECATION")
    private fun applySystemInsets(header: View, bottomNav: View) {
        if (Build.VERSION.SDK_INT >= 30) window.setDecorFitsSystemWindows(false)
        val headerLeft = header.paddingLeft
        val headerTop = header.paddingTop
        val headerRight = header.paddingRight
        val headerBottom = header.paddingBottom
        val navLeft = bottomNav.paddingLeft
        val navTop = bottomNav.paddingTop
        val navRight = bottomNav.paddingRight
        val navBottom = bottomNav.paddingBottom
        root.setOnApplyWindowInsetsListener { _, insets ->
            val left: Int
            val top: Int
            val right: Int
            val bottom: Int
            if (Build.VERSION.SDK_INT >= 30) {
                val bars = insets.getInsets(WindowInsets.Type.systemBars() or WindowInsets.Type.displayCutout())
                left = bars.left; top = bars.top; right = bars.right; bottom = bars.bottom
            } else {
                left = insets.systemWindowInsetLeft; top = insets.systemWindowInsetTop
                right = insets.systemWindowInsetRight; bottom = insets.systemWindowInsetBottom
            }
            header.setPadding(headerLeft + left, headerTop + top, headerRight + right, headerBottom)
            bottomNav.setPadding(navLeft + left, navTop, navRight + right, navBottom + bottom)
            insets
        }
        root.requestApplyInsets()
    }

    private fun buildHeader(): View {
        val header = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            setPadding(dp(18), dp(14), dp(18), dp(12))
        }
        val titleBox = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        titleBox.addView(TextView(this).apply {
            text = "KnotRoute"
            setTextColor(Color.WHITE)
            textSize = 22f
            setTypeface(typeface, Typeface.BOLD)
        })
        titleBox.addView(TextView(this).apply {
            text = getString(R.string.client_subtitle)
            setTextColor(Color.rgb(128, 143, 160))
            textSize = 9f
            letterSpacing = .18f
        })
        header.addView(titleBox, LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f))
        statusDot = TextView(this).apply { text = "●"; textSize = 18f; setTextColor(Color.rgb(255, 201, 112)) }
        statusText = TextView(this).apply { text = getString(R.string.starting); setTextColor(Color.LTGRAY); setPadding(dp(7), 0, 0, 0) }
        header.addView(statusDot)
        header.addView(statusText)
        return header
    }

    private fun buildBottomNav(): View {
        val nav = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            setPadding(dp(8), dp(6), dp(8), dp(10))
            background = solid(Color.rgb(17, 20, 26), 0f)
        }
        listOf(
            "home" to getString(R.string.home),
            "browser" to getString(R.string.browser),
            "network" to getString(R.string.network),
            "diagnostics" to getString(R.string.diagnostics),
        ).forEach { (page, label) ->
            nav.addView(Button(this).apply {
                text = label
                isAllCaps = false
                setTextColor(Color.WHITE)
                setBackgroundColor(Color.TRANSPARENT)
                setOnClickListener { showPage(page) }
            }, LinearLayout.LayoutParams(0, dp(48), 1f))
        }
        return nav
    }

    private fun showPage(page: String) {
        currentPage = page
        content.removeAllViews()
        val view = when (page) {
            "browser" -> buildBrowser()
            "network" -> buildNetworkPage()
            "diagnostics" -> buildDiagnosticsPage()
            else -> buildHomePage()
        }
        content.addView(view, FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT))
    }

    private fun buildHomePage(): View = ScrollView(this).apply {
        addView(LinearLayout(this@MainActivity).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(18), dp(8), dp(18), dp(28))
            addView(card(getString(R.string.current_network), NetworkProfiles.active(this@MainActivity).name, NetworkProfiles.active(this@MainActivity).networkId.ifBlank { "—" }))
            addView(card(getString(R.string.node_address), KnotRuntime.nodeAddress().ifBlank { "—" }, getString(R.string.proxy) + ": " + KnotRuntime.proxyUrl()))
            addView(actionButton(getString(R.string.open_site)) { showPage("browser") })
            addView(actionButton(getString(R.string.install_ca)) { installCa() })
            addView(actionButton(getString(R.string.network_settings)) { showPage("network") })
        })
    }

    @SuppressLint("SetJavaScriptEnabled")
    private fun buildBrowser(): View {
        val box = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL; setPadding(dp(10), dp(4), dp(10), dp(8)) }
        val bar = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL; gravity = Gravity.CENTER_VERTICAL }
        address = EditText(this).apply {
            hint = getString(R.string.address_hint)
            setSingleLine()
            setTextColor(Color.WHITE)
            setHintTextColor(Color.rgb(110, 120, 134))
            background = solid(Color.rgb(24, 28, 36), 12f)
            setPadding(dp(14), dp(10), dp(14), dp(10))
        }
        bar.addView(address, LinearLayout.LayoutParams(0, dp(50), 1f))
        bar.addView(Button(this).apply { text = getString(R.string.go); isAllCaps = false; setOnClickListener { navigate() } }, LinearLayout.LayoutParams(dp(88), dp(50)))
        box.addView(bar)
        web = WebView(this).apply {
            setBackgroundColor(Color.rgb(12,14,18))
            settings.javaScriptEnabled = true
            settings.domStorageEnabled = true
            settings.loadsImagesAutomatically = true
            settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
            webViewClient = object : WebViewClient() {
                override fun onReceivedSslError(view: WebView?, handler: SslErrorHandler?, error: SslError?) {
                    handler?.cancel()
                    Toast.makeText(this@MainActivity, getString(R.string.tls_error), Toast.LENGTH_LONG).show()
                }
            }
        }
        box.addView(web, LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f))
        configureProxy()
        return box
    }

    private fun buildNetworkPage(): View = ScrollView(this).apply {
        val box = LinearLayout(this@MainActivity).apply { orientation = LinearLayout.VERTICAL; setPadding(dp(18), dp(8), dp(18), dp(28)) }
        val profiles = NetworkProfiles.all(this@MainActivity)
        var selected = NetworkProfiles.activeIndex(this@MainActivity)
        val spinner = Spinner(this@MainActivity)
        spinner.adapter = ArrayAdapter(this@MainActivity, android.R.layout.simple_spinner_dropdown_item, profiles.map { it.name })
        spinner.setSelection(selected)
        val name = field(getString(R.string.profile_name))
        val network = field(getString(R.string.network_id))
        val beacons = field(getString(R.string.beacon_urls), multiline = true)
        val hops = field(getString(R.string.circuit_hops)).apply { inputType = android.text.InputType.TYPE_CLASS_NUMBER }
        fun load(i: Int) { val p = profiles[i]; name.setText(p.name); network.setText(p.networkId); beacons.setText(p.beacons.joinToString("\n")); hops.setText(p.circuitHops.toString()) }
        load(selected)
        spinner.onItemSelectedListener = object : android.widget.AdapterView.OnItemSelectedListener {
            override fun onItemSelected(parent: android.widget.AdapterView<*>?, view: View?, position: Int, id: Long) { selected = position; load(position) }
            override fun onNothingSelected(parent: android.widget.AdapterView<*>?) {}
        }
        box.addView(label(getString(R.string.profile))); box.addView(spinner)
        box.addView(label(getString(R.string.profile_name))); box.addView(name)
        box.addView(label(getString(R.string.network_id))); box.addView(network)
        box.addView(label(getString(R.string.beacon_urls))); box.addView(beacons)
        val advanced = LinearLayout(this@MainActivity).apply {
            orientation = LinearLayout.VERTICAL
            visibility = View.GONE
            addView(label(getString(R.string.circuit_hops)))
            addView(hops)
        }
        box.addView(actionButton(getString(R.string.advanced_settings), secondary = true) {
            advanced.visibility = if (advanced.visibility == View.VISIBLE) View.GONE else View.VISIBLE
        })
        box.addView(advanced)
        box.addView(TextView(this@MainActivity).apply { text=getString(R.string.about_network); setTextColor(Color.rgb(142,151,165)); setPadding(0,dp(14),0,dp(8)) })
        box.addView(actionButton(getString(R.string.save_restart)) {
            val networkId = network.text.toString().trim()
            if (networkId.isNotEmpty() && !networkId.startsWith("kn_")) {
                Toast.makeText(this@MainActivity, getString(R.string.invalid_network_id), Toast.LENGTH_LONG).show()
                return@actionButton
            }
            val beaconList = beacons.text.toString().lines().map { it.trim() }.filter { it.isNotEmpty() }
            val invalidBeacon = beaconList.firstOrNull { validateBeaconUrl(it) != null }
            if (invalidBeacon != null) {
                Toast.makeText(this@MainActivity, validateBeaconUrl(invalidBeacon), Toast.LENGTH_LONG).show()
                return@actionButton
            }
            val updated = NetworkProfile(name.text.toString().trim().ifBlank { getString(R.string.default_profile) }, networkId, beaconList, hops.text.toString().toIntOrNull()?.coerceIn(1,8) ?: 3)
            profiles[selected] = updated
            NetworkProfiles.save(this@MainActivity, profiles, selected)
            restartCore()
        })
        box.addView(actionButton(getString(R.string.add_profile)) {
            profiles.add(NetworkProfile(getString(R.string.default_profile) + " ${profiles.size + 1}", "", emptyList()))
            NetworkProfiles.save(this@MainActivity, profiles, profiles.lastIndex)
            showPage("network")
        })
        if (profiles.size > 1) box.addView(actionButton(getString(R.string.delete_profile), danger = true) {
            profiles.removeAt(selected)
            NetworkProfiles.save(this@MainActivity, profiles, 0)
            restartCore(); showPage("network")
        })
        addView(box)
    }

    private fun buildDiagnosticsPage(): View = ScrollView(this).apply {
        addView(LinearLayout(this@MainActivity).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(18), dp(8), dp(18), dp(28))
            addView(card(getString(R.string.node_address), KnotRuntime.nodeAddress().ifBlank { "—" }, ""))
            addView(card(getString(R.string.proxy), KnotRuntime.proxyUrl(), ""))
            addView(card(getString(R.string.last_error), KnotRuntime.lastError ?: getString(R.string.no_error), ""))
            addView(actionButton(getString(R.string.copy)) {
                val text = "node=${KnotRuntime.nodeAddress()}\nproxy=${KnotRuntime.proxyUrl()}\nerror=${KnotRuntime.lastError ?: "none"}"
                (getSystemService(CLIPBOARD_SERVICE) as ClipboardManager).setPrimaryClip(ClipData.newPlainText("KnotRoute diagnostics", text))
                Toast.makeText(this@MainActivity, getString(R.string.copied), Toast.LENGTH_SHORT).show()
            })
        })
    }

    private fun waitForCore(attempt: Int, generation: Int, afterRuntimeGeneration: Long?) {
        if (generation != waitGeneration || isFinishing) return
        val runtimeIsFresh = afterRuntimeGeneration == null || KnotRuntime.generation() > afterRuntimeGeneration
        if (KnotRuntime.ready() && runtimeIsFresh) {
            statusDot.setTextColor(Color.rgb(140,255,193)); statusText.text = getString(R.string.connected)
            configureProxy()
            if (currentPage == "home" || currentPage == "diagnostics") showPage(currentPage)
            return
        }
        if (attempt > 200) {
            statusDot.setTextColor(Color.rgb(255,112,126)); statusText.text = getString(R.string.failed)
            KnotRuntime.lastError?.let { Toast.makeText(this, it, Toast.LENGTH_LONG).show() }
            return
        }
        Handler(Looper.getMainLooper()).postDelayed({ waitForCore(attempt + 1, generation, afterRuntimeGeneration) }, 200)
    }

    private fun restartCore() {
        statusDot.setTextColor(Color.rgb(255,201,112)); statusText.text = getString(R.string.starting)
        val previousRuntimeGeneration = KnotRuntime.generation()
        waitGeneration++
        startForegroundService(Intent(this, KnotService::class.java).setAction(KnotService.ACTION_RESTART))
        waitForCore(0, waitGeneration, previousRuntimeGeneration)
    }

    private fun configureProxy() {
        if (!KnotRuntime.ready()) return
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.PROXY_OVERRIDE)) {
            Toast.makeText(this, getString(R.string.webview_proxy_unsupported), Toast.LENGTH_LONG).show(); return
        }
        val endpoint = KnotRuntime.proxyUrl().removePrefix("http://")
        ProxyController.getInstance().setProxyOverride(ProxyConfig.Builder().addProxyRule(endpoint).build(), mainExecutor) {}
    }

    private fun validateBeaconUrl(raw: String): String? {
        return try {
            val uri = Uri.parse(raw)
            if ((uri.scheme != "https" && uri.scheme != "http") || uri.host.isNullOrBlank()) getString(R.string.invalid_beacon_url)
            else if (uri.port == 7447) getString(R.string.beacon_relay_port_error)
            else if (!uri.path.isNullOrBlank() && uri.path != "/") getString(R.string.beacon_root_error)
            else null
        } catch (_: Throwable) { getString(R.string.invalid_beacon_url) }
    }

    private fun navigate() {
        var value = address.text.toString().trim(); if (value.isEmpty()) return
        if (!value.contains("://")) value = "https://$value"
        web.loadUrl(value)
    }

    private fun installCa() {
        if (!KnotRuntime.ready()) { Toast.makeText(this, getString(R.string.core_starting), Toast.LENGTH_SHORT).show(); return }
        try {
            val cert = CertificateFactory.getInstance("X.509").generateCertificate(ByteArrayInputStream(KnotRuntime.rootCaPem().toByteArray()))
            startActivity(KeyChain.createInstallIntent().apply { putExtra(KeyChain.EXTRA_NAME, "KnotRoute Local Root CA"); putExtra(KeyChain.EXTRA_CERTIFICATE, cert.encoded) })
        } catch (e: Exception) { Toast.makeText(this, e.message ?: getString(R.string.ca_failed), Toast.LENGTH_LONG).show() }
    }

    private fun actionButton(text: String, danger: Boolean = false, secondary: Boolean = false, action: () -> Unit): Button = Button(this).apply {
        this.text = text; isAllCaps = false; textSize = 15f
        setTextColor(if (danger) Color.rgb(255,125,137) else if (secondary) Color.WHITE else Color.rgb(8,17,13))
        background = solid(if (danger) Color.rgb(51,28,34) else if (secondary) Color.rgb(31,36,45) else Color.rgb(140,255,193), 12f)
        setOnClickListener { action() }
        layoutParams = LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(52)).apply { topMargin = dp(10) }
    }
    private fun card(title: String, value: String, detail: String): View = LinearLayout(this).apply {
        orientation = LinearLayout.VERTICAL; setPadding(dp(16),dp(14),dp(16),dp(14)); background=solid(Color.rgb(22,26,33),14f)
        addView(TextView(this@MainActivity).apply { text=title.uppercase(); setTextColor(Color.rgb(126,139,154)); textSize=10f; letterSpacing=.12f })
        addView(TextView(this@MainActivity).apply { text=value; setTextColor(Color.WHITE); textSize=18f; setTypeface(typeface,Typeface.BOLD); setPadding(0,dp(5),0,0) })
        if (detail.isNotBlank()) addView(TextView(this@MainActivity).apply { text=detail; setTextColor(Color.rgb(150,160,174)); setPadding(0,dp(5),0,0) })
        layoutParams=LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT,ViewGroup.LayoutParams.WRAP_CONTENT).apply{bottomMargin=dp(10)}
    }
    private fun label(labelText:String)=TextView(this).apply{this.text=labelText;setTextColor(Color.rgb(150,160,174));setPadding(0,dp(14),0,dp(6));textSize=12f}
    private fun field(hint:String,multiline:Boolean=false)=EditText(this).apply{this.hint=hint;setTextColor(Color.WHITE);setHintTextColor(Color.rgb(103,113,126));background=solid(Color.rgb(22,26,33),10f);setPadding(dp(12),dp(10),dp(12),dp(10));if(!multiline)setSingleLine()else{minLines=3;gravity=Gravity.TOP}}
    private fun solid(color:Int,radius:Float)=GradientDrawable().apply{setColor(color);cornerRadius=dp(radius.toInt()).toFloat()}
    private fun dp(value:Int)=(value*resources.displayMetrics.density).toInt()

    override fun onDestroy() {
        if (isFinishing && WebViewFeature.isFeatureSupported(WebViewFeature.PROXY_OVERRIDE)) ProxyController.getInstance().clearProxyOverride(mainExecutor) {}
        super.onDestroy()
    }
}
