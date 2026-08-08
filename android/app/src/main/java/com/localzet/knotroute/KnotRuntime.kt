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
            val instance = unwrap { create.invoke(null, options.toString()) } ?: error("KnotRoute core returned null client")
            call(instance, "start")
            client = instance
            generation++
            lastError = null
        } catch (t: Throwable) {
            lastError = root(t).message ?: root(t).javaClass.simpleName
            throw RuntimeException(lastError, root(t))
        }
    }

    @Synchronized fun stop() { client?.let { runCatching { call(it, "stop") } }; client = null; generation++ }
    fun ready() = client != null
    fun generation() = generation
    fun nodeAddress(): String = callString("nodeaddress")
    fun userId(): String = callString("userid")
    fun proxyUrl(): String = callString("httpproxyurl").ifBlank { "http://127.0.0.1:19478" }
    fun rootCaPem(): String = callString("rootcapem").ifBlank { error("KnotRoute core is not running") }
    fun caProfileJson(): String = callString("caprofilejson")
    fun setCaProfile(commonName:String,organization:String,organizationalUnit:String,country:String,province:String,locality:String,street:String,postal:String,validityDays:Int){call(clientOrThrow(),"setcaprofile",commonName,organization,organizationalUnit,country,province,locality,street,postal,validityDays)}
    fun rotateCa(): String = call(clientOrThrow(),"rotateca") as? String ?: ""
    fun statusJson(): String = callString("statusjson")
    fun socialStateJson(): String = callString("socialstatejson")
    fun userProfileJson(): String = callString("userprofilejson")
    fun setUserProfile(name: String, bio: String) { call(clientOrThrow(), "setuserprofile", name, bio) }
    fun addContact(node: String, alias: String): String = call(clientOrThrow(), "addcontact", node, alias) as? String ?: ""
    fun sendMessage(userId: String, body: String): String = call(clientOrThrow(), "sendmessage", userId, body) as? String ?: ""
    fun createPost(text: String, tags: String): String = call(clientOrThrow(), "createpost", text, tags) as? String ?: ""
    fun fetchContactFeed(userId: String): String = call(clientOrThrow(), "fetchcontactfeed", userId) as? String ?: ""

    private fun callString(name: String): String = client?.let { call(it, name) as? String } ?: ""
    private fun clientOrThrow(): Any = client ?: error("KnotRoute core is not running")
    private fun call(target: Any, name: String, vararg args: Any?): Any? {
        val method = target.javaClass.methods.firstOrNull { normalize(it.name) == normalize(name) && it.parameterCount == args.size }
            ?: error("KnotRoute binding method $name not found")
        return unwrap { method.invoke(target, *args) }
    }
    private fun normalize(value: String) = value.lowercase().replace("_", "")
    private fun unwrap(block: () -> Any?): Any? = try { block() } catch (e: InvocationTargetException) { throw e.targetException }
    private fun root(t: Throwable): Throwable = if (t is InvocationTargetException && t.targetException != null) t.targetException else t
}
