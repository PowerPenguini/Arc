package com.arc.sshqr

import org.junit.Assert.assertEquals
import org.junit.Test

class MainScreenHeaderTest {

    @Test
    fun `header details show disconnected marker when vpn is inactive`() {
        assertEquals(
            "//DISCONNECTED",
            buildMenuHeaderDetails(
                host = "10.0.0.5",
                vpnTunnelName = null,
                sessionState = SessionConnectionState.IDLE,
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
                sessionState = SessionConnectionState.CONNECTED,
            ),
        )
    }

    @Test
    fun `header details show disconnected marker when session is disconnected even if vpn name is cached`() {
        assertEquals(
            "//DISCONNECTED",
            buildMenuHeaderDetails(
                host = "10.0.0.5",
                vpnTunnelName = "arc",
                sessionState = SessionConnectionState.DISCONNECTED,
            ),
        )
    }
}
