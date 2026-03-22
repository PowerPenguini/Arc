package com.arc.sshqr.files

import android.content.Context
import android.net.Uri
import com.arc.sshqr.qr.SshQrConfig
import com.arc.sshqr.ssh.SshConnectionManager
import java.io.IOException
import java.util.EnumSet
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import net.schmizz.sshj.sftp.OpenMode
import net.schmizz.sshj.sftp.SFTPClient
import net.schmizz.sshj.transport.TransportException

class SshjRemoteFilesRepository(
    private val appContext: Context,
    private val connectionManager: SshConnectionManager,
) : RemoteFilesRepository {

    override suspend fun listDirectory(config: SshQrConfig, remotePath: String): List<RemoteFileEntry> =
        withSftp(config) { sftp ->
            sftp.ls(remotePath)
                .asSequence()
                .filterNot { it.name == "." || it.name == ".." }
                .map { entry ->
                    RemoteFileEntry(
                        path = RemotePathUtils.clampToRoot(entry.path),
                        name = entry.name,
                        isDirectory = entry.isDirectory,
                        sizeBytes = entry.attributes.size,
                        modifiedEpochSeconds = entry.attributes.mtime.toLong(),
                    )
                }
                .sortedWith(compareByDescending<RemoteFileEntry> { it.isDirectory }.thenBy { it.name.lowercase() })
                .toList()
        }

    override suspend fun createDirectory(config: SshQrConfig, remotePath: String) {
        withSftp(config) { sftp ->
            sftp.mkdir(remotePath)
        }
    }

    override suspend fun createFile(config: SshQrConfig, remotePath: String) {
        withSftp(config) { sftp ->
            sftp.open(
                remotePath,
                EnumSet.of(OpenMode.CREAT, OpenMode.WRITE, OpenMode.TRUNC),
            ).use { }
        }
    }

    override suspend fun rename(config: SshQrConfig, sourcePath: String, targetPath: String) {
        withSftp(config) { sftp ->
            sftp.rename(sourcePath, targetPath)
        }
    }

    override suspend fun delete(config: SshQrConfig, remotePath: String, isDirectory: Boolean) {
        withSftp(config) { sftp ->
            if (isDirectory) {
                sftp.rmdir(remotePath)
            } else {
                sftp.rm(remotePath)
            }
        }
    }

    override suspend fun upload(config: SshQrConfig, localUri: Uri, remotePath: String) {
        withSftp(config) { sftp ->
            appContext.contentResolver.openInputStream(localUri)?.use { input ->
                sftp.open(
                    remotePath,
                    EnumSet.of(OpenMode.CREAT, OpenMode.WRITE, OpenMode.TRUNC),
                ).use { remoteFile ->
                    remoteFile.RemoteFileOutputStream().use { output ->
                        input.copyTo(output)
                    }
                }
            } ?: throw IOException("Unable to open selected file.")
        }
    }

    override suspend fun download(config: SshQrConfig, remotePath: String, targetUri: Uri) {
        withSftp(config) { sftp ->
            appContext.contentResolver.openOutputStream(targetUri)?.use { output ->
                sftp.open(remotePath, EnumSet.of(OpenMode.READ)).use { remoteFile ->
                    remoteFile.RemoteFileInputStream().use { input ->
                        input.copyTo(output)
                    }
                }
            } ?: throw IOException("Unable to open destination file.")
        }
    }

    override suspend fun reconnect(config: SshQrConfig) {
        connectionManager.reconnect(config)
    }

    override suspend fun close() {
        connectionManager.disconnectSession()
    }

    private suspend fun <T> withSftp(config: SshQrConfig, block: suspend (SFTPClient) -> T): T =
        withContext(Dispatchers.IO) {
            val ssh = connectionManager.ensureConnected(config)
            ssh.newSFTPClient().use { sftp ->
                block(sftp)
            }
        }

    companion object {
        fun isRecoverableConnectionFailure(throwable: Throwable): Boolean =
            throwable is TransportException ||
                throwable is IOException ||
                throwable.cause?.let(::isRecoverableConnectionFailure) == true
    }
}
