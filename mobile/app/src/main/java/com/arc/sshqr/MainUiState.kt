package com.arc.sshqr

import com.arc.sshqr.files.FilesUiState
import com.arc.sshqr.qr.SshQrConfig

data class MainUiState(
    val status: String = "Scan one QR code with host, user and private key.",
    val screen: MainScreenRoute = MainScreenRoute.Scan,
    val config: SshQrConfig? = null,
    val sessionState: SessionConnectionState = SessionConnectionState.IDLE,
    val error: String? = null,
    val vpnTunnelName: String? = null,
    val pendingAutoConnect: Boolean = false,
    val files: FilesUiState = FilesUiState(),
) {
    val isConnecting: Boolean
        get() = sessionState.isConnecting

    val isConnected: Boolean
        get() = sessionState.isConnected
}

enum class MainScreenRoute {
    Scan,
    Menu,
    Files,
    Settings,
    Session,
}
