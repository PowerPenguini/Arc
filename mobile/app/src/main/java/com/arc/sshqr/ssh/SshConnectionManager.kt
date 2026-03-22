package com.arc.sshqr.ssh

import android.util.Log
import com.arc.sshqr.qr.SshQrConfig
import java.io.Closeable
import java.security.KeyFactory
import java.security.KeyPairGenerator
import java.security.Signature
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import net.schmizz.sshj.DefaultConfig
import net.schmizz.sshj.SSHClient
import net.schmizz.sshj.common.SecurityUtils
import net.schmizz.sshj.transport.DisconnectListener
import net.schmizz.sshj.transport.TransportException
import net.schmizz.sshj.transport.kex.Curve25519SHA256
import net.schmizz.sshj.transport.verification.PromiscuousVerifier
import net.schmizz.sshj.userauth.keyprovider.KeyProvider
import net.schmizz.sshj.userauth.keyprovider.OpenSSHKeyFile
import net.schmizz.sshj.userauth.password.PasswordUtils

class SshConnectionManager(
    private val prepareTransport: suspend (SshQrConfig, Boolean) -> Unit = { _, _ -> },
) : Closeable {

    private val mutex = Mutex()
    private val cryptoCapabilities = detectCryptoCapabilities()

    private var client: SSHClient? = null
    private var activeConfig: SshQrConfig? = null
    @Volatile
    private var connectionInvalidated = false
    @Volatile
    private var transportDisconnectMessage: String? = null

    suspend fun ensureConnected(
        config: SshQrConfig,
        forceReconnect: Boolean = false,
        refreshTransport: Boolean = false,
    ): SSHClient = withContext(Dispatchers.IO) {
        mutex.withLock {
            val reusable = client?.takeIf {
                !forceReconnect &&
                    !refreshTransport &&
                    !connectionInvalidated &&
                    activeConfig == config &&
                    it.isConnected &&
                    it.isAuthenticated
            }
            if (reusable != null) {
                return@withLock reusable
            }

            closeLocked()
            prepareTransport(config, refreshTransport)
            val ssh = connect(config)
            client = ssh
            activeConfig = config
            connectionInvalidated = false
            transportDisconnectMessage = null
            ssh
        }
    }

    suspend fun reconnect(config: SshQrConfig): SSHClient =
        ensureConnected(config, forceReconnect = true, refreshTransport = true)

    suspend fun disconnectSession(config: SshQrConfig? = null) {
        withContext(Dispatchers.IO) {
            mutex.withLock {
                if (config != null && activeConfig != config) {
                    return@withLock
                }
                closeLocked()
            }
        }
    }

    fun latestDisconnectMessage(): String? = transportDisconnectMessage

    override fun close() {
        runCatching {
            kotlinx.coroutines.runBlocking {
                disconnectSession()
            }
        }
    }

    private fun connect(config: SshQrConfig): SSHClient {
        val failures = mutableListOf<String>()
        val profiles = listOf(
            CryptoProfile("default"),
            CryptoProfile("no-curve25519", disableCurve25519 = true),
            CryptoProfile("no-ed25519-hostkey", disableEd25519HostKeys = true),
            CryptoProfile(
                name = "compat",
                disableCurve25519 = true,
                disableEd25519HostKeys = true,
                preferSshRsa = true,
            ),
        )

        for (profile in profiles) {
            val ssh = createSshClient(profile)
            try {
                ssh.addHostKeyVerifier(PromiscuousVerifier())
                ssh.connect(config.host, config.port)
                val keyProvider = createKeyProvider(config)
                ensureKeyProviderIsSupported(keyProvider)
                ssh.authPublickey(config.username, keyProvider)
                ssh.connection.keepAlive.keepAliveInterval = KEEPALIVE_INTERVAL_SECONDS
                if (!ssh.connection.keepAlive.isAlive) {
                    ssh.connection.keepAlive.start()
                }
                return ssh
            } catch (throwable: Throwable) {
                runCatching { ssh.disconnect() }
                runCatching { ssh.close() }
                failures += "${profile.name}: ${throwable.message ?: throwable::class.java.simpleName}"
                if (!isAlgorithmAvailabilityFailure(throwable)) {
                    throw throwable
                }
            }
        }

        throw TransportException("No compatible SSH crypto profile. ${failures.joinToString(" | ")}")
    }

    private fun createKeyProvider(config: SshQrConfig): KeyProvider {
        val passphraseFinder = config.passphrase?.takeIf { it.isNotBlank() }?.let {
            PasswordUtils.createOneOff(it.toCharArray())
        }
        return OpenSSHKeyFile().apply {
            init(config.privateKeyPem, null, passphraseFinder)
        }
    }

    private fun createSshClient(profile: CryptoProfile): SSHClient {
        SecurityUtils.setRegisterBouncyCastle(false)
        SecurityUtils.setSecurityProvider(null)
        val disableCurve25519 = profile.disableCurve25519 || !cryptoCapabilities.supportsX25519
        val disableEd25519HostKeys = profile.disableEd25519HostKeys || !cryptoCapabilities.supportsEd25519
        val disableEcdsaHostKeys = profile.disableEcdsaHostKeys || !cryptoCapabilities.supportsEcdsa
        val config = DefaultConfig().apply {
            if (disableCurve25519) {
                keyExchangeFactories = keyExchangeFactories.filterNot { factory ->
                    factory is Curve25519SHA256.Factory || factory is Curve25519SHA256.FactoryLibSsh
                }
            }
            if (disableEd25519HostKeys || disableEcdsaHostKeys) {
                keyAlgorithms = keyAlgorithms.filterNot { factory ->
                    (disableEd25519HostKeys && (
                        factory.name == "ssh-ed25519" ||
                            factory.name == "ssh-ed25519-cert-v01@openssh.com"
                        )) ||
                        (disableEcdsaHostKeys && factory.name.startsWith("ecdsa-sha2-"))
                }
            }
            if (profile.preferSshRsa) {
                prioritizeSshRsaKeyAlgorithm()
            }
        }
        return SSHClient(config).apply {
            setConnectTimeout(SOCKET_CONNECT_TIMEOUT_MS)
            setTimeout(0)
            transport.timeoutMs = 0
            transport.setDisconnectListener(
                DisconnectListener { reason, message ->
                    transportDisconnectMessage = "SSH transport disconnected: reason=$reason message=${message.orEmpty()}"
                    connectionInvalidated = true
                    Log.w(TAG, transportDisconnectMessage.orEmpty())
                },
            )
        }
    }

    private fun closeLocked() {
        connectionInvalidated = true
        runCatching {
            client?.disconnect()
            client?.close()
        }
        client = null
        activeConfig = null
    }

    private fun isAlgorithmAvailabilityFailure(throwable: Throwable): Boolean {
        var current: Throwable? = throwable
        while (current != null) {
            val message = current.message.orEmpty()
            if (
                message.contains("no such algorithm", ignoreCase = true) ||
                message.contains("KeyFactory not available", ignoreCase = true) ||
                message.contains("KeyPairGenerator not available", ignoreCase = true)
            ) {
                return true
            }
            current = current.cause
        }
        return false
    }

    private fun detectCryptoCapabilities(): CryptoCapabilities {
        SecurityUtils.setRegisterBouncyCastle(false)
        SecurityUtils.setSecurityProvider(null)
        return CryptoCapabilities(
            supportsX25519 = supportsKeyPairGenerator("X25519"),
            supportsEd25519 = supportsKeyFactory("Ed25519"),
            supportsEcdsa = supportsKeyFactory("ECDSA"),
            supportsEd25519Signature = supportsSignature("Ed25519"),
            supportsEcdsaSignature = supportsSignature("SHA256withECDSA"),
        )
    }

    private fun supportsKeyPairGenerator(algorithm: String): Boolean =
        runCatching { KeyPairGenerator.getInstance(algorithm) }.isSuccess

    private fun supportsKeyFactory(algorithm: String): Boolean =
        runCatching { KeyFactory.getInstance(algorithm) }.isSuccess

    private fun supportsSignature(algorithm: String): Boolean =
        runCatching { Signature.getInstance(algorithm) }.isSuccess

    private fun ensureKeyProviderIsSupported(keyProvider: KeyProvider) {
        val keyType = runCatching { keyProvider.type.toString() }.getOrDefault("unknown")
        if (keyType.contains("ssh-ed25519", ignoreCase = true) && !cryptoCapabilities.supportsEd25519Signature) {
            throw IllegalStateException(
                "This Android device cannot use Ed25519 SSH user keys. Generate an RSA key for mobile or use a device/provider with Ed25519 signatures.",
            )
        }
        if (keyType.contains("ecdsa", ignoreCase = true) && !cryptoCapabilities.supportsEcdsaSignature) {
            throw IllegalStateException(
                "This Android device cannot use ECDSA SSH user keys. Generate an RSA key for mobile or use a device/provider with ECDSA signatures.",
            )
        }
    }

    private data class CryptoProfile(
        val name: String,
        val disableCurve25519: Boolean = false,
        val disableEd25519HostKeys: Boolean = false,
        val disableEcdsaHostKeys: Boolean = false,
        val preferSshRsa: Boolean = false,
    )

    private data class CryptoCapabilities(
        val supportsX25519: Boolean,
        val supportsEd25519: Boolean,
        val supportsEcdsa: Boolean,
        val supportsEd25519Signature: Boolean,
        val supportsEcdsaSignature: Boolean,
    )

    companion object {
        private const val TAG = "SshConnectionManager"
        private const val SOCKET_CONNECT_TIMEOUT_MS = 10_000
        private const val KEEPALIVE_INTERVAL_SECONDS = 15
    }
}
