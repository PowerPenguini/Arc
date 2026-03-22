package com.arc.sshqr

import android.content.ContentResolver
import android.net.Uri
import android.net.VpnService
import android.os.Bundle
import android.os.SystemClock
import android.provider.OpenableColumns
import android.util.Log
import android.util.TypedValue
import android.view.inputmethod.InputMethodManager
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.core.content.res.ResourcesCompat
import androidx.core.content.getSystemService
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material.icons.outlined.QrCodeScanner
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.Terminal
import androidx.compose.material3.Button
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.mutableStateOf
import androidx.compose.foundation.rememberScrollState
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.input.pointer.pointerInteropFilter
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import com.arc.sshqr.files.FilesUiState
import com.arc.sshqr.files.RemoteFileEntry
import com.arc.sshqr.qr.SshQrConfig
import com.arc.sshqr.qr.SshQrParser
import com.arc.sshqr.ssh.SshTerminalController
import com.arc.sshqr.ui.theme.ArcTerminalAccent
import com.arc.sshqr.ui.theme.ArcTerminalFontFamily
import com.arc.sshqr.ui.theme.ArcSshTheme
import com.google.mlkit.vision.codescanner.GmsBarcodeScanner
import com.google.mlkit.vision.codescanner.GmsBarcodeScannerOptions
import com.google.mlkit.vision.codescanner.GmsBarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.termux.view.TerminalView
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private val SharpShape = RoundedCornerShape(0.dp)
private const val FORGET_HOLD_DURATION_MS = 10_000L
private const val FORGET_HOLD_TICK_MS = 50L
private val TerminalToolbarHeight = 56.dp
class MainActivity : ComponentActivity() {

    private val viewModel: MainViewModel by viewModels()

