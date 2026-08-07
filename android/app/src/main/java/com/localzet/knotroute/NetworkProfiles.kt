package com.localzet.knotroute

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject

data class NetworkProfile(
    val name: String,
    val networkId: String,
    val beacons: List<String>,
    val circuitHops: Int = 3,
)

object NetworkProfiles {
    private const val PREFS = "knotroute"
    private const val KEY_PROFILES = "profiles_v1"
    private const val KEY_ACTIVE = "active_profile"

    fun all(context: Context): MutableList<NetworkProfile> {
        val prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val raw = prefs.getString(KEY_PROFILES, null)
        if (raw.isNullOrBlank()) {
            val legacy = NetworkProfile(
                name = context.getString(R.string.default_profile),
                networkId = prefs.getString("network_id", "") ?: "",
                beacons = (prefs.getString("beacons", "") ?: "").split(',').map { it.trim() }.filter { it.isNotEmpty() },
                circuitHops = prefs.getInt("circuit_hops", 3),
            )
            return mutableListOf(legacy)
        }
        return runCatching {
            val arr = JSONArray(raw)
            MutableList(arr.length()) { i ->
                val item = arr.getJSONObject(i)
                val beacons = item.optJSONArray("beacons") ?: JSONArray()
                NetworkProfile(
                    name = item.optString("name", context.getString(R.string.default_profile)),
                    networkId = item.optString("network_id", ""),
                    beacons = List(beacons.length()) { j -> beacons.optString(j) }.filter { it.isNotBlank() },
                    circuitHops = item.optInt("circuit_hops", 3).coerceIn(1, 8),
                )
            }
        }.getOrElse { mutableListOf(NetworkProfile(context.getString(R.string.default_profile), "", emptyList())) }
    }

    fun activeIndex(context: Context): Int {
        val profiles = all(context)
        return context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getInt(KEY_ACTIVE, 0).coerceIn(0, (profiles.size - 1).coerceAtLeast(0))
    }

    fun active(context: Context): NetworkProfile = all(context)[activeIndex(context)]

    fun save(context: Context, profiles: List<NetworkProfile>, activeIndex: Int) {
        require(profiles.isNotEmpty())
        val arr = JSONArray()
        profiles.forEach { profile ->
            arr.put(JSONObject().apply {
                put("name", profile.name)
                put("network_id", profile.networkId)
                put("beacons", JSONArray(profile.beacons))
                put("circuit_hops", profile.circuitHops)
            })
        }
        val active = profiles[activeIndex.coerceIn(0, profiles.lastIndex)]
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit()
            .putString(KEY_PROFILES, arr.toString())
            .putInt(KEY_ACTIVE, activeIndex.coerceIn(0, profiles.lastIndex))
            .putString("network_id", active.networkId)
            .putString("beacons", active.beacons.joinToString(","))
            .putInt("circuit_hops", active.circuitHops)
            .apply()
    }

    fun importJoinUri(context: Context, uri: android.net.Uri): NetworkProfile? {
        if (uri.scheme != "knotroute" || uri.host != "join") return null
        val networkId = uri.getQueryParameter("network_id")?.trim().orEmpty()
        if (!networkId.startsWith("kn_")) return null
        val name = uri.getQueryParameter("name")?.trim().takeUnless { it.isNullOrBlank() } ?: context.getString(R.string.imported_profile)
        val beacons = uri.getQueryParameters("beacon").map { it.trim() }.filter {
            runCatching {
                val parsed = android.net.Uri.parse(it)
                (parsed.scheme == "https" || parsed.scheme == "http") &&
                    !parsed.host.isNullOrBlank() && parsed.port != 7447 &&
                    (parsed.path.isNullOrBlank() || parsed.path == "/") &&
                    parsed.query.isNullOrBlank() && parsed.fragment.isNullOrBlank() && parsed.userInfo.isNullOrBlank()
            }.getOrDefault(false)
        }
        return NetworkProfile(name, networkId, beacons, 3)
    }
}
