package com.arc.sshqr.vpn

import org.junit.Assert.assertEquals
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
}