    private val scanner: GmsBarcodeScanner by lazy {
        val options = GmsBarcodeScannerOptions.Builder()
            .setBarcodeFormats(Barcode.FORMAT_QR_CODE)
            .enableAutoZoom()
            .build()
        GmsBarcodeScanning.getClient(this, options)
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
        super.onCreate(savedInstanceState)
        Log.d(TAG, "onCreate savedInstanceState=${savedInstanceState != null}")
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
                        onScanClick = ::launchScanner,
                        onAutoConnect = { config ->
                            viewModel.consumePendingAutoConnect()
                            startConnection(config)
                        },
                        onTerminalClick = {
                            viewModel.openTerminal()
                            viewModel.uiState.config?.let { config ->
                                if (!viewModel.uiState.sessionState.isConnecting && !viewModel.uiState.sessionState.isConnected) {
                                    startConnection(config)
                                }
                            }
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

    override fun onStart() {
        super.onStart()
        Log.d(TAG, "onStart")
    }

    override fun onResume() {
        super.onResume()
        Log.d(TAG, "onResume")
    }

    override fun onPause() {
        Log.d(TAG, "onPause")
        super.onPause()
    }

    override fun onStop() {
        Log.d(TAG, "onStop")
        super.onStop()
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        Log.d(TAG, "onWindowFocusChanged hasFocus=$hasFocus")
        if (hasFocus) {
            enableImmersiveMode()
        }
    }

    override fun onDestroy() {
        Log.d(TAG, "onDestroy finishing=$isFinishing")
        super.onDestroy()
    }

    private fun launchScanner() {
        Log.d(TAG, "launchScanner")
        viewModel.onScanStarted()

        scanner.startScan()
            .addOnSuccessListener { barcode ->
                Log.d(TAG, "scanner success format=${barcode.format} rawLength=${barcode.rawValue?.length ?: 0}")
                val payload = barcode.rawValue.orEmpty()
                val parseResult = SshQrParser.parse(payload)
                parseResult.onSuccess { config ->
                    viewModel.onQrParsed(config)
                    startConnection(config)
                }.onFailure { failure ->
                    viewModel.onQrRejected(failure.message ?: "QR payload is invalid.")
                }
            }
            .addOnCanceledListener {
                viewModel.onScanCancelled()
            }
            .addOnFailureListener { error ->
                viewModel.onScannerError(error.message ?: "Unable to read QR code.")
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
            systemBarsBehavior =
                WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        }
    }
}

data class MainUiState(
    val status: String = "Scan one QR code with host, user and private key.",
    val screen: MainScreenRoute = MainScreenRoute.Scan,
    val config: SshQrConfig? = null,
    val sessionState: SessionConnectionState = SessionConnectionState.IDLE,
    val error: String? = null,
    val vpnTunnelName: String? = null,
    val pendingAutoConnect: Boolean = false,
    val files: FilesUiState = FilesUiState(),
) {
    val isConnecting: Boolean
        get() = sessionState.isConnecting

    val isConnected: Boolean
        get() = sessionState.isConnected
}

enum class MainScreenRoute {
    Scan,
    Menu,
    Files,
    Settings,
    Session,
}

@Composable
private fun MainScreen(
    state: MainUiState,
    terminalController: SshTerminalController,
    onScanClick: () -> Unit,
    onAutoConnect: (SshQrConfig) -> Unit,
    onTerminalClick: () -> Unit,
    onFilesClick: () -> Unit,
    onRefreshFiles: () -> Unit,
    onNavigateFilesUp: () -> Unit,
    onOpenDirectory: (RemoteFileEntry) -> Unit,
    onCreateDirectory: (String) -> Unit,
    onCreateFile: (String) -> Unit,
    onRenameFile: (RemoteFileEntry, String) -> Unit,
    onDeleteFile: (RemoteFileEntry) -> Unit,
    onUploadFile: (Uri, String) -> Unit,
    onDownloadFile: (RemoteFileEntry, Uri) -> Unit,
    onSettingsClick: () -> Unit,
    onForgetSavedConfig: () -> Unit,
    onBackToMenu: () -> Unit,
) {
    val context = LocalContext.current
    var pendingDownload by remember { mutableStateOf<RemoteFileEntry?>(null) }
    val uploadLauncher = rememberLauncherForActivityResult(ActivityResultContracts.GetContent()) { uri ->
        if (uri == null) {
            return@rememberLauncherForActivityResult
        }
        val displayName = rememberDisplayName(context.contentResolver, uri)
        if (!displayName.isNullOrBlank()) {
            onUploadFile(uri, displayName)
        }
    }
    val downloadLauncher = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("*/*")) { uri ->
        val entry = pendingDownload
        pendingDownload = null
        if (uri != null && entry != null) {
            onDownloadFile(entry, uri)
        }
    }

    LaunchedEffect(state.pendingAutoConnect, state.config) {
        val config = state.config
        if (state.pendingAutoConnect && config != null) {
            onAutoConnect(config)
        }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black)
    ) {
        when (state.screen) {
            MainScreenRoute.Scan -> {
                ScanView(
                    state = state,
                    onScanClick = onScanClick,
                )
            }

            MainScreenRoute.Menu -> {
                MenuView(
                    state = state,
                    onTerminalClick = onTerminalClick,
                    onFilesClick = onFilesClick,
                    onSettingsClick = onSettingsClick,
                )
            }

            MainScreenRoute.Files -> {
                FilesView(
                    state = state.files,
                    onBackToMenu = onBackToMenu,
                    onRefresh = onRefreshFiles,
                    onNavigateUp = onNavigateFilesUp,
                    onOpenDirectory = onOpenDirectory,
                    onCreateDirectory = onCreateDirectory,
                    onCreateFile = onCreateFile,
                    onRename = onRenameFile,
                    onDelete = onDeleteFile,
                    onUploadClick = {
                        uploadLauncher.launch("*/*")
                    },
                    onDownloadClick = { entry ->
                        pendingDownload = entry
                        downloadLauncher.launch(entry.name)
                    },
                )
            }

            MainScreenRoute.Settings -> {
                SettingsView(
                    onForgetSavedConfig = onForgetSavedConfig,
                    onBackToMenu = onBackToMenu,
                )
            }

            MainScreenRoute.Session -> {
                SessionView(
                    terminalController = terminalController,
                    onBackToMenu = onBackToMenu,
                )
            }
        }
    }
}

