package com.arc.sshqr

import android.net.VpnService
import android.os.Bundle
import android.util.Log
import androidx.activity.ComponentActivity
import androidx.activity.OnBackPressedCallback
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import com.arc.sshqr.qr.SshQrConfig
import com.arc.sshqr.qr.SshQrParser
import com.arc.sshqr.ui.theme.ArcSshTheme

class MainActivity : ComponentActivity() {

    private val viewModel: MainViewModel by viewModels()
    private val sessionBackCallback =
        object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                if (viewModel.uiState.screen == MainScreenRoute.Session) {
                    viewModel.showMenu()
                    return
                }
                isEnabled = false
                onBackPressedDispatcher.onBackPressed()
                isEnabled = true
            }
        }

    private val vpnPermissionLauncher =
        registerForActivityResult(ActivityResultContracts.StartActivityForResult()) {
            Log.d(TAG, "vpnPermissionLauncher result")
            val config = viewModel.pendingVpnConfig ?: return@registerForActivityResult
            viewModel.pendingVpnConfig = null

            if (VpnService.prepare(this) != null) {
                viewModel.onVpnPermissionDenied()
                return@registerForActivityResult
            }

            viewModel.connectWithTransport(config)
        }

    override fun onCreate(savedInstanceState: Bundle?) {
        installSplashScreen()
        super.onCreate(savedInstanceState)
        Log.d(TAG, "onCreate savedInstanceState=${savedInstanceState != null}")
        onBackPressedDispatcher.addCallback(this, sessionBackCallback)
        enableImmersiveMode()

        setContent {
            ArcSshTheme {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MaterialTheme.colorScheme.background,
                ) {
                    MainScreen(
                        state = viewModel.uiState,
                        terminalController = viewModel.terminalController,
                        onScanStarted = viewModel::onScanStarted,
                        onScanCancelled = viewModel::onScanCancelled,
                        onScanError = viewModel::onScannerError,
                        onQrPayloadScanned = ::handleQrPayload,
                        onAutoConnect = { config ->
                            viewModel.consumePendingAutoConnect()
                            startConnection(config)
                        },
                        onTerminalClick = {
                            viewModel.openTerminal()
                            viewModel.uiState.config?.takeUnless {
                                viewModel.uiState.sessionState.isConnecting || viewModel.uiState.sessionState.isConnected
                            }?.let(::startConnection)
                        },
                        onFilesClick = viewModel::openFiles,
                        onRefreshFiles = { viewModel.refreshFiles() },
                        onNavigateFilesUp = viewModel::navigateFilesUp,
                        onOpenDirectory = viewModel::openDirectory,
                        onCreateDirectory = viewModel::createDirectory,
                        onCreateFile = viewModel::createFile,
                        onRenameFile = viewModel::renameFile,
                        onDeleteFile = viewModel::deleteFile,
                        onUploadFile = viewModel::uploadFile,
                        onDownloadFile = viewModel::downloadFile,
                        onSettingsClick = viewModel::showSettings,
                        onForgetSavedConfig = viewModel::resetToScan,
                        onBackToMenu = viewModel::showMenu,
                    )
                }
            }
        }
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        Log.d(TAG, "onWindowFocusChanged hasFocus=$hasFocus")
        if (hasFocus) {
            enableImmersiveMode()
        }
    }

    private fun handleQrPayload(payload: String) {
        Log.d(TAG, "handleQrPayload rawLength=${payload.length}")
        SshQrParser.parse(payload).onSuccess { config ->
            viewModel.onQrParsed(config)
            startConnection(config)
        }.onFailure { failure ->
            viewModel.onQrRejected(failure.message ?: "QR payload is invalid.")
        }
    }

    private fun startConnection(config: SshQrConfig) {
        Log.d(TAG, "startConnection wireguard=${!config.wireGuardConfig.isNullOrBlank()}")
        val permissionIntent = if (config.wireGuardConfig.isNullOrBlank()) null else VpnService.prepare(this)
        if (permissionIntent != null) {
            viewModel.onVpnPermissionRequired(config)
            vpnPermissionLauncher.launch(permissionIntent)
            return
        }
        viewModel.connectWithTransport(config)
    }

    private fun enableImmersiveMode() {
        WindowCompat.setDecorFitsSystemWindows(window, false)
        WindowInsetsControllerCompat(window, window.decorView).apply {
            hide(WindowInsetsCompat.Type.systemBars())
            systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        }
    }

    companion object {
        private const val TAG = "MainActivity"
    }
}
