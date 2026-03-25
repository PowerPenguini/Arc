package com.arc.sshqr

import android.app.Application
import android.net.Uri
import android.util.Log
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.arc.sshqr.files.FilesUiState
import com.arc.sshqr.files.RemoteFileEntry
import com.arc.sshqr.files.RemoteFilesRepository
import com.arc.sshqr.files.RemotePathUtils
import com.arc.sshqr.files.SshjRemoteFilesRepository
import com.arc.sshqr.qr.SavedConfigStore
import com.arc.sshqr.qr.SharedPreferencesSavedConfigStore
import com.arc.sshqr.qr.SshQrConfig
import com.arc.sshqr.ssh.SshConnectionManager
import com.arc.sshqr.ssh.SshTerminalController
import com.arc.sshqr.vpn.WireGuardTunnelManager
import com.arc.sshqr.vpn.toWireGuardMessage
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

class MainViewModel(application: Application) : AndroidViewModel(application) {

    var uiState by mutableStateOf(MainUiState())
        private set

    var pendingVpnConfig: SshQrConfig? = null

    private val savedConfigStore: SavedConfigStore =
        SharedPreferencesSavedConfigStore(application.applicationContext)

    private val wireGuardTunnelManager = WireGuardTunnelManager(application.applicationContext)

    private val sshConnectionManager = SshConnectionManager(
        prepareTransport = { config, forceRestart ->
            val sessionInfo = wireGuardTunnelManager.ensureTunnelUp(config, forceRestart = forceRestart)
            Log.d(
                TAG,
                "prepareTransport: forceRestart=$forceRestart reused=${sessionInfo?.reused} tunnel=${sessionInfo?.tunnelName}",
            )
        },
    )

    private val remoteFilesRepository: RemoteFilesRepository =
        SshjRemoteFilesRepository(application.applicationContext, sshConnectionManager)

    val terminalController = SshTerminalController(
        appContext = application.applicationContext,
        scope = viewModelScope,
        connectionManager = sshConnectionManager,
        callbacks = object : SshTerminalController.Callbacks {
            override fun onStatusChanged(status: String) {
                Log.d(TAG, "onStatusChanged: $status")
                updateUiState { copy(status = status) }
            }

            override fun onSessionStateChanged(state: SessionConnectionState) {
                Log.d(TAG, "onSessionStateChanged: $state")
                updateUiState { copy(sessionState = state) }
            }

            override fun onConnected(config: SshQrConfig) {
                Log.d(TAG, "onConnected: ${config.username}@${config.host}:${config.port}")
                updateUiState {
                    copy(
                    screen = preferredConnectionScreen(uiState.screen),
                    config = config,
                    status = "Live shell on ${config.host}",
                    sessionState = SessionConnectionState.CONNECTED,
                    error = null,
                )
                }
            }

            override fun onDisconnected(message: String) {
                Log.d(TAG, "onDisconnected: $message")
                updateUiState {
                    copy(
                    screen = if (uiState.config != null) MainScreenRoute.Menu else MainScreenRoute.Scan,
                    status = message,
                    sessionState = SessionConnectionState.DISCONNECTED,
                )
                }
            }

            override fun onConnectionFailed(message: String) {
                Log.e(TAG, "onConnectionFailed: $message")
                updateUiState {
                    copy(
                    screen = if (uiState.config != null) MainScreenRoute.Menu else MainScreenRoute.Scan,
                    status = "SSH failed",
                    sessionState = SessionConnectionState.FAILED,
                    error = message,
                )
                }
            }
        },
        prepareReconnectTransport = { config ->
            val sessionInfo = wireGuardTunnelManager.ensureTunnelUp(config, forceRestart = true)
            Log.d(
                TAG,
                "prepareReconnectTransport: wireguard refreshed reused=${sessionInfo?.reused} tunnel=${sessionInfo?.tunnelName}",
            )
        },
        diagnoseTransport = { config ->
            wireGuardTunnelManager.diagnosticSnapshot(config)
        },
    )

    init {
        restoreSavedConfig()
    }

    fun onScanStarted() {
        Log.d(TAG, "onScanStarted")
        updateUiState {
            copy(
            screen = if (uiState.config != null) uiState.screen else MainScreenRoute.Scan,
            status = "Waiting for QR scan...",
            error = null,
            sessionState = SessionConnectionState.IDLE,
        )
        }
    }