@Composable
private fun ScanView(
    state: MainUiState,
    onScanClick: () -> Unit,
) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black)
            .padding(20.dp),
    ) {
        state.error?.let {
            Text(
                text = it,
                style = MaterialTheme.typography.bodyMedium,
                fontFamily = ArcTerminalFontFamily,
                color = MaterialTheme.colorScheme.error,
                modifier = Modifier.align(Alignment.TopStart),
            )
        }

        Column(
            modifier = Modifier.align(Alignment.Center),
            verticalArrangement = Arrangement.spacedBy(16.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                text = "ARC",
                style = MaterialTheme.typography.headlineLarge,
                fontFamily = ArcTerminalFontFamily,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onSurface,
            )
            Text(
                text = "Scan QR to connect to the ARC server.",
                modifier = Modifier.fillMaxWidth(),
                style = MaterialTheme.typography.bodyLarge,
                fontFamily = ArcTerminalFontFamily,
                color = MaterialTheme.colorScheme.onSurface,
                textAlign = TextAlign.Center,
            )
            Button(
                onClick = onScanClick,
                shape = SharpShape,
            ) {
                Icon(
                    imageVector = Icons.Outlined.QrCodeScanner,
                    contentDescription = null,
                    modifier = Modifier.padding(end = 8.dp),
                )
                Text("Scan QR", fontFamily = ArcTerminalFontFamily)
            }
        }
    }
}

@Composable
private fun MenuView(
    state: MainUiState,
    onTerminalClick: () -> Unit,
    onFilesClick: () -> Unit,
    onSettingsClick: () -> Unit,
) {
    state.config ?: return
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black)
            .padding(20.dp),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth(),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 2.dp, vertical = 4.dp),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = "ARC",
                    style = MaterialTheme.typography.headlineMedium,
                    fontFamily = ArcTerminalFontFamily,
                    fontWeight = FontWeight.Bold,
                    color = Color(0xFFD8DADF),
                )
                Text(
                    text = buildString {
                        append("@")
                        append(state.config.host)
                        state.vpnTunnelName?.let {
                            append(" / wireguard")
                        }
                    },
                    style = MaterialTheme.typography.bodySmall,
                    fontFamily = ArcTerminalFontFamily,
                    fontWeight = FontWeight.Medium,
                    color = Color(0xFF9EA4AF),
                )
            }

            MenuActionRow(
                title = "Terminal",
                icon = {
                    Icon(
                        imageVector = Icons.Outlined.Terminal,
                        contentDescription = null,
                        tint = Color(0xFFD8DADF),
                        modifier = Modifier.size(20.dp),
                    )
                },
                enabled = true,
                onClick = onTerminalClick,
            )

            MenuActionRow(
                title = "Files",
                icon = {
                    Icon(
                        imageVector = Icons.Outlined.Description,
                        contentDescription = null,
                        tint = Color(0xFFD8DADF),
                        modifier = Modifier.size(20.dp),
                    )
                },
                enabled = true,
                onClick = onFilesClick,
            )

            MenuActionRow(
                title = "Settings",
                icon = {
                    Icon(
                        imageVector = Icons.Outlined.Settings,
                        contentDescription = null,
                        tint = Color(0xFFD8DADF),
                        modifier = Modifier.size(20.dp),
                    )
                },
                enabled = true,
                onClick = onSettingsClick,
            )
        }
    }
}

@Composable
private fun MenuActionRow(
    title: String,
    icon: @Composable () -> Unit,
    enabled: Boolean,
    onClick: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .border(1.dp, Color(0xFF15171C), SharpShape)
            .background(Color(0xFF16191F))
            .clickable(enabled = enabled, onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 12.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Row(
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            icon()
            Text(
                text = "//",
                style = MaterialTheme.typography.titleMedium,
                fontFamily = ArcTerminalFontFamily,
                fontWeight = FontWeight.Bold,
                color = ArcTerminalAccent,
            )
            Text(
                text = title,
                style = MaterialTheme.typography.titleLarge,
                fontFamily = ArcTerminalFontFamily,
                fontWeight = FontWeight.Bold,
                color = Color(0xFFD8DADF),
            )
        }
    }
}

