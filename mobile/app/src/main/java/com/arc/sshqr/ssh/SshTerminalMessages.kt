package com.arc.sshqr.ssh

import com.arc.sshqr.SessionConnectionState
import com.arc.sshqr.qr.SshQrConfig
import java.io.IOException
import java.io.InterruptedIOException
import net.schmizz.sshj.transport.TransportException
import net.schmizz.sshj.userauth.UserAuthException

internal data class DisconnectReason(
    val userMessage: String,
    val recoverable: Boolean,
    val finalState: SessionConnectionState,
)

internal fun describeConnectionFailure(
    throwable: Throwable,
    activeConfig: SshQrConfig?,
): String = describeAuthFailure(throwable, activeConfig) ?: (throwable.message ?: "Unable to connect over SSH.")

internal fun describeAuthFailure(
    throwable: Throwable,
    activeConfig: SshQrConfig?,
): String? {
    val authThrowable = throwable.causalChain().firstOrNull { it is UserAuthException }
    val authMessage = authThrowable?.message ?: throwable.causalChain()
        .mapNotNull { it.message }
        .firstOrNull { it.contains("auth", ignoreCase = true) }

    if (authThrowable == null && authMessage?.contains("exhausted available auth methods", ignoreCase = true) != true) {
        return null
    }

    val methods = extractBracketedList(authMessage).orEmpty()
    val methodsSuffix = if (methods.isNotEmpty()) {
        " Serwer oferuje: $methods."
    } else {
        ""
    }
    val passphraseHint = activeConfig?.passphrase?.takeIf { it.isNotBlank() }?.let {
        ""
    } ?: " Jeśli klucz prywatny jest zaszyfrowany, dodaj poprawne hasło do klucza."

    return buildString {
        append("SSH auth failed. Serwer odrzucił logowanie kluczem publicznym dla użytkownika ${activeConfig?.username.orEmpty()}.")
        append(methodsSuffix)
        append(" Sprawdź username, zgodność klucza z authorized_keys i to, czy PubkeyAuthentication jest włączone na serwerze.")
        append(passphraseHint)
    }.trim()
}

internal fun classifyDisconnect(
    throwable: Throwable,
    fallback: String,
    activeConfig: SshQrConfig?,
    transportDisconnectMessage: String?,
): DisconnectReason {
    val authDetails = describeAuthFailure(throwable, activeConfig)
    if (authDetails != null) {
        return DisconnectReason(
            userMessage = authDetails,
            recoverable = false,
            finalState = SessionConnectionState.FAILED,
        )
    }

    val chain = throwable.causalChain().toList()
    val message = chain.mapNotNull { it.message }.firstOrNull { it.isNotBlank() }
        ?: transportDisconnectMessage
        ?: fallback
    val normalized = listOfNotNull(message, transportDisconnectMessage).joinToString(" ").lowercase()
    val recoverable = chain.any { it is IOException || it is TransportException } && (
        normalized.contains("timeout") ||
            normalized.contains("timed out") ||
            normalized.contains("connection reset") ||
            normalized.contains("broken pipe") ||
            normalized.contains("connection lost") ||
            normalized.contains("socket closed") ||
            normalized.contains("network is unreachable") ||
            normalized.contains("no route to host") ||
            normalized.contains("software caused connection abort") ||
            normalized.contains("connection aborted") ||
            normalized.contains("connection refused") ||
            normalized.contains("connection closed") ||
            throwable is InterruptedIOException
        )

    return DisconnectReason(
        userMessage = message,
        recoverable = recoverable,
        finalState = SessionConnectionState.FAILED,
    )
}

internal fun classifyTransportDisconnect(reason: String, message: String?): DisconnectReason {
    val transportMessage = buildString {
        append("SSH transport disconnected")
        if (reason.isNotBlank()) {
            append(": ")
            append(reason)
        }
        if (!message.isNullOrBlank()) {
            append(" - ")
            append(message)
        }
    }
    val normalized = transportMessage.lowercase()
    val recoverable =
        normalized.contains("connection reset") ||
            normalized.contains("broken pipe") ||
            normalized.contains("timeout") ||
            normalized.contains("timed out") ||
            normalized.contains("socket") ||
            normalized.contains("abort") ||
            normalized.contains("connection closed") ||
            normalized.contains("unknown")

    return DisconnectReason(
        userMessage = transportMessage,
        recoverable = recoverable,
        finalState = SessionConnectionState.FAILED,
    )
}

private fun extractBracketedList(message: String?): String? {
    if (message.isNullOrBlank()) {
        return null
    }
    val match = Regex("\\[(.*?)]").find(message) ?: return null
    return match.groupValues.getOrNull(1)?.takeIf { it.isNotBlank() }
}

private fun Throwable.causalChain(): Sequence<Throwable> = sequence {
    var current: Throwable? = this@causalChain
    while (current != null) {
        yield(current)
        current = current.cause
    }
}