    fun onQrParsed(config: SshQrConfig) {
        Log.d(TAG, "onQrParsed: ${config.username}@${config.host}:${config.port}")
        savedConfigStore.save(config)
        updateUiState {
            copy(
            screen = MainScreenRoute.Menu,
            config = config,
            status = "Connection profile loaded",
            error = null,
            sessionState = SessionConnectionState.IDLE,
            vpnTunnelName = null,
            pendingAutoConnect = false,
        )
        }
    }

    fun onQrRejected(message: String) {
        Log.e(TAG, "onQrRejected: $message")
        updateUiState {
            copy(
            screen = MainScreenRoute.Scan,
            status = "QR rejected",
            error = message,
            sessionState = SessionConnectionState.FAILED,
        )
        }
    }

    fun onScanCancelled() {
        Log.d(TAG, "onScanCancelled")
        updateUiState {
            copy(
            screen = if (uiState.config != null) uiState.screen else MainScreenRoute.Scan,
            status = "Scan cancelled",
            sessionState = SessionConnectionState.IDLE,
        )
        }
    }

    fun onScannerError(message: String) {
        Log.e(TAG, "onScannerError: $message")
        updateUiState {
            copy(
            screen = MainScreenRoute.Scan,
            status = "Scanner error",
            error = message,
            sessionState = SessionConnectionState.FAILED,
        )
        }
    }

    fun onVpnPermissionRequired(config: SshQrConfig) {
        Log.d(TAG, "onVpnPermissionRequired")
        pendingVpnConfig = config
        updateUiState {
            copy(
            screen = preferredConnectionScreen(uiState.screen),
            config = config,
            status = "WireGuard permission required",
            sessionState = SessionConnectionState.CONNECTING,
            error = null,
        )
        }
    }

    fun onVpnPermissionDenied() {
        Log.e(TAG, "onVpnPermissionDenied")
        pendingVpnConfig = null
        updateUiState {
            copy(
            screen = MainScreenRoute.Menu,
            status = "WireGuard permission denied",
            sessionState = SessionConnectionState.FAILED,
            error = "WireGuard permission is required.",
            vpnTunnelName = null,
        )
        }
    }

    fun connectWithTransport(config: SshQrConfig) {
        Log.d(TAG, "connectWithTransport: start")
        updateUiState {
            copy(
            screen = preferredConnectionScreen(uiState.screen),
            config = config,
            status = "Preparing transport",
            sessionState = SessionConnectionState.CONNECTING,
            error = null,
            pendingAutoConnect = false,
        )
        }
        viewModelScope.launch {
            val tunnelResult = runCatching {
                wireGuardTunnelManager.ensureTunnelUp(config)
            }

            tunnelResult.onSuccess { sessionInfo ->
                Log.d(TAG, "connectWithTransport: wireguard ok reused=${sessionInfo?.reused} tunnel=${sessionInfo?.tunnelName}")
                updateUiState {
                    copy(
                    screen = preferredConnectionScreen(uiState.screen),
                    config = config,
                    status = sessionInfo?.let {
                        if (it.reused) {
                            "WireGuard ready on ${it.tunnelName}"
                        } else {
                            "WireGuard started on ${it.tunnelName}"
                        }
                    } ?: "Connecting to SSH",
                    sessionState = SessionConnectionState.CONNECTING,
                    error = null,
                    vpnTunnelName = sessionInfo?.tunnelName,
                )
                }
                terminalController.connect(config)
            }.onFailure { failure ->
                Log.e(TAG, "connectWithTransport: wireguard failed", failure)
                updateUiState {
                    copy(
                    config = config,
                    status = "WireGuard failed",
                    sessionState = SessionConnectionState.FAILED,
                    error = failure.toWireGuardMessage(),
                    vpnTunnelName = null,
                )
                }
            }
        }
    }

    fun openTerminal() {
        Log.d(TAG, "openTerminal")
        if (uiState.config == null) return
        updateUiState { copy(screen = MainScreenRoute.Session) }
    }

    fun showSettings() {
        Log.d(TAG, "showSettings")
        if (uiState.config == null) return
        updateUiState { copy(screen = MainScreenRoute.Settings) }
    }

    fun openFiles() {
        Log.d(TAG, "openFiles")
        if (uiState.config == null) return
        updateUiState {
            copy(
                screen = MainScreenRoute.Files,
                files = files.copy(
                connectionState = SessionConnectionState.CONNECTING,
                reconnectAttempt = 0,
                error = null,
            ),
            )
        }
        refreshFiles()
    }

    fun reconnectTerminal() {
        val config = uiState.config ?: return
        Log.d(TAG, "reconnectTerminal")
        connectWithTransport(config)
    }