@Composable
private fun SettingsView(
    onForgetSavedConfig: () -> Unit,
    onBackToMenu: () -> Unit,
) {
    BackHandler(onBack = onBackToMenu)
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black)
            .padding(20.dp),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            modifier = Modifier.fillMaxWidth(),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(
                text = "Settings",
                style = MaterialTheme.typography.headlineMedium,
                fontFamily = ArcTerminalFontFamily,
                fontWeight = FontWeight.Bold,
                color = Color(0xFFD8DADF),
                modifier = Modifier.padding(horizontal = 2.dp, vertical = 4.dp),
            )

            MenuActionRow(
                title = "Back",
                icon = {
                    Icon(
                        imageVector = Icons.Outlined.ArrowBack,
                        contentDescription = null,
                        tint = Color(0xFFD8DADF),
                        modifier = Modifier.size(20.dp),
                    )
                },
                enabled = true,
                onClick = onBackToMenu,
            )

            HoldToForgetRow(onForgetSavedConfig = onForgetSavedConfig)
        }
    }
}

@Composable
private fun HoldToForgetRow(
    onForgetSavedConfig: () -> Unit,
) {
    var holdProgress by remember { mutableFloatStateOf(0f) }
    var remainingMs by remember { mutableLongStateOf(FORGET_HOLD_DURATION_MS) }

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .border(1.dp, Color(0xFF15171C), SharpShape)
            .background(Color(0xFF16191F))
            .pointerInput(onForgetSavedConfig) {
                detectTapGestures(
                    onPress = {
                        coroutineScope {
                            val startAt = SystemClock.elapsedRealtime()
                            var completed = false
                            holdProgress = 0f
                            remainingMs = FORGET_HOLD_DURATION_MS

                            val progressJob = launch {
                                while (true) {
                                    val elapsed = SystemClock.elapsedRealtime() - startAt
                                    holdProgress = (elapsed.toFloat() / FORGET_HOLD_DURATION_MS.toFloat())
                                        .coerceIn(0f, 1f)
                                    remainingMs = (FORGET_HOLD_DURATION_MS - elapsed).coerceAtLeast(0L)
                                    if (elapsed >= FORGET_HOLD_DURATION_MS) {
                                        completed = true
                                        onForgetSavedConfig()
                                        break
                                    }
                                    delay(FORGET_HOLD_TICK_MS)
                                }
                            }

                            tryAwaitRelease()
                            progressJob.cancel()

                            if (!completed) {
                                holdProgress = 0f
                                remainingMs = FORGET_HOLD_DURATION_MS
                            }
                        }
                    },
                )
            }
            .padding(horizontal = 14.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Row(
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                imageVector = Icons.Outlined.Delete,
                contentDescription = null,
                tint = Color(0xFFD8DADF),
                modifier = Modifier.size(20.dp),
            )
            Text(
                text = "//",
                style = MaterialTheme.typography.titleMedium,
                fontFamily = ArcTerminalFontFamily,
                fontWeight = FontWeight.Bold,
                color = ArcTerminalAccent,
            )
            Column(
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                Text(
                    text = "Forget saved config",
                    style = MaterialTheme.typography.titleLarge,
                    fontFamily = ArcTerminalFontFamily,
                    fontWeight = FontWeight.Bold,
                    color = Color(0xFFD8DADF),
                )
                Text(
                    text = if (holdProgress > 0f) {
                        "Keep holding ${formatHoldRemaining(remainingMs)}"
                    } else {
                        "Hold 10s to erase."
                    },
                    style = MaterialTheme.typography.bodySmall,
                    fontFamily = ArcTerminalFontFamily,
                    color = if (holdProgress > 0f) ArcTerminalAccent else Color(0xFF9EA4AF),
                )
            }
        }
    }
}

@Composable
private fun SessionView(
    terminalController: SshTerminalController,
    onBackToMenu: () -> Unit,
) {
    BackHandler(onBack = onBackToMenu)
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black)
            .imePadding()
            .navigationBarsPadding(),
    ) {
        TerminalViewport(
            controller = terminalController,
            modifier = Modifier
                .fillMaxSize()
                .padding(bottom = TerminalToolbarHeight),
        )
        TerminalToolbar(
            controller = terminalController,
            modifier = Modifier.align(Alignment.BottomCenter),
        )
    }
}

