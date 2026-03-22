package com.arc.sshqr.qr

import org.json.JSONException
import org.json.JSONObject

object SshQrParser {
    fun parse(raw: String): Result<SshQrConfig> = runCatching {
        val json = JSONObject(raw)
        val host = json.optString("host").trim()
        require(host.isNotEmpty()) { "Missing host." }

        val username = json.optString("username").trim()
        require(username.isNotEmpty()) { "Missing username." }

        val privateKeyPem = json.optString("privateKeyPem")
            .replace("\\n", "\n")
            .trim()
        require(privateKeyPem.startsWith("-----BEGIN")) { "Invalid private key PEM." }

        val port = json.optInt("port", 22)
        require(port in 1..65535) { "Invalid port." }

        val passphrase = json.optString("passphrase")
            .trim()
            .ifEmpty { null }

        val wireGuardConfig = json.optString("wireguardConfig")
            .replace("\\n", "\n")
            .trim()
            .ifEmpty { null }

        val wireGuardTunnelName = json.optString("wireguardTunnelName")
            .trim()
            .ifEmpty { null }

        SshQrConfig(
            host = host,
            port = port,
            username = username,
            privateKeyPem = privateKeyPem,
            passphrase = passphrase,
            wireGuardConfig = wireGuardConfig,
            wireGuardTunnelName = wireGuardTunnelName,
        )
    }.recoverCatching { throwable ->
        when (throwable) {
            is JSONException -> throw IllegalArgumentException("QR payload is not valid JSON.", throwable)
            else -> throw throwable
        }
    }
}
