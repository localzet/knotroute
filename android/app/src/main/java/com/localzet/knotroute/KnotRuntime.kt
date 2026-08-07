package com.localzet.knotroute

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import java.lang.reflect.InvocationTargetException

object KnotRuntime {
    @Volatile private var client: Any? = null
    @Volatile private var generation: Long = 0
    @Volatile var lastError: String? = null
        private set

    @Synchronized
    fun start(context: Context) {
        if (client != null) return
        try {
            val prefs = context.getSharedPreferences("knotroute", Context.MODE_PRIVATE)
            val beacons = JSONArray()
            prefs.getString("beacons", "")!!.split(',').map { it.trim() }.filter { it.isNotEmpty() }.forEach { beacons.put(it) }
            val options = JSONObject()
                .put("data_dir", context.filesDir.resolve("core").absolutePath)
                .put("network_id", prefs.getString("network_id", "") ?: "")
                .put("beacons", beacons)
                .put("circuit_hops", prefs.getInt("circuit_hops", 3))
                .put("http_proxy_port", 19478)
            val api = Class.forName("knotmobile.Knotmobile")
            val create = api.methods.firstOrNull { normalize(it.name) == "createclient" }
                ?: error("knotmobile CreateClient binding not found")
            val instance = unwrap { create.invoke(null, options.toString()) }
                ?: error("KnotRoute core returned null client")
            invoke(instance, "start")
            client = instance
            generation++
            lastError = null
        } catch (t: Throwable) {
            lastError = root(t).message ?: root(t).javaClass.simpleName
            throw RuntimeException(lastError, root(t))
        }
    }

    @Synchronized
    fun stop() {
        client?.let { runCatching { invoke(it, "stop") } }
        client = null
        generation++
    }

    fun ready(): Boolean = client != null
    fun generation(): Long = generation
    fun nodeAddress(): String = client?.let { invoke(it, "nodeaddress") as? String } ?: ""
    fun proxyUrl(): String = client?.let { invoke(it, "httpproxyurl") as? String } ?: "http://127.0.0.1:19478"
    fun rootCaPem(): String = client?.let { invoke(it, "rootcapem") as? String } ?: error("KnotRoute core is not running")

    private fun invoke(target: Any, name: String): Any? {
        val method = target.javaClass.methods.firstOrNull { normalize(it.name) == normalize(name) && it.parameterCount == 0 }
            ?: error("KnotRoute binding method $name not found")
        return unwrap { method.invoke(target) }
    }
    private fun normalize(value: String) = value.lowercase().replace("_", "")
    private fun unwrap(block: () -> Any?): Any? = try { block() } catch (e: InvocationTargetException) { throw e.targetException }
    private fun root(t: Throwable): Throwable = if (t is InvocationTargetException && t.targetException != null) t.targetException else t
}