@Composable
private fun TerminalViewport(
    controller: SshTerminalController,
    modifier: Modifier = Modifier,
) {
    AndroidView(
        factory = { context ->
            TerminalView(context, null).apply {
                addOnLayoutChangeListener { _, left, top, right, bottom, oldLeft, oldTop, oldRight, oldBottom ->
                    if ((right - left) != (oldRight - oldLeft) || (bottom - top) != (oldBottom - oldTop)) {
                        controller.onTerminalViewDidResize(this)
                    }
                }
                initializeRenderer(controller.terminalFontSizeSp())
                controller.bind(this)
                focusAndShowKeyboard()
            }
        },
        update = {
            controller.bind(it)
        },
        modifier = modifier,
    )
}

@Composable
private fun TerminalToolbar(
    controller: SshTerminalController,
    modifier: Modifier = Modifier,
) {
    val scrollState = rememberScrollState()
    val toolbarState = controller.toolbarState
    Row(
        modifier = modifier
            .fillMaxWidth()
            .border(1.dp, Color(0xFF15171C), SharpShape)
            .background(Color(0xFF0E1115))
            .horizontalScroll(scrollState)
            .padding(horizontal = 8.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        TerminalToolbarButton(
            label = "Ctrl",
            active = toolbarState.ctrl,
            onClick = { controller.sendToolbarKey(SshTerminalController.ToolbarKey.CTRL) },
        )
        TerminalToolbarButton(
            label = "Alt",
            active = toolbarState.alt,
            onClick = { controller.sendToolbarKey(SshTerminalController.ToolbarKey.ALT) },
        )
        TerminalToolbarButton(
            label = "Esc",
            onClick = { controller.sendToolbarKey(SshTerminalController.ToolbarKey.ESC) },
        )
        TerminalToolbarButton(
            label = "Tab",
            onClick = { controller.sendToolbarKey(SshTerminalController.ToolbarKey.TAB) },
        )
        TerminalToolbarButton(
            label = "Home",
            onClick = { controller.sendToolbarKey(SshTerminalController.ToolbarKey.HOME) },
        )
        TerminalToolbarButton(
            label = "End",
            onClick = { controller.sendToolbarKey(SshTerminalController.ToolbarKey.END) },
        )
        TerminalToolbarButton(
            label = "PgUp",
            onClick = { controller.sendToolbarKey(SshTerminalController.ToolbarKey.PAGE_UP) },
        )
        TerminalToolbarButton(
            label = "PgDn",
            onClick = { controller.sendToolbarKey(SshTerminalController.ToolbarKey.PAGE_DOWN) },
        )
    }
}

@Composable
private fun TerminalToolbarButton(
    label: String,
    active: Boolean = false,
    onClick: () -> Unit,
) {
    Row(
        modifier = Modifier
            .border(1.dp, if (active) ArcTerminalAccent else Color(0xFF20252D), SharpShape)
            .background(if (active) Color(0xFF1A2118) else Color(0xFF15191F))
            .clickable(onClick = onClick)
            .padding(horizontal = 10.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            fontFamily = ArcTerminalFontFamily,
            fontWeight = FontWeight.Medium,
            color = if (active) ArcTerminalAccent else Color(0xFFD8DADF),
        )
    }
}

private fun TerminalView.initializeRenderer(fontSizeSp: Float) {
    val textSizePx = TypedValue.applyDimension(
        TypedValue.COMPLEX_UNIT_SP,
        fontSizeSp,
        resources.displayMetrics,
    ).toInt()
    setTextSize(textSizePx)
    ResourcesCompat.getFont(context, R.font.jetbrainsmono_nerd_font_regular)?.let(::setTypeface)
}

private fun TerminalView.focusAndShowKeyboard() {
    isFocusable = true
    isFocusableInTouchMode = true
    requestFocus()
    post {
        requestFocus()
        context.getSystemService<InputMethodManager>()?.showSoftInput(this, InputMethodManager.SHOW_IMPLICIT)
    }
}

private fun formatHoldRemaining(remainingMs: Long): String {
    val seconds = (remainingMs + 999L) / 1000L
    return "${seconds}s"
}

private fun rememberDisplayName(contentResolver: ContentResolver, uri: Uri): String? {
    contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { cursor ->
        if (cursor.moveToFirst()) {
            val columnIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
            if (columnIndex >= 0) {
                return cursor.getString(columnIndex)
            }
        }
    }
    return uri.lastPathSegment?.substringAfterLast('/')
}

private const val TAG = "MainActivity"
