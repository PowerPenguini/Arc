package com.arc.sshqr

import android.content.ContentResolver
import android.net.Uri
import android.os.SystemClock
import android.provider.OpenableColumns
import android.util.TypedValue
import android.view.inputmethod.InputMethodManager
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
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
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.draw.clip
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material.icons.outlined.QrCodeScanner
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.Terminal
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.getSystemService
import androidx.core.content.res.ResourcesCompat
import com.arc.sshqr.files.RemoteFileEntry
import com.arc.sshqr.qr.SshQrConfig
import com.arc.sshqr.ssh.SshTerminalController
import com.arc.sshqr.ui.theme.ArcTerminalAccent
import com.arc.sshqr.ui.theme.ArcTerminalFontFamily
import com.termux.view.TerminalView
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private val SharpShape = RoundedCornerShape(0.dp)
private const val FORGET_HOLD_DURATION_MS = 10_000L
private const val FORGET_HOLD_TICK_MS = 50L
private val TerminalToolbarHeight = 56.dp
private val TerminalToolbarBorder = Color(0xFF202628)
private val TerminalToolbarBackground = Color(0xFF101313)
private val TerminalToolbarButtonBorder = Color(0xFF2A3134)
private val TerminalToolbarButtonBackground = Color(0xFF171C1E)
private val TerminalToolbarButtonText = Color(0xFFD8DADF)
private val TerminalToolbarButtonActiveBackground = Color(0xFF222A18)

internal fun buildMenuHeaderDetails(
    host: String,
    vpnTunnelName: String?,
): String {
    if (!hasActiveTunnel(vpnTunnelName)) {
        return "//DISCONNECTED"
    }
    return "@$host / wireguard"
}

internal fun hasActiveTunnel(
    vpnTunnelName: String?,
): Boolean = vpnTunnelName != null

internal fun shouldShowReconnectAction(
    vpnTunnelName: String?,
    sessionState: SessionConnectionState,
): Boolean = !hasActiveTunnel(vpnTunnelName) && !sessionState.isConnecting

@Composable
fun MainScreen(
    state: MainUiState,
    terminalController: SshTerminalController,
    onScanStarted: () -> Unit,
    onScanCancelled: () -> Unit,
    onScanError: (String) -> Unit,
    onQrPayloadScanned: (String) -> Unit,
    onAutoConnect: (SshQrConfig) -> Unit,
    onTerminalClick: () -> Unit,
    onFilesClick: () -> Unit,
    onReconnectClick: () -> Unit,
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
            .background(Color.Black),
    ) {
        when (state.screen) {
            MainScreenRoute.Scan -> ScanView(
                state = state,
                onScanStarted = onScanStarted,
                onScanCancelled = onScanCancelled,
                onScanError = onScanError,
                onQrPayloadScanned = onQrPayloadScanned,
            )
            MainScreenRoute.Menu -> MenuView(
                state = state,
                onTerminalClick = onTerminalClick,
                onFilesClick = onFilesClick,
                onReconnectClick = onReconnectClick,
                onSettingsClick = onSettingsClick,
            )
            MainScreenRoute.Files -> FilesView(
                state = state.files,
                onBackToMenu = onBackToMenu,
                onRefresh = onRefreshFiles,
                onNavigateUp = onNavigateFilesUp,
                onOpenDirectory = onOpenDirectory,
                onCreateDirectory = onCreateDirectory,
                onCreateFile = onCreateFile,
                onRename = onRenameFile,
                onDelete = onDeleteFile,
                onUploadClick = { uploadLauncher.launch("*/*") },
                onDownloadClick = { entry ->
                    pendingDownload = entry
                    downloadLauncher.launch(entry.name)
                },
            )
            MainScreenRoute.Settings -> SettingsView(
                onForgetSavedConfig = onForgetSavedConfig,
                onBackToMenu = onBackToMenu,
            )
            MainScreenRoute.Session -> SessionView(
                terminalController = terminalController,
                onBackToMenu = onBackToMenu,
            )
        }
    }
}

