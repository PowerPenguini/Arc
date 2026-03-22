package com.arc.sshqr

enum class SessionConnectionState {
    IDLE,
    CONNECTING,
    CONNECTED,
    RECONNECTING,
    DISCONNECTED,
    FAILED,
    ;

    val isConnecting: Boolean
        get() = this == CONNECTING || this == RECONNECTING

    val isConnected: Boolean
        get() = this == CONNECTED
}