    fun refreshFiles(path: String = uiState.files.currentPath) {
        val config = uiState.config ?: return
        val targetPath = RemotePathUtils.clampToRoot(path, uiState.files.rootPath)
        updateFilesState {
            copy(
                currentPath = targetPath,
                isLoading = true,
                error = null,
                status = "Loading $targetPath",
                connectionState = if (connectionState == SessionConnectionState.CONNECTED) {
                    SessionConnectionState.CONNECTED
                } else {
                    SessionConnectionState.CONNECTING
                },
                reconnectAttempt = 0,
            )
        }
        viewModelScope.launch(Dispatchers.IO) {
            listDirectoryWithReconnect(config, targetPath).onSuccess { entries ->
                updateFilesState {
                    copy(
                        currentPath = targetPath,
                        entries = entries,
                        isLoading = false,
                        error = null,
                        status = "${entries.size} item(s)",
                        connectionState = SessionConnectionState.CONNECTED,
                        reconnectAttempt = 0,
                    )
                }
            }.onFailure { failure ->
                Log.e(TAG, "refreshFiles failed", failure)
                updateFilesState {
                    copy(
                        currentPath = targetPath,
                        entries = emptyList(),
                        isLoading = false,
                        error = failure.message ?: "Unable to load files.",
                        status = null,
                        connectionState = SessionConnectionState.FAILED,
                        reconnectAttempt = 0,
                    )
                }
            }
        }
    }

    fun openDirectory(entry: RemoteFileEntry) {
        if (!entry.isDirectory) return
        refreshFiles(entry.path)
    }

    fun navigateFilesUp() {
        val parent = RemotePathUtils.parent(uiState.files.currentPath, uiState.files.rootPath) ?: return
        refreshFiles(parent)
    }

    fun createDirectory(name: String) {
        val config = uiState.config ?: return
        val targetPath = runCatching {
            RemotePathUtils.child(uiState.files.currentPath, name, uiState.files.rootPath)
        }.getOrElse { failure ->
            setFilesError(failure.message ?: "Folder name is invalid.")
            return
        }
        runFileOperation(
            loadingStatus = "Creating folder",
            successStatus = "Folder created",
        ) {
            remoteFilesRepository.createDirectory(config, targetPath)
        }
    }

    fun createFile(name: String) {
        val config = uiState.config ?: return
        val targetPath = runCatching {
            RemotePathUtils.child(uiState.files.currentPath, name, uiState.files.rootPath)
        }.getOrElse { failure ->
            setFilesError(failure.message ?: "File name is invalid.")
            return
        }
        runFileOperation(
            loadingStatus = "Creating file",
            successStatus = "File created",
        ) {
            remoteFilesRepository.createFile(config, targetPath)
        }
    }

    fun renameFile(entry: RemoteFileEntry, newName: String) {
        val config = uiState.config ?: return
        val targetPath = runCatching {
            RemotePathUtils.sibling(entry.path, newName, uiState.files.rootPath)
        }.getOrElse { failure ->
            setFilesError(failure.message ?: "Name is invalid.")
            return
        }
        runFileOperation(
            loadingStatus = "Renaming ${entry.name}",
            successStatus = "Renamed to $newName",
        ) {
            remoteFilesRepository.rename(config, entry.path, targetPath)
        }
    }

    fun deleteFile(entry: RemoteFileEntry) {
        val config = uiState.config ?: return
        runFileOperation(
            loadingStatus = "Deleting ${entry.name}",
            successStatus = "Deleted ${entry.name}",
        ) {
            remoteFilesRepository.delete(config, entry.path, entry.isDirectory)
        }
    }

    fun uploadFile(uri: Uri, displayName: String) {
        val config = uiState.config ?: return
        val targetPath = runCatching {
            RemotePathUtils.child(uiState.files.currentPath, displayName, uiState.files.rootPath)
        }.getOrElse { failure ->
            setFilesError(failure.message ?: "Upload name is invalid.")
            return
        }
        runFileOperation(
            loadingStatus = "Uploading $displayName",
            successStatus = "Uploaded $displayName",
        ) {
            remoteFilesRepository.upload(config, uri, targetPath)
        }
    }

    fun downloadFile(entry: RemoteFileEntry, targetUri: Uri) {
        val config = uiState.config ?: return
        if (entry.isDirectory) {
            setFilesError("Directory download is not supported.")
            return
        }
        runFileOperation(
            loadingStatus = "Downloading ${entry.name}",
            successStatus = "Downloaded ${entry.name}",
            refreshAfterSuccess = false,
        ) {
            remoteFilesRepository.download(config, entry.path, targetUri)
        }
    }