@Composable
private fun ScanView(
    state: MainUiState,
    onScanStarted: () -> Unit,
    onScanCancelled: () -> Unit,
    onScanError: (String) -> Unit,
    onQrPayloadScanned: (String) -> Unit,
) {
    var scannerActive by rememberSaveable { mutableStateOf(false) }
    var pasteDialogVisible by rememberSaveable { mutableStateOf(false) }
    var manualPayload by rememberSaveable { mutableStateOf("") }

    BackHandler(enabled = scannerActive || pasteDialogVisible) {
        when {
            pasteDialogVisible -> pasteDialogVisible = false
            scannerActive -> {
                scannerActive = false
                onScanCancelled()
            }
        }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black)
            .navigationBarsPadding()
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

        if (scannerActive) {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(top = 28.dp),
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
                Spacer(modifier = Modifier.height(12.dp))
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(420.dp)
                        .background(Color(0xFF0B0F10))
                        .clip(SharpShape)
                        .border(1.dp, Color(0xFF1A2326)),
                ) {
                    QrScannerView(
                        active = scannerActive,
                        onCameraAccessDenied = {
                            scannerActive = false
                            onScanError("Camera permission denied.")
                        },
                        onPayloadScanned = { payload ->
                            scannerActive = false
                            onQrPayloadScanned(payload)
                        },
                        onScannerError = { message ->
                            scannerActive = false
                            onScanError(message)
                        },
                        modifier = Modifier.fillMaxSize(),
                    )
                }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.Center,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Button(
                        onClick = {
                            scannerActive = false
                            onScanCancelled()
                        },
                        shape = SharpShape,
                    ) {
                        Text("Stop scan", fontFamily = ArcTerminalFontFamily)
                    }
                }
                Button(
                    onClick = {
                        scannerActive = false
                        pasteDialogVisible = true
                    },
                    shape = SharpShape,
                ) {
                    Text("Paste code", fontFamily = ArcTerminalFontFamily)
                }
            }
        } else {
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
                    onClick = {
                        onScanStarted()
                        scannerActive = true
                    },
                    shape = SharpShape,
                ) {
                    Icon(
                        imageVector = Icons.Outlined.QrCodeScanner,
                        contentDescription = null,
                        modifier = Modifier.padding(end = 8.dp),
                    )
                    Text("Scan QR", fontFamily = ArcTerminalFontFamily)
                }
                Button(
                    onClick = {
                        onScanStarted()
                        pasteDialogVisible = true
                    },
                    shape = SharpShape,
                ) {
                    Text("Paste QR payload", fontFamily = ArcTerminalFontFamily)
                }
            }
        }
    }

    if (pasteDialogVisible) {
        AlertDialog(
            onDismissRequest = { pasteDialogVisible = false },
            containerColor = Color(0xFF16191F),
            title = {
                Text(
                    text = "Paste QR payload",
                    fontFamily = ArcTerminalFontFamily,
                    color = Color(0xFFD8DADF),
                )
            },
            text = {
                OutlinedTextField(
                    value = manualPayload,
                    onValueChange = { manualPayload = it },
                    minLines = 6,
                    textStyle = MaterialTheme.typography.bodyMedium.copy(
                        fontFamily = ArcTerminalFontFamily,
                        color = Color(0xFFD8DADF),
                    ),
                    label = {
                        Text("JSON payload", fontFamily = ArcTerminalFontFamily)
                    },
                )
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        pasteDialogVisible = false
                        onQrPayloadScanned(manualPayload)
                        manualPayload = ""
                    },
                    enabled = manualPayload.isNotBlank(),
                ) {
                    Text("Connect", fontFamily = ArcTerminalFontFamily, color = ArcTerminalAccent)
                }
            },
            dismissButton = {
                TextButton(onClick = { pasteDialogVisible = false }) {
                    Text("Cancel", fontFamily = ArcTerminalFontFamily, color = Color(0xFFD8DADF))
                }
            },
        )
    }
}

