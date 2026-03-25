package com.arc.sshqr.vpn

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class WireGuardTunnelManagerTest {

    @Test
    fun `parse accepts split tunnel config`() {
        val config = parseAndValidateWireGuardConfig(
            """
            [Interface]
            PrivateKey = En/amX38X7POn/c65YILvb4xfWZALdGH20fh39lZdJQ=
            Address = 10.20.0.2/32
            DNS = 10.20.0.1

            [Peer]
            PublicKey = w+8T2QOH0QxAkpyHu1aT3jxhqOGBGOoWbivg+dBrsKY=
            AllowedIPs = 10.20.0.0/24, 192.168.50.0/24
            Endpoint = vpn.example.com:51820
            """.trimIndent(),
        )

        assertEquals(1, config.getPeers().size)
        assertTrue(config.getPeers().first().getAllowedIps().any { it.toString() == "10.20.0.0/24" })
    }

    @Test
    fun `parse rejects full tunnel config`() {
        val result = runCatching {
            parseAndValidateWireGuardConfig(
                """
                [Interface]
                PrivateKey = En/amX38X7POn/c65YILvb4xfWZALdGH20fh39lZdJQ=
                Address = 10.20.0.2/32

                [Peer]
                PublicKey = w+8T2QOH0QxAkpyHu1aT3jxhqOGBGOoWbivg+dBrsKY=
                AllowedIPs = 0.0.0.0/0
                Endpoint = vpn.example.com:51820
                """.trimIndent(),
            )
        }

        assertTrue(result.isFailure)
        assertEquals("WireGuard config must use split tunnel routes.", result.exceptionOrNull()?.message)
    }

    @Test
    fun `resolve defaults tunnel name to arc`() {
        assertEquals("arc", resolveWireGuardTunnelName(null))
    }

    @Test
    fun `handshake helper reports success when peer has latest handshake`() {
        val snapshots = listOf(
            WireGuardPeerSnapshot(
                endpoint = "vpn.example.com:51820",
                latestHandshakeEpochMillis = 1234L,
                rxBytes = 10L,
                txBytes = 20L,
            ),
        )

        assertTrue(hasSuccessfulHandshake(snapshots))
    }

    @Test
    fun `handshake helper reports failure when peer has no handshake yet`() {
        val snapshots = listOf(
            WireGuardPeerSnapshot(
                endpoint = "vpn.example.com:51820",
                latestHandshakeEpochMillis = 0L,
                rxBytes = 0L,
                txBytes = 128L,
            ),
        )

        assertFalse(hasSuccessfulHandshake(snapshots))
    }

    @Test
    fun `stats summary includes endpoint handshake rx and tx`() {
        val summary = summarizeWireGuardStats(
            listOf(
                WireGuardPeerSnapshot(
                    endpoint = "vpn.example.com:51820",
                    latestHandshakeEpochMillis = 1234L,
                    rxBytes = 10L,
                    txBytes = 20L,
                ),
            ),
        )

        assertTrue(summary.contains("endpoint=vpn.example.com:51820"))
        assertTrue(summary.contains("handshake=1234"))
        assertTrue(summary.contains("rx=10"))
        assertTrue(summary.contains("tx=20"))
    }

    @Test
    fun `timeout message includes endpoint and stats summary`() {
        val config = parseAndValidateWireGuardConfig(
            """
            [Interface]
            PrivateKey = En/amX38X7POn/c65YILvb4xfWZALdGH20fh39lZdJQ=
            Address = 10.20.0.2/32

            [Peer]
            PublicKey = w+8T2QOH0QxAkpyHu1aT3jxhqOGBGOoWbivg+dBrsKY=
            AllowedIPs = 10.20.0.0/24
            Endpoint = vpn.example.com:51820
            """.trimIndent(),
        )

        val message = buildWireGuardHandshakeTimeoutMessage(
            config = config,
            tunnelName = "arc",
            statsSummary = "endpoint=vpn.example.com:51820,handshake=0,rx=0,tx=128",
        )

        assertTrue(message.contains("vpn.example.com:51820"))
        assertTrue(message.contains("no peer handshake completed within 12s"))
        assertTrue(message.contains("tx=128"))
    }

    @Test
    fun `wireguard config fingerprint ignores formatting-only differences`() {
        val compact = """
            [Interface]
            PrivateKey = test
            Address = 10.20.0.2/32

            [Peer]
            PublicKey = peer
            AllowedIPs = 10.20.0.0/24
            Endpoint = vpn.example.com:51820
        """.trimIndent()
        val spaced = """

            [Interface]
              PrivateKey = test
              Address = 10.20.0.2/32


            [Peer]
              PublicKey = peer
              AllowedIPs = 10.20.0.0/24
              Endpoint = vpn.example.com:51820

        """.trimIndent()

        assertEquals(
            fingerprintWireGuardConfig(compact),
            fingerprintWireGuardConfig(spaced),
        )
    }
}
