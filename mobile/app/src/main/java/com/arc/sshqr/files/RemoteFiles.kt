package com.arc.sshqr.files

import android.net.Uri
import com.arc.sshqr.SessionConnectionState
import com.arc.sshqr.qr.SshQrConfig

const val ARC_FILES_ROOT_PATH = "/home/arc"

data class RemoteFileEntry(
    val path: String,
    val name: String,
    val isDirectory: Boolean,
    val sizeBytes: Long?,
    val modifiedEpochSeconds: Long?,
)

data class FilesUiState(
    val rootPath: String = ARC_FILES_ROOT_PATH,
    val currentPath: String = ARC_FILES_ROOT_PATH,
    val entries: List<RemoteFileEntry> = emptyList(),
    val isLoading: Boolean = false,
    val error: String? = null,
    val status: String? = null,
    val connectionState: SessionConnectionState = SessionConnectionState.IDLE,
    val reconnectAttempt: Int = 0,
) {
    val isReconnecting: Boolean
        get() = connectionState == SessionConnectionState.RECONNECTING
}

interface RemoteFilesRepository {
    suspend fun listDirectory(config: SshQrConfig, remotePath: String): List<RemoteFileEntry>

    suspend fun createDirectory(config: SshQrConfig, remotePath: String)

    suspend fun createFile(config: SshQrConfig, remotePath: String)

    suspend fun rename(config: SshQrConfig, sourcePath: String, targetPath: String)

    suspend fun delete(config: SshQrConfig, remotePath: String, isDirectory: Boolean)

    suspend fun upload(config: SshQrConfig, localUri: Uri, remotePath: String)

    suspend fun download(config: SshQrConfig, remotePath: String, targetUri: Uri)

    suspend fun reconnect(config: SshQrConfig)

    suspend fun close()
}

object RemotePathUtils {
    fun normalize(path: String, rootPath: String = ARC_FILES_ROOT_PATH): String {
        val rootSegments = splitSegments(rootPath)
        val pathSegments = splitSegments(path)
        val targetSegments = if (path.startsWith("/")) pathSegments else rootSegments + pathSegments
        return buildNormalized(rootSegments, targetSegments)
    }

    fun clampToRoot(path: String, rootPath: String = ARC_FILES_ROOT_PATH): String {
        val normalized = normalize(path, rootPath)
        return if (normalized.startsWith("$rootPath/") || normalized == rootPath) {
            normalized
        } else {
            rootPath
        }
    }

    fun parent(path: String, rootPath: String = ARC_FILES_ROOT_PATH): String? {
        val normalized = clampToRoot(path, rootPath)
        if (normalized == rootPath) {
            return null
        }
        val segments = splitSegments(normalized).dropLast(1)
        return if (segments.isEmpty()) "/" else "/" + segments.joinToString("/")
    }

    fun child(path: String, name: String, rootPath: String = ARC_FILES_ROOT_PATH): String {
        val safeName = sanitizeSegment(name)
        return if (path == "/") "/$safeName" else "${clampToRoot(path, rootPath)}/$safeName"
    }

    fun sibling(path: String, name: String, rootPath: String = ARC_FILES_ROOT_PATH): String {
        val parent = parent(path, rootPath) ?: rootPath
        return child(parent, name, rootPath)
    }

    fun sanitizeSegment(name: String): String {
        val trimmed = name.trim()
        require(trimmed.isNotEmpty()) { "Name cannot be empty." }
        require(trimmed != "." && trimmed != "..") { "Name is invalid." }
        require('/' !in trimmed && '\\' !in trimmed) { "Name cannot contain path separators." }
        return trimmed
    }

    private fun splitSegments(path: String): List<String> =
        path.split('/')
            .filter { it.isNotBlank() && it != "." }

    private fun buildNormalized(rootSegments: List<String>, targetSegments: List<String>): String {
        val stack = mutableListOf<String>()
        for (segment in targetSegments) {
            if (segment == "..") {
                if (stack.size > rootSegments.size) {
                    stack.removeAt(stack.lastIndex)
                }
            } else {
                stack += segment
            }
        }
        if (stack.size < rootSegments.size || stack.take(rootSegments.size) != rootSegments) {
            return "/" + rootSegments.joinToString("/")
        }
        return "/" + stack.joinToString("/")
    }
}
