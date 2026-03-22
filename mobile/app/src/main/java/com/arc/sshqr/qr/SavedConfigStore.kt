package com.arc.sshqr.qr

import android.content.Context
import android.util.Log
import org.json.JSONObject

interface SavedConfigStore {
    fun load(): SshQrConfig?
    fun save(config: SshQrConfig)
    fun clear()
}

class SharedPreferencesSavedConfigStore(
    context: Context,
) : SavedConfigStore {

    private val preferences = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    override fun load(): SshQrConfig? {
        val raw = preferences.getString(KEY_LAST_CONFIG, null) ?: return null
        return runCatching {
            SavedConfigSerializer.deserialize(raw)
        }.onFailure { error ->
            Log.e(TAG, "Failed to load saved config", error)
            clear()
        }.getOrNull()
    }

    override fun save(config: SshQrConfig) {
        preferences.edit()
            .putString(KEY_LAST_CONFIG, SavedConfigSerializer.serialize(config))
            .apply()
    }

    override fun clear() {
        preferences.edit()
            .remove(KEY_LAST_CONFIG)
            .apply()
    }

    private companion object {
        private const val TAG = "SavedConfigStore"
        private const val PREFS_NAME = "arc_saved_config"
        private const val KEY_LAST_CONFIG = "last_ssh_qr_config"
    }
}

object SavedConfigSerializer {

    fun serialize(config: SshQrConfig): String = JSONObject()
        .put("host", config.host)
        .put("port", config.port)
        .put("username", config.username)
        .put("privateKeyPem", config.privateKeyPem)
        .put("passphrase", config.passphrase)
        .put("wireguardConfig", config.wireGuardConfig)
        .put("wireguardTunnelName", config.wireGuardTunnelName)
        .toString()

    fun deserialize(raw: String): SshQrConfig =
        SshQrParser.parse(raw).getOrElse { throw IllegalArgumentException("Saved config is invalid.", it) }
}
