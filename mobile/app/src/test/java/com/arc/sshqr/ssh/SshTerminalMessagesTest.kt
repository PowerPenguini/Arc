package com.arc.sshqr.ssh

import com.arc.sshqr.qr.SshQrConfig
import java.io.IOException
import net.schmizz.sshj.userauth.UserAuthException
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class SshTerminalMessagesTest {

    @Test
    fun `describe auth failure includes username and methods`() {
        val config = testConfig()
        val throwable = UserAuthException("Exhausted available auth methods [publickey,password]")

        val message = describeAuthFailure(throwable, config)

        assertTrue(message!!.contains("demo"))
        assertTrue(message.contains("publickey,password"))
    }

    @Test
    fun `classify disconnect marks socket failures recoverable`() {
        val reason = classifyDisconnect(
            throwable = IOException("Connection reset by peer"),
            fallback = "fallback",
            activeConfig = testConfig(),
            transportDisconnectMessage = null,
        )

        assertTrue(reason.recoverable)
        assertEquals("Connection reset by peer", reason.userMessage)
    }

    @Test
    fun `classify transport disconnect keeps readable message`() {
        val reason = classifyTransportDisconnect("connection-lost", "socket closed")

        assertTrue(reason.recoverable)
        assertTrue(reason.userMessage.contains("connection-lost"))
        assertTrue(reason.userMessage.contains("socket closed"))
    }

    @Test
    fun `describe connection failure falls back to throwable message`() {
        val message = describeConnectionFailure(
            throwable = IllegalStateException("boom"),
            activeConfig = null,
        )

        assertEquals("boom", message)
    }

    @Test
    fun `classify disconnect keeps auth failures non recoverable`() {
        val reason = classifyDisconnect(
            throwable = UserAuthException("Exhausted available auth methods [publickey]"),
            fallback = "fallback",
            activeConfig = testConfig(),
            transportDisconnectMessage = null,
        )

        assertFalse(reason.recoverable)
        assertTrue(reason.userMessage.contains("SSH auth failed"))
    }

    private fun testConfig(): SshQrConfig = SshQrConfig(
        host = "host",
        port = 22,
        username = "demo",
        privateKeyPem = "pem",
        passphrase = null,
        wireGuardConfig = null,
        wireGuardTunnelName = null,
    )
}