@Composable
private fun MenuView(
    state: MainUiState,
    onTerminalClick: () -> Unit,
    onFilesClick: () -> Unit,
    onReconnectClick: () -> Unit,
    onSettingsClick: () -> Unit,
) {
    state.config ?: return
    val headerDetails = buildMenuHeaderDetails(
        host = state.config.host,
        vpnTunnelName = state.vpnTunnelName,
    )
    val isConnectionAvailable = hasActiveTunnel(
        vpnTunnelName = state.vpnTunnelName,
    )
    val showReconnectAction = shouldShowReconnectAction(
        vpnTunnelName = state.vpnTunnelName,
        sessionState = state.sessionState,
    )
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
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 2.dp, vertical = 4.dp),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Row(
                    modifier = Modifier.weight(1f),
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
                        text = headerDetails,
                        style = MaterialTheme.typography.bodySmall,
                        fontFamily = ArcTerminalFontFamily,
                        fontWeight = FontWeight.Medium,
                        color = Color(0xFF9EA4AF),
                    )
                }
                if (showReconnectAction) {
                    Button(
                        onClick = onReconnectClick,
                        shape = SharpShape,
                        modifier = Modifier.height(30.dp),
                        colors = ButtonDefaults.buttonColors(
                            containerColor = Color(0xFF171C1E),
                            contentColor = Color(0xFFD8DADF),
                        ),
                        contentPadding = androidx.compose.foundation.layout.PaddingValues(
                            horizontal = 8.dp,
                            vertical = 4.dp,
                        ),
                    ) {
                        Icon(
                            imageVector = Icons.Outlined.Refresh,
                            contentDescription = null,
                            modifier = Modifier
                                .size(12.dp)
                                .padding(end = 4.dp),
                        )
                        Text(
                            text = "Reconnect",
                            fontFamily = ArcTerminalFontFamily,
                            style = MaterialTheme.typography.labelSmall,
                        )
                    }
                }
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
                enabled = isConnectionAvailable,
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
                enabled = isConnectionAvailable,
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
    val rowBorder = if (enabled) Color(0xFF15171C) else Color(0xFF101215)
    val rowBackground = if (enabled) Color(0xFF16191F) else Color(0xFF0D1014)
    val contentTint = if (enabled) Color(0xFFD8DADF) else Color(0xFF5F666F)
    val accentTint = if (enabled) ArcTerminalAccent else Color(0xFF4A524F)
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .border(1.dp, rowBorder, SharpShape)
            .background(rowBackground)
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
                color = accentTint,
            )
            Text(
                text = title,
                style = MaterialTheme.typography.titleLarge,
                fontFamily = ArcTerminalFontFamily,
                fontWeight = FontWeight.Bold,
                color = contentTint,
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
                        imageVector = Icons.AutoMirrored.Outlined.ArrowBack,
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
private fun HoldToForgetRow(onForgetSavedConfig: () -> Unit) {
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
                                    holdProgress = (elapsed.toFloat() / FORGET_HOLD_DURATION_MS.toFloat()).coerceIn(0f, 1f)
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
            Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
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
            .background(TerminalToolbarBackground)
            .imePadding()
            .navigationBarsPadding(),
    ) {
        TerminalViewport(
            controller = terminalController,
            modifier = Modifier
                .fillMaxSize()
                .background(TerminalToolbarBackground)
                .padding(5.dp)
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
        update = { controller.bind(it) },
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
            .border(1.dp, TerminalToolbarBorder, SharpShape)
            .background(TerminalToolbarBackground)
            .horizontalScroll(scrollState)
            .padding(horizontal = 8.dp, vertical = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        TerminalToolbarButton(label = "Ctrl", active = toolbarState.ctrl) {
            controller.sendToolbarKey(SshTerminalController.ToolbarKey.CTRL)
        }
        TerminalToolbarButton(label = "Alt", active = toolbarState.alt) {
            controller.sendToolbarKey(SshTerminalController.ToolbarKey.ALT)
        }
        TerminalToolbarButton(label = "Esc") {
            controller.sendToolbarKey(SshTerminalController.ToolbarKey.ESC)
        }
        TerminalToolbarButton(label = "Tab") {
            controller.sendToolbarKey(SshTerminalController.ToolbarKey.TAB)
        }
        TerminalToolbarButton(label = "Del") {
            controller.sendToolbarKey(SshTerminalController.ToolbarKey.DELETE)
        }
        TerminalToolbarButton(label = "Home") {
            controller.sendToolbarKey(SshTerminalController.ToolbarKey.HOME)
        }
        TerminalToolbarButton(label = "End") {
            controller.sendToolbarKey(SshTerminalController.ToolbarKey.END)
        }
        TerminalToolbarButton(label = "PgUp") {
            controller.sendToolbarKey(SshTerminalController.ToolbarKey.PAGE_UP)
        }
        TerminalToolbarButton(label = "PgDn") {
            controller.sendToolbarKey(SshTerminalController.ToolbarKey.PAGE_DOWN)
        }
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
            .border(1.dp, if (active) ArcTerminalAccent else TerminalToolbarButtonBorder, SharpShape)
            .background(if (active) TerminalToolbarButtonActiveBackground else TerminalToolbarButtonBackground)
            .clickable(onClick = onClick)
            .padding(horizontal = 10.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            fontFamily = ArcTerminalFontFamily,
            fontWeight = FontWeight.Medium,
            color = if (active) ArcTerminalAccent else TerminalToolbarButtonText,
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
