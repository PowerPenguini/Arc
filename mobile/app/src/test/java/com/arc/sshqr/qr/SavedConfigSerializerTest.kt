package com.arc.sshqr.qr

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class SavedConfigSerializerTest {

    @Test
    fun `round trip preserves all fields`() {
        val config = SshQrConfig(
            host = "10.0.0.1",
            port = 22,
            username = "arc",
            privateKeyPem = "-----BEGIN PRIVATE KEY-----\nabc123\n-----END PRIVATE KEY-----",
            passphrase = "secret",
            wireGuardConfig = """
                [Interface]
                Address = 10.0.0.3/32
                
                [Peer]
                AllowedIPs = 10.0.0.1/32
            """.trimIndent(),
            wireGuardTunnelName = "arc-mobile",
        )

        val restored = SavedConfigSerializer.deserialize(SavedConfigSerializer.serialize(config))

        assertEquals(config, restored)
    }

    @Test
    fun `round trip preserves nullable fields`() {
        val config = SshQrConfig(
            host = "example.com",
            port = 2222,
            username = "demo",
            privateKeyPem = "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----",
            passphrase = null,
            wireGuardConfig = null,
            wireGuardTunnelName = null,
        )

        val restored = SavedConfigSerializer.deserialize(SavedConfigSerializer.serialize(config))

        assertEquals(config.host, restored.host)
        assertEquals(config.port, restored.port)
        assertEquals(config.username, restored.username)
        assertEquals(config.privateKeyPem, restored.privateKeyPem)
        assertNull(restored.passphrase)
        assertNull(restored.wireGuardConfig)
        assertNull(restored.wireGuardTunnelName)
    }
}
