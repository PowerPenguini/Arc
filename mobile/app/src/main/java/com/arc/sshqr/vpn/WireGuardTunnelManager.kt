package com.arc.sshqr.vpn

import android.content.Context
import com.arc.sshqr.qr.SshQrConfig
import com.wireguard.android.backend.BackendException
import com.wireguard.android.backend.GoBackend
import com.wireguard.android.backend.Statistics
import com.wireguard.android.backend.Tunnel
import com.wireguard.config.BadConfigException
import com.wireguard.config.Config
import java.io.BufferedReader
import java.io.StringReader
import java.net.Inet4Address
import java.net.Inet6Address
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext

internal const val DEFAULT_TUNNEL_NAME = "arc"

data class WireGuardSessionInfo(
    val tunnelName: String,
    val reused: Boolean,
)

class WireGuardTunnelManager(
    appContext: Context,
) {
    private val backend = GoBackend(appContext.applicationContext)
    private val stateMutex = Mutex()

    private var currentTunnel: AppTunnel? = null
    private var currentConfig: Config? = null

    suspend fun ensureTunnelUp(
        config: SshQrConfig,
        forceRestart: Boolean = false,
    ): WireGuardSessionInfo? {
        val rawConfig = config.wireGuardConfig?.takeIf { it.isNotBlank() } ?: return null
        val parsedConfig = parseAndValidateWireGuardConfig(rawConfig)
        val tunnelName = resolveWireGuardTunnelName(config.wireGuardTunnelName)

        return withContext(Dispatchers.IO) {
            stateMutex.withLock {
                val tunnel = currentTunnel?.takeIf { it.tunnelName == tunnelName } ?: AppTunnel(tunnelName)
                val reused = tunnel === currentTunnel &&
                    currentConfig == parsedConfig &&
                    runCatching { backend.getState(tunnel) == Tunnel.State.UP }.getOrDefault(false)

                if (forceRestart) {
                    runCatching { backend.setState(tunnel, Tunnel.State.DOWN, currentConfig ?: parsedConfig) }
                    backend.setState(tunnel, Tunnel.State.UP, parsedConfig)
                    currentTunnel = tunnel
                    currentConfig = parsedConfig
                } else if (!reused) {
                    backend.setState(tunnel, Tunnel.State.UP, parsedConfig)
                    currentTunnel = tunnel
                    currentConfig = parsedConfig
                }

                waitForHandshakeOrThrow(
                    tunnel = tunnel,
                    config = parsedConfig,
                )

                WireGuardSessionInfo(
                    tunnelName = tunnelName,
                    reused = reused && !forceRestart,
                )
            }
        }
    }

    suspend fun diagnosticSnapshot(config: SshQrConfig?): String {
        val requestedTunnelName = config?.wireGuardTunnelName?.takeIf { it.isNotBlank() } ?: DEFAULT_TUNNEL_NAME
        val requestedHasWireGuard = !config?.wireGuardConfig.isNullOrBlank()
        val requestedConfig = config?.wireGuardConfig?.takeIf { it.isNotBlank() }?.let(::parseAndValidateWireGuardConfig)

        return withContext(Dispatchers.IO) {
            stateMutex.withLock {
                val tunnel = currentTunnel
                val backendState = tunnel?.let {
                    try {
                        backend.getState(it).name
                    } catch (cancelled: CancellationException) {
                        throw cancelled
                    } catch (throwable: Throwable) {
                        "ERROR:${throwable.javaClass.simpleName}:${throwable.message.orEmpty()}"
                    }
                } ?: "NONE"

                buildString {
                    append("requestedHasWireGuard=")
                    append(requestedHasWireGuard)
                    append(", requestedTunnel=")
                    append(requestedTunnelName)
                    append(", currentTunnel=")
                    append(tunnel?.tunnelName ?: "null")
                    append(", appTunnelState=")
                    append(tunnel?.state?.name ?: "NONE")
                    append(", backendState=")
                    append(backendState)
                    append(", hasCurrentConfig=")
                    append(currentConfig != null)
                    append(", configMatches=")
                    append(requestedConfig != null && currentConfig == requestedConfig)
                    val statsSummary = tunnel?.let {
                        runCatching {
                            summarizeWireGuardStats(backend.getStatistics(it), requestedConfig ?: currentConfig)
                        }.getOrElse { throwable ->
                            "ERROR:${throwable.javaClass.simpleName}:${throwable.message.orEmpty()}"
                        }
                    } ?: "NONE"
                    append(", stats=")
                    append(statsSummary)
                }
            }
        }
    }

    private suspend fun waitForHandshakeOrThrow(
        tunnel: AppTunnel,
        config: Config,
    ) {
        val deadline = System.currentTimeMillis() + HANDSHAKE_TIMEOUT_MS
        var lastSummary: String

        while (true) {
            val statistics = backend.getStatistics(tunnel)
            lastSummary = summarizeWireGuardStats(statistics, config)
            if (hasSuccessfulHandshake(statistics, config)) {
                return
            }
            if (System.currentTimeMillis() >= deadline) {
                throw IllegalStateException(
                    buildWireGuardHandshakeTimeoutMessage(config, tunnel.tunnelName, lastSummary),
                )
            }
            delay(HANDSHAKE_POLL_INTERVAL_MS)
        }
    }

    private class AppTunnel(
        val tunnelName: String,
    ) : Tunnel {
        var state: Tunnel.State = Tunnel.State.DOWN
            private set

        override fun getName(): String = tunnelName

        override fun onStateChange(newState: Tunnel.State) {
            state = newState
        }
    }

    companion object {
        private const val HANDSHAKE_TIMEOUT_MS = 12_000L
        private const val HANDSHAKE_POLL_INTERVAL_MS = 250L
    }
}

