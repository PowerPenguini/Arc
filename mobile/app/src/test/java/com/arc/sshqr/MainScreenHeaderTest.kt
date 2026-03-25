package com.arc.sshqr

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class MainScreenHeaderTest {

    @Test
    fun `header details show disconnected marker when vpn is inactive`() {
        assertEquals(
            "//DISCONNECTED",
            buildMenuHeaderDetails(
                host = "10.0.0.5",
                vpnTunnelName = null,
            ),
        )
    }

    @Test
    fun `header details show host and wireguard when vpn is active`() {
        assertEquals(
            "@10.0.0.5 / wireguard",
            buildMenuHeaderDetails(
                host = "10.0.0.5",
                vpnTunnelName = "arc",
            ),
        )
    }

    @Test
    fun `header details stay connected when tunnel is active even if ssh session is disconnected`() {
        assertEquals(
            "@10.0.0.5 / wireguard",
            buildMenuHeaderDetails(
                host = "10.0.0.5",
                vpnTunnelName = "arc",
            ),
        )
    }

    @Test
    fun `reconnect action is shown only when there is no active tunnel and app is not connecting`() {
        assertTrue(
            shouldShowReconnectAction(
                vpnTunnelName = null,
                sessionState = SessionConnectionState.DISCONNECTED,
            ),
        )
        assertTrue(
            shouldShowReconnectAction(
                vpnTunnelName = null,
                sessionState = SessionConnectionState.IDLE,
            ),
        )
        assertFalse(
            shouldShowReconnectAction(
                vpnTunnelName = null,
                sessionState = SessionConnectionState.CONNECTING,
            ),
        )
        assertFalse(
            shouldShowReconnectAction(
                vpnTunnelName = null,
                sessionState = SessionConnectionState.RECONNECTING,
            ),
        )
        assertFalse(
            shouldShowReconnectAction(
                vpnTunnelName = "arc",
                sessionState = SessionConnectionState.DISCONNECTED,
            ),
        )
        assertFalse(
            shouldShowReconnectAction(
                vpnTunnelName = "arc",
                sessionState = SessionConnectionState.FAILED,
            ),
        )
    }
}