    fun showMenu() {
        Log.d(TAG, "showMenu")
        if (uiState.config == null) {
            updateUiState { copy(screen = MainScreenRoute.Scan) }
            return
        }
        updateUiState { copy(screen = MainScreenRoute.Menu) }
    }

    fun consumePendingAutoConnect() {
        if (!uiState.pendingAutoConnect) return
        updateUiState { copy(pendingAutoConnect = false) }
    }

    fun resetToScan() {
        Log.d(TAG, "resetToScan")
        pendingVpnConfig = null
        savedConfigStore.clear()
        terminalController.disconnect()
        viewModelScope.launch(Dispatchers.IO) {
            remoteFilesRepository.close()
        }
        uiState = MainUiState()
    }

    private fun restoreSavedConfig() {
        val config = savedConfigStore.load() ?: return
        Log.d(TAG, "restoreSavedConfig: ${config.username}@${config.host}:${config.port}")
        updateUiState {
            copy(
            screen = MainScreenRoute.Menu,
            config = config,
            status = "Restored saved profile",
            error = null,
            sessionState = SessionConnectionState.IDLE,
            vpnTunnelName = null,
            pendingAutoConnect = true,
        )
        }
    }

    override fun onCleared() {
        terminalController.close()
        viewModelScope.launch(Dispatchers.IO) {
            remoteFilesRepository.close()
        }
        super.onCleared()
    }

    private fun runFileOperation(
        loadingStatus: String,
        successStatus: String,
        refreshAfterSuccess: Boolean = true,
        block: suspend () -> Unit,
    ) {
        updateFilesState {
            copy(
                isLoading = true,
                error = null,
                status = loadingStatus,
                connectionState = if (connectionState == SessionConnectionState.FAILED) {
                    SessionConnectionState.CONNECTING
                } else {
                    connectionState
                },
            )
        }
        viewModelScope.launch(Dispatchers.IO) {
            runCatching {
                block()
            }.onSuccess {
                updateFilesState {
                    copy(
                        isLoading = false,
                        error = null,
                        status = successStatus,
                        connectionState = SessionConnectionState.CONNECTED,
                        reconnectAttempt = 0,
                    )
                }
                if (refreshAfterSuccess) {
                    refreshFiles(uiState.files.currentPath)
                }
            }.onFailure { failure ->
                Log.e(TAG, "file operation failed", failure)
                setFilesError(failure.message ?: "File operation failed.")
            }
        }
    }

    private fun setFilesError(message: String) {
        updateFilesState {
            copy(
                isLoading = false,
                error = message,
                status = null,
                connectionState = SessionConnectionState.FAILED,
            )
        }
    }

    private suspend fun listDirectoryWithReconnect(
        config: SshQrConfig,
        targetPath: String,
    ): Result<List<RemoteFileEntry>> {
        repeat(FILES_MAX_RECONNECT_ATTEMPTS) { attempt ->
            val result = runCatching {
                remoteFilesRepository.listDirectory(config, targetPath)
            }
            if (result.isSuccess) {
                return result
            }

            val failure = result.exceptionOrNull() ?: return result
            if (!SshjRemoteFilesRepository.isRecoverableConnectionFailure(failure) || attempt == FILES_MAX_RECONNECT_ATTEMPTS - 1) {
                return result
            }

            val nextAttempt = attempt + 1
            updateFilesState {
                copy(
                    isLoading = true,
                    error = null,
                    status = "Reconnecting... ($nextAttempt/$FILES_MAX_RECONNECT_ATTEMPTS)",
                    connectionState = SessionConnectionState.RECONNECTING,
                    reconnectAttempt = nextAttempt,
                )
            }
            runCatching {
                remoteFilesRepository.reconnect(config)
            }.onFailure { reconnectFailure ->
                Log.w(TAG, "files reconnect failed", reconnectFailure)
            }
        }

        return Result.failure(IllegalStateException("Unable to load files."))
    }

    private inline fun updateUiState(transform: MainUiState.() -> MainUiState) {
        uiState = uiState.transform()
    }

    private inline fun updateFilesState(transform: FilesUiState.() -> FilesUiState) {
        updateUiState {
            copy(files = files.transform())
        }
    }

    companion object {
        private const val TAG = "MainViewModel"
        private const val FILES_MAX_RECONNECT_ATTEMPTS = 3
    }
}

internal fun preferredConnectionScreen(currentScreen: MainScreenRoute): MainScreenRoute =
    when (currentScreen) {
        MainScreenRoute.Session -> MainScreenRoute.Session
        MainScreenRoute.Files -> MainScreenRoute.Files
        else -> MainScreenRoute.Menu
    }