internal fun parseAndValidateWireGuardConfig(rawConfig: String): Config {
    val parsedConfig = try {
        BufferedReader(StringReader(rawConfig)).use(Config::parse)
    } catch (exception: BadConfigException) {
        throw IllegalArgumentException("Invalid WireGuard config.", exception)
    }

    val hasDefaultRoute = parsedConfig.getPeers()
        .flatMap { it.getAllowedIps() }
        .any { network ->
            network.getMask() == 0 && (
                network.getAddress() is Inet4Address ||
                    network.getAddress() is Inet6Address
                ) && network.getAddress().isAnyLocalAddress
        }
    require(!hasDefaultRoute) { "WireGuard config must use split tunnel routes." }

    return parsedConfig
}

internal fun resolveWireGuardTunnelName(rawTunnelName: String?): String {
    val candidate = rawTunnelName?.takeIf { it.isNotBlank() } ?: DEFAULT_TUNNEL_NAME
    require(!Tunnel.isNameInvalid(candidate)) {
        "WireGuard tunnel name is invalid."
    }
    return candidate
}

internal fun hasSuccessfulHandshake(
    statistics: Statistics,
    config: Config,
): Boolean = hasSuccessfulHandshake(collectPeerSnapshots(statistics, config))

internal fun summarizeWireGuardStats(
    statistics: Statistics,
    config: Config?,
): String = summarizeWireGuardStats(
    config?.getPeers()?.map { peer ->
        val peerStats = statistics.peer(peer.publicKey)
        WireGuardPeerSnapshot(
            endpoint = peer.getEndpoint().map { it.toString() }.orElse("none"),
            latestHandshakeEpochMillis = peerStats?.latestHandshakeEpochMillis() ?: 0L,
            rxBytes = peerStats?.rxBytes() ?: 0L,
            txBytes = peerStats?.txBytes() ?: 0L,
        )
    }.orEmpty(),
)

internal fun hasSuccessfulHandshake(
    peerSnapshots: Iterable<WireGuardPeerSnapshot>,
): Boolean = peerSnapshots.any { it.latestHandshakeEpochMillis > 0L }

internal fun summarizeWireGuardStats(
    peerSnapshots: Iterable<WireGuardPeerSnapshot>,
): String {
    val snapshots = peerSnapshots.toList()
    if (snapshots.isEmpty()) {
        return "peers=0"
    }
    return snapshots.joinToString(separator = ";") { snapshot ->
        "endpoint=${snapshot.endpoint},handshake=${snapshot.latestHandshakeEpochMillis},rx=${snapshot.rxBytes},tx=${snapshot.txBytes}"
    }
}

private fun collectPeerSnapshots(
    statistics: Statistics,
    config: Config,
): List<WireGuardPeerSnapshot> = config.getPeers().map { peer ->
    val peerStats = statistics.peer(peer.publicKey)
    WireGuardPeerSnapshot(
        endpoint = peer.getEndpoint().map { it.toString() }.orElse("none"),
        latestHandshakeEpochMillis = peerStats?.latestHandshakeEpochMillis() ?: 0L,
        rxBytes = peerStats?.rxBytes() ?: 0L,
        txBytes = peerStats?.txBytes() ?: 0L,
    )
}

internal data class WireGuardPeerSnapshot(
    val endpoint: String,
    val latestHandshakeEpochMillis: Long,
    val rxBytes: Long,
    val txBytes: Long,
)

internal fun buildWireGuardHandshakeTimeoutMessage(
    config: Config,
    tunnelName: String,
    statsSummary: String,
): String {
    val endpoints = config.getPeers()
        .mapNotNull { it.getEndpoint().orElse(null)?.toString() }
        .ifEmpty { listOf("unknown endpoint") }
        .joinToString()
    return "WireGuard tunnel $tunnelName started, but no peer handshake completed within 12s. Endpoint(s): $endpoints. Details: $statsSummary"
}

fun Throwable.toWireGuardMessage(): String = when (this) {
    is BackendException -> when (reason) {
        BackendException.Reason.VPN_NOT_AUTHORIZED -> "WireGuard permission is required."
        BackendException.Reason.DNS_RESOLUTION_FAILURE -> "WireGuard could not resolve a peer endpoint."
        BackendException.Reason.TUNNEL_MISSING_CONFIG -> "WireGuard config is missing."
        BackendException.Reason.UNABLE_TO_START_VPN -> "WireGuard VPN service could not start."
        BackendException.Reason.TUN_CREATION_ERROR -> "Android VPN tunnel creation failed."
        BackendException.Reason.GO_ACTIVATION_ERROR_CODE -> "wireguard-go failed to activate the tunnel."
        else -> message ?: "WireGuard failed."
    }

    else -> message ?: "WireGuard failed."
}
