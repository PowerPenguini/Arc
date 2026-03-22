package com.arc.sshqr.qr

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class SshQrParserTest {

    @Test
    fun `parse accepts valid payload`() {
        val payload = """
            {
              "host": "example.com",
              "username": "arc",
              "privateKeyPem": "-----BEGIN OPENSSH PRIVATE KEY-----\\nabc\\n-----END OPENSSH PRIVATE KEY-----",
              "port": 2202
            }
        """.trimIndent()

        val result = SshQrParser.parse(payload)

        assertTrue(result.isSuccess)
        val config = result.getOrThrow()
        assertEquals("example.com", config.host)
        assertEquals("arc", config.username)
        assertEquals(2202, config.port)
        assertTrue(config.privateKeyPem.contains("\nabc\n"))
        assertNull(config.passphrase)
        assertNull(config.wireGuardConfig)
        assertNull(config.wireGuardTunnelName)
    }

    @Test
    fun `parse defaults port to 22`() {
        val payload = """
            {
              "host": "example.com",
              "username": "arc",
              "privateKeyPem": "-----BEGIN OPENSSH PRIVATE KEY-----\\nabc\\n-----END OPENSSH PRIVATE KEY-----"
            }
        """.trimIndent()

        val config = SshQrParser.parse(payload).getOrThrow()

        assertEquals(22, config.port)
    }

    @Test
    fun `parse accepts optional wireguard payload`() {
        val payload = """
            {
              "host": "ssh.internal",
              "username": "arc",
              "privateKeyPem": "-----BEGIN OPENSSH PRIVATE KEY-----\\nabc\\n-----END OPENSSH PRIVATE KEY-----",
              "wireguardTunnelName": "arcvpn",
              "wireguardConfig": "[Interface]\\nPrivateKey = test\\nAddress = 10.0.0.2/32\\n\\n[Peer]\\nPublicKey = peer\\nAllowedIPs = 10.0.0.0/24\\nEndpoint = vpn.example.com:51820"
            }
        """.trimIndent()

        val config = SshQrParser.parse(payload).getOrThrow()
        val wireGuardConfig = requireNotNull(config.wireGuardConfig)

        assertEquals("arcvpn", config.wireGuardTunnelName)
        assertTrue(wireGuardConfig.contains("[Interface]\n"))
        assertTrue(wireGuardConfig.contains("AllowedIPs = 10.0.0.0/24"))
    }

    @Test
    fun `parse rejects invalid json`() {
        val result = SshQrParser.parse("not-json")

        assertTrue(result.isFailure)
        assertEquals("QR payload is not valid JSON.", result.exceptionOrNull()?.message)
    }
}
