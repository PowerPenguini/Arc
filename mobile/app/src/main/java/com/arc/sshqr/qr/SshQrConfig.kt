package com.arc.sshqr.qr

data class SshQrConfig(
    val host: String,
    val port: Int = 22,
    val username: String,
    val privateKeyPem: String,
    val passphrase: String?,
    val wireGuardConfig: String?,
    val wireGuardTunnelName: String?,
)
