package com.arc.sshqr.ssh

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.pm.ApplicationInfo
import android.util.Log
import android.util.TypedValue
import android.view.KeyEvent
import android.view.MotionEvent
import android.view.View
import android.view.inputmethod.InputMethodManager
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.core.content.getSystemService
import com.arc.sshqr.SessionConnectionState
import com.arc.sshqr.qr.SshQrConfig
import com.termux.terminal.KeyHandler
import com.termux.terminal.TerminalOutput
import com.termux.terminal.TerminalSession
import com.termux.terminal.TerminalSessionClient
import com.termux.view.TerminalView
import com.termux.view.TerminalViewClient
import java.io.Closeable
import java.io.EOFException
import java.io.IOException
import java.io.InterruptedIOException
import java.io.OutputStream
import java.nio.charset.StandardCharsets
import java.util.concurrent.atomic.AtomicBoolean
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import net.schmizz.sshj.connection.channel.direct.Session
import net.schmizz.sshj.connection.channel.direct.SessionChannel
import net.schmizz.sshj.transport.TransportException
import net.schmizz.sshj.userauth.UserAuthException

class SshTerminalController(
    private val appContext: Context,
    private val scope: CoroutineScope,
    private val connectionManager: SshConnectionManager,
    private val callbacks: Callbacks,
    private val prepareReconnectTransport: suspend (SshQrConfig) -> Unit = {},
    private val diagnoseTransport: suspend (SshQrConfig?) -> String = { "transport diagnostics unavailable" },
) : Closeable {

    enum class ToolbarKey {
        CTRL,
        ALT,
        ESC,
        TAB,
        UP,
        DOWN,
        LEFT,
        RIGHT,
        HOME,
        END,
        PAGE_UP,
        PAGE_DOWN,
        PIPE,
        SLASH,
        TILDE,
        BACKTICK,
    }

    data class ToolbarState(
        val ctrl: Boolean = false,
        val alt: Boolean = false,
    )

    interface Callbacks {
        fun onStatusChanged(status: String)
        fun onSessionStateChanged(state: SessionConnectionState)
        fun onConnected(config: SshQrConfig)
        fun onDisconnected(message: String)
        fun onConnectionFailed(message: String)
    }

    private val ioMutex = Mutex()
    private val preferences = appContext.getSharedPreferences(TERMINAL_PREFS_NAME, Context.MODE_PRIVATE)
    private val terminalClient = ControllerTerminalViewClient()
    private val sessionClient = ControllerTerminalSessionClient()
    private val connected = AtomicBoolean(false)
    private val manualDisconnectRequested = AtomicBoolean(false)
    private val disconnectHandled = AtomicBoolean(true)
    var toolbarState by mutableStateOf(ToolbarState())
        private set

    private var terminalView: TerminalView? = null
    private var terminalSession: TerminalSession? = null
    private var activeConfig: SshQrConfig? = null
    private var terminalOutputProxyInstalled = false

    private var sshSession: Session? = null
    private var sshShell: Session.Command? = null
    private var remoteInput: OutputStream? = null
    private var readJob: Job? = null
    private var pendingConnectJob: Job? = null
    private var reconnectJob: Job? = null
    private var repeatArrowJob: Job? = null
    private val remoteWriteQueue = Channel<ByteArray>(Channel.UNLIMITED)
    private val remoteWriterJob: Job
    private var lastColumns: Int = 80
    private var lastRows: Int = 24
    private var terminalFontSizeSp: Float = loadSavedTerminalFontSize()
    private var viewportState = TerminalViewportState()
    private var remoteColumns: Int = 0
    private var remoteRows: Int = 0
    private var currentState: SessionConnectionState = SessionConnectionState.IDLE
    private var reconnectAttempt = 0
    private var remoteEscapeLogTail = ""
    @Volatile
    private var transportDisconnectMessage: String? = null
    private val debugTouchLogs = (appContext.applicationInfo.flags and ApplicationInfo.FLAG_DEBUGGABLE) != 0

    init {
        remoteWriterJob = scope.launch(Dispatchers.IO) {
            processRemoteWrites()
        }
    }

    fun bind(view: TerminalView) {
        val isNewView = terminalView !== view
        terminalView = view
        if (isNewView) {
            view.keepScreenOn = true
            view.setBackgroundColor(TERMINAL_BACKGROUND_COLOR)
            view.setTerminalViewClient(terminalClient)
            view.setOnTouchListener(TerminalTouchInterceptor(view))
        }
        ensureTerminalSession()
        syncTerminalBackgroundPalette()

        view.attachSession(checkNotNull(terminalSession))
        installTerminalOutputProxy()
        restoreViewportAfterScreenUpdate(view)
        if (
            activeConfig != null &&
            pendingConnectJob == null &&
            reconnectJob == null &&
            !connected.get() &&
            (currentState == SessionConnectionState.CONNECTING || currentState == SessionConnectionState.RECONNECTING)
        ) {
            connect(activeConfig!!)
        }
    }

    fun connect(config: SshQrConfig) {
        activeConfig = config
        manualDisconnectRequested.set(false)
        reconnectAttempt = 0
        reconnectJob?.cancel()
        ensureTerminalSession()
        pendingConnectJob?.cancel()
        pendingConnectJob = scope.launch(Dispatchers.IO) {
            try {
                disconnectHandled.set(true)
                cleanupConnection(cancelReadJob = true)
                appendStatusLine("Scanning complete. Opening SSH session...")
                updateStatus(
                    SessionConnectionState.CONNECTING,
                    "Connecting to ${config.username}@${config.host}:${config.port}",
                )
                establishConnection(config, isReconnect = false)
            } finally {
                pendingConnectJob = null
            }
        }
    }

    fun sendToolbarKey(key: ToolbarKey) {
        dispatchToolbarKey(key, focusAfterSend = true)
    }

    fun beginToolbarKeyHold(key: ToolbarKey) {
        dispatchToolbarKey(key, focusAfterSend = false)
        if (!isRepeatableNavigationKey(key) || !connected.get()) {
            return
        }
        repeatArrowJob?.cancel()
        repeatArrowJob = scope.launch(Dispatchers.IO) {
            delay(TOOLBAR_REPEAT_INITIAL_DELAY_MS)
            while (connected.get()) {
                dispatchToolbarKey(key, focusAfterSend = false)
                delay(TOOLBAR_REPEAT_INTERVAL_MS)
            }
        }
    }

    fun endToolbarKeyHold() {
        stopRepeatingArrowKey()
    }

    private fun dispatchToolbarKey(key: ToolbarKey, focusAfterSend: Boolean) {
        when (key) {
            ToolbarKey.CTRL -> {
                toolbarState = toolbarState.copy(ctrl = !toolbarState.ctrl)
                if (focusAfterSend) {
                    focusTerminalInput(showKeyboard = false)
                }
            }

            ToolbarKey.ALT -> {
                toolbarState = toolbarState.copy(alt = !toolbarState.alt)
                if (focusAfterSend) {
                    focusTerminalInput(showKeyboard = false)
                }
            }

            ToolbarKey.ESC -> sendSpecialSequence("\u001B", focusAfterSend)
            ToolbarKey.TAB -> sendSpecialSequence("\t", focusAfterSend)
            ToolbarKey.UP -> sendSpecialKeyCode(KeyEvent.KEYCODE_DPAD_UP, focusAfterSend)
            ToolbarKey.DOWN -> sendSpecialKeyCode(KeyEvent.KEYCODE_DPAD_DOWN, focusAfterSend)
            ToolbarKey.LEFT -> sendSpecialKeyCode(KeyEvent.KEYCODE_DPAD_LEFT, focusAfterSend)
            ToolbarKey.RIGHT -> sendSpecialKeyCode(KeyEvent.KEYCODE_DPAD_RIGHT, focusAfterSend)
            ToolbarKey.HOME -> sendSpecialKeyCode(KeyEvent.KEYCODE_MOVE_HOME, focusAfterSend)
            ToolbarKey.END -> sendSpecialKeyCode(KeyEvent.KEYCODE_MOVE_END, focusAfterSend)
            ToolbarKey.PAGE_UP -> sendSpecialKeyCode(KeyEvent.KEYCODE_PAGE_UP, focusAfterSend)
            ToolbarKey.PAGE_DOWN -> sendSpecialKeyCode(KeyEvent.KEYCODE_PAGE_DOWN, focusAfterSend)
            ToolbarKey.PIPE -> sendCodePointWithToolbarModifiers('|'.code, focusAfterSend)
            ToolbarKey.SLASH -> sendCodePointWithToolbarModifiers('/'.code, focusAfterSend)
            ToolbarKey.TILDE -> sendCodePointWithToolbarModifiers('~'.code, focusAfterSend)
            ToolbarKey.BACKTICK -> sendCodePointWithToolbarModifiers('`'.code, focusAfterSend)
        }
    }

    fun focusInput() {
        focusTerminalInput()
    }

    fun terminalFontSizeSp(): Float = terminalFontSizeSp

    private fun ensureTerminalSession() {
        if (terminalSession != null) {
            return
        }
        terminalSession = TerminalSession(
            "/system/bin/sh",
            appContext.filesDir.absolutePath,
            arrayOf("/system/bin/sh", "-c", "while true; do sleep 3600; done"),
            emptyArray(),
            2_000,
            sessionClient,
        )
        terminalOutputProxyInstalled = false
        syncTerminalBackgroundPalette()
        installTerminalOutputProxy()
    }

    fun disconnect() {
        scope.launch(Dispatchers.IO) {
            disconnectInternal(showStatus = true)
        }
    }

    fun handleTerminalSizeChange(columns: Int, rows: Int) {
        if (!connected.get()) {
            return
        }
        if (columns <= 0 || rows <= 0) {
            return
        }
        if (columns == remoteColumns && rows == remoteRows) {
            return
        }

        scope.launch(Dispatchers.IO) {
            ioMutex.withLock {
                val session = sshSession as? SessionChannel ?: return@withLock
                if (sshShell == null || !session.isOpen) {
                    return@withLock
                }
                session.changeWindowDimensions(
                    columns,
                    rows,
                    columns * CELL_WIDTH_PX,
                    rows * CELL_HEIGHT_PX,
                )
                remoteColumns = columns
                remoteRows = rows
                Log.d(TAG, "Updated remote PTY size to ${columns}x${rows}")
            }
        }
    }

    override fun close() {
        remoteWriterJob.cancel()
        scope.launch(Dispatchers.IO) {
            disconnectInternal(showStatus = false)
        }
    }

    private suspend fun disconnectInternal(showStatus: Boolean) {
        manualDisconnectRequested.set(true)
        disconnectHandled.set(true)
        reconnectJob?.cancel()
        stopRepeatingArrowKey()
        pendingConnectJob?.cancel()
        connected.set(false)
        viewportState = TerminalViewportState()
        remoteColumns = 0
        remoteRows = 0
        cleanupConnection(cancelReadJob = true)

        if (showStatus) {
            appendStatusLine("Disconnected.")
            updateSessionState(SessionConnectionState.DISCONNECTED)
            dispatchCallback {
                callbacks.onDisconnected("Disconnected.")
            }
        } else {
            updateSessionState(SessionConnectionState.IDLE)
        }
    }

    private suspend fun pumpRemoteOutput(shell: Session.Command) {
        try {
            val stream = shell.inputStream
            val buffer = ByteArray(DEFAULT_BUFFER_SIZE)
            while (true) {
                val read = stream.read(buffer)
                if (read < 0) {
                    break
                }
                if (read > 0) {
                    appendBytes(buffer, read)
                }
            }
            handleUnexpectedDisconnect(
                reason = DisconnectReason(
                    userMessage = "Remote session closed.",
                    recoverable = false,
                    finalState = SessionConnectionState.DISCONNECTED,
                ),
                cancelReadJob = false,
            )
        } catch (_: CancellationException) {
            return
        } catch (_: EOFException) {
            handleUnexpectedDisconnect(
                reason = DisconnectReason(
                    userMessage = "Remote session closed.",
                    recoverable = false,
                    finalState = SessionConnectionState.DISCONNECTED,
                ),
                cancelReadJob = false,
            )
        } catch (ioe: IOException) {
            handleUnexpectedDisconnect(
                reason = classifyDisconnect(ioe, fallback = "SSH stream failure."),
                cancelReadJob = false,
            )
        }
    }

    private fun handleConnectionFailure(throwable: Throwable) {
        Log.e(TAG, "SSH connection failed", throwable)
        connected.set(false)
        val message = describeConnectionFailure(throwable)
        updateSessionState(SessionConnectionState.FAILED)
        dispatchCallback {
            callbacks.onConnectionFailed(message)
        }
        appendStatusLine("Connection failed: $message")
    }

    private fun dispatchCallback(block: suspend () -> Unit) {
        scope.launch(Dispatchers.Main.immediate) {
            block()
        }
    }

    private fun describeConnectionFailure(throwable: Throwable): String {
        val authDetails = describeAuthFailure(throwable)
        if (authDetails != null) {
            return authDetails
        }

        return throwable.message ?: "Unable to connect over SSH."
    }

    private fun describeAuthFailure(throwable: Throwable): String? {
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

    private fun appendControlSequence(sequence: String) {
        val bytes = sequence.toByteArray(StandardCharsets.UTF_8)
        appendBytes(bytes, bytes.size)
    }

    private fun appendStatusLine(message: String) {
        appendControlSequence("\r\n$message\r\n")
    }

    private fun appendBytes(bytes: ByteArray, length: Int) {
        val chunk = bytes.copyOf(length)
        scope.launch(Dispatchers.Main.immediate) {
            val emulator = terminalSession?.emulator ?: return@launch
            emulator.append(chunk, chunk.size)
            logRemoteEscapeSequences(chunk)
            terminalView?.let(::refreshTerminalViewport)
        }
    }

    private fun logRemoteEscapeSequences(chunk: ByteArray) {
        if (!debugTouchLogs) {
            return
        }
        val text = remoteEscapeLogTail + String(chunk, StandardCharsets.ISO_8859_1)
        val matches =
            REMOTE_ESCAPE_REGEX.findAll(text)
                .map { it.value.replace("\u001B", "<ESC>") }
                .toList()
        if (matches.isNotEmpty()) {
            Log.d(TAG, "remote-escapes=${matches.joinToString(" | ")}")
        }
        remoteEscapeLogTail = text.takeLast(MAX_ESCAPE_LOG_TAIL)
    }

    private fun writeToRemote(bytes: ByteArray) {
        if (!connected.get()) {
            return
        }
        remoteWriteQueue.trySend(bytes.copyOf())
    }

    private fun sendString(text: String) {
        writeToRemote(text.toByteArray(StandardCharsets.UTF_8))
    }

    private suspend fun processRemoteWrites() {
        for (bytes in remoteWriteQueue) {
            if (!connected.get()) {
                continue
            }

            val failure = ioMutex.withLock {
                try {
                    remoteInput?.write(bytes)
                    remoteInput?.flush()
                    null
                } catch (_: CancellationException) {
                    return
                } catch (ioe: IOException) {
                    ioe
                }
            }
            failure ?: continue
            handleUnexpectedDisconnect(
                reason = classifyDisconnect(failure, fallback = "SSH write failed."),
                cancelReadJob = true,
            )
        }
    }

    private fun consumeToolbarModifiers(): ToolbarState {
        val state = toolbarState
        if (state.ctrl || state.alt) {
            toolbarState = state.copy(ctrl = false, alt = false)
        }
        return state
    }

    private fun sendSpecialSequence(sequence: String, focusAfterSend: Boolean = true) {
        val modifiers = consumeToolbarModifiers()
        if (modifiers.alt) {
            sendString("\u001B")
        }
        sendString(sequence)
        if (focusAfterSend) {
            focusTerminalInput(showKeyboard = false)
        }
    }

    private fun sendSpecialKeyCode(keyCode: Int, focusAfterSend: Boolean = true) {
        val modifiers = consumeToolbarModifiers()
        if (modifiers.alt) {
            sendString("\u001B")
        }
        sendTerminalKeyCode(keyCode)
        if (focusAfterSend) {
            focusTerminalInput(showKeyboard = false)
        }
    }

    private fun sendCodePointWithToolbarModifiers(codePoint: Int, focusAfterSend: Boolean = true) {
        val modifiers = consumeToolbarModifiers()
        if (modifiers.alt) {
            sendString("\u001B")
        }
        sendCodePoint(codePoint, modifiers.ctrl)
        if (focusAfterSend) {
            focusTerminalInput(showKeyboard = false)
        }
    }

    private fun sendCodePoint(codePoint: Int, ctrlDown: Boolean) {
        val actual = if (ctrlDown) mapCtrlCodePoint(codePoint) else codePoint
        if (actual == 0) {
            writeToRemote(byteArrayOf(0))
            return
        }
        sendString(String(Character.toChars(actual)))
    }

    private fun mapCtrlCodePoint(codePoint: Int): Int {
        val upper = Character.toUpperCase(codePoint)
        return when {
            upper in 0x41..0x5A -> upper - 0x40
            upper == 0x20 -> 0
            upper == '['.code -> 27
            upper == '\\'.code -> 28
            upper == ']'.code -> 29
            upper == '^'.code -> 30
            upper == '_'.code -> 31
            upper == '?'.code -> 127
            else -> codePoint
        }
    }

    private fun consumeKeyCode(keyCode: Int): Boolean {
        val emulator = terminalSession?.emulator
        val sequence =
            if (emulator != null) {
                KeyHandler.getCode(
                    keyCode,
                    0,
                    emulator.isCursorKeysApplicationMode,
                    emulator.isKeypadApplicationMode,
                )
            } else {
                when (keyCode) {
                    KeyEvent.KEYCODE_ENTER -> "\r"
                    KeyEvent.KEYCODE_DEL -> "\u007F"
                    KeyEvent.KEYCODE_TAB -> "\t"
                    KeyEvent.KEYCODE_DPAD_UP -> "\u001B[A"
                    KeyEvent.KEYCODE_DPAD_DOWN -> "\u001B[B"
                    KeyEvent.KEYCODE_DPAD_RIGHT -> "\u001B[C"
                    KeyEvent.KEYCODE_DPAD_LEFT -> "\u001B[D"
                    KeyEvent.KEYCODE_MOVE_HOME -> "\u001B[H"
                    KeyEvent.KEYCODE_MOVE_END -> "\u001B[F"
                    KeyEvent.KEYCODE_PAGE_UP -> "\u001B[5~"
                    KeyEvent.KEYCODE_PAGE_DOWN -> "\u001B[6~"
                    KeyEvent.KEYCODE_ESCAPE -> "\u001B"
                    KeyEvent.KEYCODE_FORWARD_DEL -> "\u001B[3~"
                    else -> null
                }
            }
        sequence ?: return false
        sendString(sequence)
        return true
    }

    private fun sendArrowKey(keyCode: Int, requestFocus: Boolean = false) {
        if (sendTerminalKeyCode(keyCode) && requestFocus) {
            focusTerminalInput(showKeyboard = false)
        }
    }

    private fun sendGestureArrowKey(keyCode: Int, requestFocus: Boolean = false) {
        if (consumeKeyCode(keyCode) && requestFocus) {
            focusTerminalInput(showKeyboard = false)
        }
    }

    private fun sendTerminalKeyCode(keyCode: Int): Boolean {
        return consumeKeyCode(keyCode)
    }

    private fun startRepeatingArrowKey(keyCode: Int) {
        if (!connected.get()) {
            return
        }
        repeatArrowJob?.cancel()
        repeatArrowJob = scope.launch(Dispatchers.IO) {
            delay(SWIPE_ARROW_REPEAT_INITIAL_DELAY_MS)
            while (connected.get()) {
                sendArrowKey(keyCode, requestFocus = false)
                delay(SWIPE_ARROW_REPEAT_INTERVAL_MS)
            }
        }
    }

    private fun stopRepeatingArrowKey() {
        repeatArrowJob?.cancel()
        repeatArrowJob = null
    }

    private fun isRepeatableNavigationKey(key: ToolbarKey): Boolean {
        return when (key) {
            ToolbarKey.UP,
            ToolbarKey.DOWN,
            ToolbarKey.LEFT,
            ToolbarKey.RIGHT,
            ToolbarKey.HOME,
            ToolbarKey.END,
            ToolbarKey.PAGE_UP,
            ToolbarKey.PAGE_DOWN,
            -> true

            else -> false
        }
    }

    private inner class ControllerTerminalSessionClient : TerminalSessionClient {
        override fun onTextChanged(changedSession: TerminalSession?) {
            terminalView?.post {
                terminalView?.let(::refreshTerminalViewport)
            }
        }

        override fun onTitleChanged(changedSession: TerminalSession?) = Unit

        override fun onSessionFinished(finishedSession: TerminalSession?) = Unit

        override fun onCopyTextToClipboard(session: TerminalSession?, text: String?) {
            val safeText = text ?: return
            val clipboard = appContext.getSystemService<ClipboardManager>() ?: return
            clipboard.setPrimaryClip(ClipData.newPlainText("terminal", safeText))
        }

        override fun onPasteTextFromClipboard(session: TerminalSession?) {
            val clipboard = appContext.getSystemService<ClipboardManager>() ?: return
            val clip = clipboard.primaryClip ?: return
            val item = clip.getItemAt(0) ?: return
            val text = item.coerceToText(appContext)?.toString().orEmpty()
            if (text.isNotBlank()) {
                sendString(text)
            }
        }

        override fun onBell(session: TerminalSession?) = Unit

        override fun onColorsChanged(changedSession: TerminalSession?) {
            terminalView?.post {
                terminalView?.invalidate()
            }
        }

        override fun onTerminalCursorStateChange(state: Boolean) {
            terminalView?.post {
                terminalView?.invalidate()
            }
        }

        override fun getTerminalCursorStyle(): Int? = 0

        override fun logError(tag: String?, message: String?) {
            Log.e(tag ?: TAG, message.orEmpty())
        }

        override fun logWarn(tag: String?, message: String?) {
            Log.w(tag ?: TAG, message.orEmpty())
        }

        override fun logInfo(tag: String?, message: String?) {
            Log.i(tag ?: TAG, message.orEmpty())
        }

        override fun logDebug(tag: String?, message: String?) {
            Log.d(tag ?: TAG, message.orEmpty())
        }

        override fun logVerbose(tag: String?, message: String?) {
            Log.v(tag ?: TAG, message.orEmpty())
        }

        override fun logStackTraceWithMessage(tag: String?, message: String?, throwable: Exception?) {
            Log.e(tag ?: TAG, message.orEmpty(), throwable)
        }

        override fun logStackTrace(tag: String?, throwable: Exception?) {
            Log.e(tag ?: TAG, throwable?.message.orEmpty(), throwable)
        }
    }

    private inner class ControllerTerminalViewClient : TerminalViewClient {
        override fun onScale(scale: Float): Float = applyTerminalScale(scale)

        override fun onSingleTapUp(e: MotionEvent) {
            focusTerminalInput(showKeyboard = true)
        }

        override fun onKeyDown(keyCode: Int, e: KeyEvent?, session: TerminalSession?): Boolean {
            val modifiers = consumeToolbarModifiers()
            if (modifiers.alt) {
                sendString("\u001B")
            }
            val consumed = consumeKeyCode(keyCode)
            if (consumed) {
                focusTerminalInput(showKeyboard = false)
            }
            return consumed
        }

        override fun onKeyUp(keyCode: Int, e: KeyEvent?): Boolean = false

        override fun readControlKey(): Boolean = false

        override fun readAltKey(): Boolean = false

        override fun readShiftKey(): Boolean = false

        override fun readFnKey(): Boolean = false

        override fun onCodePoint(codePoint: Int, ctrlDown: Boolean, session: TerminalSession?): Boolean {
            val modifiers = consumeToolbarModifiers()
            if (modifiers.alt) {
                sendString("\u001B")
            }
            sendCodePoint(codePoint, ctrlDown || modifiers.ctrl)
            focusTerminalInput(showKeyboard = false)
            return true
        }

        override fun onLongPress(event: MotionEvent?): Boolean = false

        override fun onEmulatorSet() {
            updateLastKnownTerminalSize()
            handleTerminalSizeChange(lastColumns, lastRows)
        }

        override fun copyModeChanged(copyMode: Boolean) = Unit

        override fun shouldBackButtonBeMappedToEscape(): Boolean = false

        override fun shouldEnforceCharBasedInput(): Boolean = true

        override fun shouldUseCtrlSpaceWorkaround(): Boolean = true

        override fun isTerminalViewSelected(): Boolean = true

        override fun logError(tag: String?, message: String?) {
            Log.e(tag ?: TAG, message.orEmpty())
        }

        override fun logWarn(tag: String?, message: String?) {
            Log.w(tag ?: TAG, message.orEmpty())
        }

        override fun logInfo(tag: String?, message: String?) {
            Log.i(tag ?: TAG, message.orEmpty())
        }

        override fun logDebug(tag: String?, message: String?) {
            Log.d(tag ?: TAG, message.orEmpty())
        }

        override fun logVerbose(tag: String?, message: String?) {
            Log.v(tag ?: TAG, message.orEmpty())
        }

        override fun logStackTraceWithMessage(tag: String?, message: String?, throwable: Exception?) {
            Log.e(tag ?: TAG, message.orEmpty(), throwable)
        }

        override fun logStackTrace(tag: String?, throwable: Exception?) {
            Log.e(tag ?: TAG, throwable?.message.orEmpty(), throwable)
        }
    }

    private fun updateLastKnownTerminalSize() {
        val emulator = terminalSession?.emulator ?: return
        lastColumns = readIntField(emulator, "mColumns", lastColumns)
        lastRows = readIntField(emulator, "mRows", lastRows)
    }

    private fun applyTerminalScale(scale: Float): Float {
        val view = terminalView ?: return 1.0f
        val clampedFontSizeSp = (DEFAULT_TERMINAL_FONT_SIZE_SP * scale)
            .coerceIn(MIN_TERMINAL_FONT_SIZE_SP, MAX_TERMINAL_FONT_SIZE_SP)
        val normalizedScale = clampedFontSizeSp / DEFAULT_TERMINAL_FONT_SIZE_SP
        if (kotlin.math.abs(clampedFontSizeSp - terminalFontSizeSp) < FONT_SIZE_EPSILON_SP) {
            return normalizedScale
        }

        syncViewportState(view)
        terminalFontSizeSp = clampedFontSizeSp
        preferences.edit().putFloat(KEY_TERMINAL_FONT_SIZE_SP, terminalFontSizeSp).apply()
        val textSizePx = TypedValue.applyDimension(
            TypedValue.COMPLEX_UNIT_SP,
            terminalFontSizeSp,
            appContext.resources.displayMetrics,
        ).toInt()
        view.setTextSize(textSizePx)
        refreshTerminalViewport(view)
        return normalizedScale
    }

    private fun loadSavedTerminalFontSize(): Float {
        val saved = preferences.getFloat(KEY_TERMINAL_FONT_SIZE_SP, DEFAULT_TERMINAL_FONT_SIZE_SP)
        return saved.coerceIn(MIN_TERMINAL_FONT_SIZE_SP, MAX_TERMINAL_FONT_SIZE_SP)
    }

    fun onTerminalViewWillResize(view: TerminalView) {
        syncViewportState(view)
    }

    fun onTerminalViewDidResize(view: TerminalView) {
        refreshTerminalViewport(view)
    }

    private fun focusTerminalInput(showKeyboard: Boolean = true) {
        terminalView?.post {
            terminalView?.isFocusable = true
            terminalView?.isFocusableInTouchMode = true
            terminalView?.requestFocus()
            if (showKeyboard) {
                val imm = appContext.getSystemService<InputMethodManager>()
                terminalView?.let { view ->
                    imm?.showSoftInput(view, InputMethodManager.SHOW_IMPLICIT)
                }
            }
        }
    }

    private fun refreshTerminalViewport(view: TerminalView) {
        logTouchDebug("refreshViewport before topRow=${view.topRow}")
        view.onScreenUpdated()
        restoreViewportAfterScreenUpdate(view)
        logTouchDebug("refreshViewport after topRow=${view.topRow} state=$viewportState")
    }

    private fun syncViewportState(view: TerminalView) {
        val emulator = terminalSession?.emulator ?: return
        if (emulator.isAlternateBufferActive) {
            viewportState = TerminalViewportState()
            return
        }
        val transcriptRows = emulator.screen.activeTranscriptRows
        viewportState = TerminalViewportLogic.syncState(
            topRow = view.topRow,
            transcriptRows = transcriptRows,
        )
        logTouchDebug(
            "syncViewportState topRow=${view.topRow} transcriptRows=$transcriptRows state=$viewportState",
        )
    }

    private fun restoreViewportAfterScreenUpdate(view: TerminalView) {
        val emulator = terminalSession?.emulator ?: return
        if (emulator.isAlternateBufferActive) {
            viewportState = TerminalViewportState()
            if (view.topRow != 0) {
                view.setTopRow(0)
                view.invalidate()
            }
            return
        }
        val transcriptRows = emulator.screen.activeTranscriptRows
        if (transcriptRows <= 0) {
            viewportState = TerminalViewportState()
            return
        }

        val (restoredState, restoredTopRow) = TerminalViewportLogic.restoreTopRow(
            state = viewportState,
            transcriptRows = transcriptRows,
            currentTopRow = view.topRow,
        )
        if (restoredTopRow != view.topRow) {
            view.setTopRow(restoredTopRow)
            view.invalidate()
        }
        viewportState = restoredState
    }

    private inner class TerminalTouchInterceptor(
        private val view: TerminalView,
    ) : View.OnTouchListener {
        private var touchStartX = 0f
        private var touchStartY = 0f
        private var touchStartTimeMs = 0L
        private var suppressNextSingleFingerUp = false
        private var activeArrowKeyCode: Int? = null
        private var activeArrowGesture = false
        private var activeTwoFingerScroll = false
        private var twoFingerPointerId1 = MotionEvent.INVALID_POINTER_ID
        private var twoFingerPointerId2 = MotionEvent.INVALID_POINTER_ID
        private var twoFingerStartY1 = 0f
        private var twoFingerStartY2 = 0f
        private var twoFingerStartX1 = 0f
        private var twoFingerStartX2 = 0f
        private var twoFingerLastY1 = 0f
        private var twoFingerLastY2 = 0f
        private var twoFingerLastX1 = 0f
        private var twoFingerLastX2 = 0f
        private var twoFingerLastFocusY = 0f
        private var twoFingerScrollRemainder = 0f
        override fun onTouch(v: View?, event: MotionEvent): Boolean {
            logTouchDebug(
                "touch action=${event.actionMasked} pointers=${event.pointerCount} topRow=${view.topRow} selecting=${view.isSelectingText()}",
            )
            if (view.isSelectingText()) {
                resetArrowGesture()
                if (event.actionMasked == MotionEvent.ACTION_UP || event.actionMasked == MotionEvent.ACTION_CANCEL) {
                    view.post { syncViewportState(view) }
                }
                logTouchDebug("touch pass-through selection action=${event.actionMasked}")
                return false
            }

            if (event.pointerCount >= 2 || activeTwoFingerScroll) {
                return handleTwoFingerScroll(event)
            }

            val deltaX = event.x - touchStartX
            val deltaY = event.y - touchStartY
            when (event.actionMasked) {
                MotionEvent.ACTION_DOWN -> {
                    touchStartX = event.x
                    touchStartY = event.y
                    touchStartTimeMs = event.eventTime
                    suppressNextSingleFingerUp = false
                    activeArrowKeyCode = null
                    activeArrowGesture = false
                    stopRepeatingArrowKey()
                    return true
                }
                MotionEvent.ACTION_MOVE -> {
                    return true
                }
                MotionEvent.ACTION_UP,
                MotionEvent.ACTION_CANCEL -> {
                    if (suppressNextSingleFingerUp) {
                        suppressNextSingleFingerUp = false
                        resetArrowGesture()
                        view.post { syncViewportState(view) }
                        return true
                    }
                    val consumedByArrowGesture = activeArrowGesture
                    val gestureDurationMs = event.eventTime - touchStartTimeMs
                    val movedDistance = kotlin.math.hypot(deltaX.toDouble(), deltaY.toDouble()).toFloat()
                    val isTap = event.actionMasked == MotionEvent.ACTION_UP && movedDistance <= TAP_SLOP_PX
                    val flickKeyCode =
                        if (
                            event.actionMasked == MotionEvent.ACTION_UP &&
                            gestureDurationMs <= ONE_FINGER_FLICK_MAX_DURATION_MS &&
                            TerminalTouchPolicy.shouldStartArrowGesture(deltaX, deltaY, view.topRow)
                        ) {
                            TerminalTouchPolicy.resolveArrowKeyCode(deltaX, deltaY)
                        } else {
                            null
                        }
                    resetArrowGesture()
                    view.post { syncViewportState(view) }
                    if (flickKeyCode != null) {
                        logTouchDebug("flickGesture key=$flickKeyCode dx=$deltaX dy=$deltaY duration=$gestureDurationMs")
                        sendGestureArrowKey(flickKeyCode, requestFocus = false)
                        return true
                    }
                    if (isTap) {
                        focusTerminalInput(showKeyboard = true)
                        view.performClick()
                        return true
                    }
                    if (consumedByArrowGesture) {
                        logTouchDebug("arrowGesture end consumed")
                        return true
                    }
                    return true
                }
            }
            return TerminalTouchPolicy.shouldConsumeEvent(
                actionMasked = event.actionMasked,
                pointerCount = event.pointerCount,
                isSelectingText = view.isSelectingText(),
                activeArrowGesture = activeArrowGesture,
                consumedByGestureDetector = false,
                deltaX = deltaX,
                deltaY = deltaY,
                topRow = view.topRow,
            )
        }

        private fun resetArrowGesture() {
            activeArrowKeyCode = null
            activeArrowGesture = false
            stopRepeatingArrowKey()
        }

        private fun handleTwoFingerScroll(event: MotionEvent): Boolean {
            resetArrowGesture()
            when (event.actionMasked) {
                MotionEvent.ACTION_DOWN,
                MotionEvent.ACTION_POINTER_DOWN -> {
                    if (event.pointerCount >= 2) {
                        suppressNextSingleFingerUp = true
                        initializeTwoFingerTracking(event)
                        logTouchDebug("twoFingerTrack start pointers=${event.pointerCount}")
                        return true
                    }
                }

                MotionEvent.ACTION_MOVE -> {
                    if (event.pointerCount < 2) {
                        return activeTwoFingerScroll
                    }
                    val metrics = readTwoFingerMetrics(event) ?: return true
                    if (!activeTwoFingerScroll) {
                        if (
                            TerminalTouchPolicy.shouldStartTwoFingerScroll(
                                totalDeltaX1 = metrics.totalDeltaX1,
                                totalDeltaY1 = metrics.totalDeltaY1,
                                totalDeltaX2 = metrics.totalDeltaX2,
                                totalDeltaY2 = metrics.totalDeltaY2,
                            )
                        ) {
                            activeTwoFingerScroll = true
                            twoFingerScrollRemainder = 0f
                            twoFingerLastFocusY = metrics.focusY
                            logTouchDebug(
                                "twoFingerScroll start totalDy1=${metrics.totalDeltaY1} totalDy2=${metrics.totalDeltaY2}",
                            )
                        } else {
                            updateTwoFingerTracking(metrics)
                            return true
                        }
                    }

                    val distanceY = metrics.focusY - twoFingerLastFocusY
                    val lineHeightPx = ((view.getPointY(1) - view.getPointY(0)).takeIf { it > 0 } ?: CELL_HEIGHT_PX).toFloat()
                    val scrollRows = ((twoFingerScrollRemainder + distanceY) / lineHeightPx).toInt()
                    twoFingerScrollRemainder = (twoFingerScrollRemainder + distanceY) - (scrollRows * lineHeightPx)
                    updateTwoFingerTracking(metrics)
                    if (scrollRows != 0) {
                        dispatchTwoFingerScroll(view, event, scrollRows)
                    }
                    return true
                }

                MotionEvent.ACTION_POINTER_UP -> {
                    suppressNextSingleFingerUp = true
                    if (!activeTwoFingerScroll) {
                        if (event.pointerCount - 1 >= 2) {
                            initializeTwoFingerTrackingExcluding(event, event.actionIndex)
                        } else {
                            clearTwoFingerTracking()
                        }
                        return true
                    }
                    if (event.pointerCount - 1 >= 2) {
                        initializeTwoFingerTrackingExcluding(event, event.actionIndex)
                        return true
                    }
                    finishTwoFingerScroll()
                    view.post { syncViewportState(view) }
                    return true
                }

                MotionEvent.ACTION_UP,
                MotionEvent.ACTION_CANCEL -> {
                    val consumed = activeTwoFingerScroll
                    finishTwoFingerScroll()
                    view.post { syncViewportState(view) }
                    return consumed
                }
            }
            return activeTwoFingerScroll
        }

        private fun finishTwoFingerScroll() {
            if (activeTwoFingerScroll) {
                logTouchDebug("twoFingerScroll end")
            }
            activeTwoFingerScroll = false
            twoFingerScrollRemainder = 0f
            clearTwoFingerTracking()
        }

        private fun initializeTwoFingerTracking(event: MotionEvent) {
            if (event.pointerCount < 2) {
                clearTwoFingerTracking()
                return
            }
            twoFingerPointerId1 = event.getPointerId(0)
            twoFingerPointerId2 = event.getPointerId(1)
            val firstX = event.getX(0)
            val firstY = event.getY(0)
            val secondX = event.getX(1)
            val secondY = event.getY(1)
            twoFingerStartX1 = firstX
            twoFingerStartY1 = firstY
            twoFingerStartX2 = secondX
            twoFingerStartY2 = secondY
            twoFingerLastX1 = firstX
            twoFingerLastY1 = firstY
            twoFingerLastX2 = secondX
            twoFingerLastY2 = secondY
            twoFingerLastFocusY = (firstY + secondY) / 2f
        }

        private fun initializeTwoFingerTrackingExcluding(event: MotionEvent, excludedIndex: Int) {
            if (event.pointerCount - 1 < 2) {
                clearTwoFingerTracking()
                return
            }
            var firstIndex = -1
            var secondIndex = -1
            for (index in 0 until event.pointerCount) {
                if (index == excludedIndex) continue
                if (firstIndex == -1) {
                    firstIndex = index
                } else {
                    secondIndex = index
                    break
                }
            }
            if (firstIndex == -1 || secondIndex == -1) {
                clearTwoFingerTracking()
                return
            }
            twoFingerPointerId1 = event.getPointerId(firstIndex)
            twoFingerPointerId2 = event.getPointerId(secondIndex)
            val firstX = event.getX(firstIndex)
            val firstY = event.getY(firstIndex)
            val secondX = event.getX(secondIndex)
            val secondY = event.getY(secondIndex)
            twoFingerStartX1 = firstX
            twoFingerStartY1 = firstY
            twoFingerStartX2 = secondX
            twoFingerStartY2 = secondY
            twoFingerLastX1 = firstX
            twoFingerLastY1 = firstY
            twoFingerLastX2 = secondX
            twoFingerLastY2 = secondY
            twoFingerLastFocusY = (firstY + secondY) / 2f
        }

        private fun readTwoFingerMetrics(event: MotionEvent): TwoFingerMetrics? {
            val index1 = event.findPointerIndex(twoFingerPointerId1)
            val index2 = event.findPointerIndex(twoFingerPointerId2)
            if (index1 < 0 || index2 < 0) {
                initializeTwoFingerTracking(event)
                return null
            }
            val currentX1 = event.getX(index1)
            val currentY1 = event.getY(index1)
            val currentX2 = event.getX(index2)
            val currentY2 = event.getY(index2)
            return TwoFingerMetrics(
                currentX1 = currentX1,
                currentY1 = currentY1,
                currentX2 = currentX2,
                currentY2 = currentY2,
                totalDeltaX1 = currentX1 - twoFingerStartX1,
                totalDeltaY1 = currentY1 - twoFingerStartY1,
                totalDeltaX2 = currentX2 - twoFingerStartX2,
                totalDeltaY2 = currentY2 - twoFingerStartY2,
                focusY = (currentY1 + currentY2) / 2f,
            )
        }

        private fun updateTwoFingerTracking(metrics: TwoFingerMetrics) {
            twoFingerLastX1 = metrics.currentX1
            twoFingerLastY1 = metrics.currentY1
            twoFingerLastX2 = metrics.currentX2
            twoFingerLastY2 = metrics.currentY2
            twoFingerLastFocusY = metrics.focusY
        }

        private fun clearTwoFingerTracking() {
            twoFingerPointerId1 = MotionEvent.INVALID_POINTER_ID
            twoFingerPointerId2 = MotionEvent.INVALID_POINTER_ID
            twoFingerStartX1 = 0f
            twoFingerStartY1 = 0f
            twoFingerStartX2 = 0f
            twoFingerStartY2 = 0f
            twoFingerLastX1 = 0f
            twoFingerLastY1 = 0f
            twoFingerLastX2 = 0f
            twoFingerLastY2 = 0f
            twoFingerLastFocusY = 0f
        }

        private fun dispatchTwoFingerScroll(view: TerminalView, event: MotionEvent, scrollRows: Int) {
            val emulator = terminalSession?.emulator ?: return
            if (emulator.isMouseTrackingActive()) {
                val point = view.getColumnAndRow(event, false)
                val column = point[0] + 1
                val row = point[1] + 1
                val button =
                    if (scrollRows < 0) {
                        com.termux.terminal.TerminalEmulator.MOUSE_WHEELDOWN_BUTTON
                    } else {
                        com.termux.terminal.TerminalEmulator.MOUSE_WHEELUP_BUTTON
                    }
                repeat(kotlin.math.abs(scrollRows)) {
                    emulator.sendMouseEvent(button, column, row, true)
                }
                return
            }
            scrollViewportRows(view, scrollRows)
        }

    }

    private data class TwoFingerMetrics(
        val currentX1: Float,
        val currentY1: Float,
        val currentX2: Float,
        val currentY2: Float,
        val totalDeltaX1: Float,
        val totalDeltaY1: Float,
        val totalDeltaX2: Float,
        val totalDeltaY2: Float,
        val focusY: Float,
    )

    private fun scrollViewportRows(view: TerminalView, rowsDelta: Int) {
        val transcriptRows = terminalSession?.emulator?.screen?.activeTranscriptRows ?: return
        val clampedTopRow = (view.topRow - rowsDelta).coerceIn(-transcriptRows, 0)
        if (clampedTopRow == view.topRow) {
            return
        }
        view.setTopRow(clampedTopRow)
        view.invalidate()
    }

    private fun readIntField(instance: Any, name: String, fallback: Int): Int {
        return runCatching {
            val field = instance.javaClass.getDeclaredField(name)
            field.isAccessible = true
            field.getInt(instance)
        }.getOrElse { fallback }
    }

    private fun syncTerminalBackgroundPalette() {
        val session = terminalSession ?: return
        runCatching {
            val emulatorField = session.javaClass.getDeclaredField("mEmulator")
            emulatorField.isAccessible = true
            val emulator = emulatorField.get(session) ?: return

            val colorsField = emulator.javaClass.getDeclaredField("mColors")
            colorsField.isAccessible = true
            val colors = colorsField.get(emulator) ?: return

            val currentColorsField = colors.javaClass.getDeclaredField("mCurrentColors")
            currentColorsField.isAccessible = true
            val currentColors = currentColorsField.get(colors) as? IntArray ?: return

            if (currentColors.size > TERMINAL_COLOR_INDEX_BACKGROUND) {
                currentColors[TERMINAL_COLOR_INDEX_BACKGROUND] = TERMINAL_BACKGROUND_COLOR
            }
        }.onFailure {
            Log.w(TAG, "Unable to sync terminal background palette", it)
        }
        terminalView?.invalidate()
    }

    private fun installTerminalOutputProxy() {
        if (terminalOutputProxyInstalled) {
            return
        }
        val session = terminalSession ?: return
        runCatching {
            val emulatorField = session.javaClass.getDeclaredField("mEmulator")
            emulatorField.isAccessible = true
            val emulator = emulatorField.get(session) ?: return

            val sessionField = emulator.javaClass.getDeclaredField("mSession")
            sessionField.isAccessible = true
            sessionField.set(emulator, RemoteAwareTerminalOutput(session))
            terminalOutputProxyInstalled = true
        }.onFailure {
            Log.w(TAG, "Unable to install terminal output proxy", it)
        }
    }

    private inner class RemoteAwareTerminalOutput(
        private val delegate: TerminalSession,
    ) : TerminalOutput() {
        override fun write(data: ByteArray, offset: Int, count: Int) {
            if (count <= 0) {
                return
            }
            if (connected.get()) {
                writeToRemote(data.copyOfRange(offset, offset + count))
            } else {
                delegate.write(data, offset, count)
            }
        }

        override fun titleChanged(oldTitle: String?, newTitle: String?) {
            delegate.titleChanged(oldTitle, newTitle)
        }

        override fun onCopyTextToClipboard(text: String?) {
            delegate.onCopyTextToClipboard(text)
        }

        override fun onPasteTextFromClipboard() {
            delegate.onPasteTextFromClipboard()
        }

        override fun onBell() {
            delegate.onBell()
        }

        override fun onColorsChanged() {
            delegate.onColorsChanged()
        }
    }

    private fun logTouchDebug(message: String) {
        if (debugTouchLogs) {
            Log.d(TAG, message)
        }
    }

    companion object {
        private const val TAG = "SshTerminalController"
        private const val TERMINAL_PREFS_NAME = "terminal_prefs"
        private const val KEY_TERMINAL_FONT_SIZE_SP = "terminal_font_size_sp"
        private const val TERMINAL_COLOR_INDEX_BACKGROUND = 257
        private const val MAX_RECONNECT_ATTEMPTS = 3
        private const val DEFAULT_TERMINAL_FONT_SIZE_SP = 12f
        private const val MIN_TERMINAL_FONT_SIZE_SP = 8f
        private const val MAX_TERMINAL_FONT_SIZE_SP = 24f
        private const val FONT_SIZE_EPSILON_SP = 0.05f
        private const val SWIPE_ARROW_REPEAT_INITIAL_DELAY_MS = 120L
        private const val SWIPE_ARROW_REPEAT_INTERVAL_MS = 45L
        private const val TOOLBAR_REPEAT_INITIAL_DELAY_MS = 120L
        private const val TOOLBAR_REPEAT_INTERVAL_MS = 45L
        private const val CELL_WIDTH_PX = 9
        private const val CELL_HEIGHT_PX = 18
        private const val TAP_SLOP_PX = 24f
        private const val ONE_FINGER_FLICK_MAX_DURATION_MS = 250L
        private const val MAX_ESCAPE_LOG_TAIL = 64
        private const val TERMINAL_BACKGROUND_COLOR = 0xFF101313.toInt()
        private val REMOTE_ESCAPE_REGEX = Regex("\u001B\\[[0-9;?<>:=]*[ -/]*[@-~]")
        private const val REMOTE_WORKSPACE_COMMAND =
            "env TERM=xterm-256color COLORTERM=truecolor " +
                "sh -lc 'tmux has-session -t arc 2>/dev/null || tmux new-session -d -s arc; exec tmux attach-session -d -t arc'"
        private val RECONNECT_BACKOFF_MS = longArrayOf(1_000L, 2_000L, 4_000L)
    }

    private suspend fun establishConnection(
        config: SshQrConfig,
        isReconnect: Boolean,
    ) {
        try {
            awaitTerminalReadyForHandshake()
            appendStatusLine("Opening shared SSH transport...")
            val ssh = connectionManager.ensureConnected(
                config = config,
                forceReconnect = isReconnect,
            )
            transportDisconnectMessage = connectionManager.latestDisconnectMessage()

            val cols = lastColumns.coerceAtLeast(40)
            val rows = lastRows.coerceAtLeast(10)
            val session = ssh.startSession()
            session.allocatePTY(
                "xterm-256color",
                cols,
                rows,
                cols * 9,
                rows * 18,
                emptyMap(),
            )
            val shell = session.exec(REMOTE_WORKSPACE_COMMAND)

            ioMutex.withLock {
                sshSession = session
                sshShell = shell
                remoteInput = shell.outputStream
            }
            remoteColumns = cols
            remoteRows = rows

            connected.set(true)
            disconnectHandled.set(false)
            reconnectAttempt = 0
            reconnectJob = null
            transportDisconnectMessage = null
            updateSessionState(SessionConnectionState.CONNECTED)
            dispatchCallback {
                callbacks.onConnected(config)
            }
            if (isReconnect) {
                appendStatusLine("Reconnected to ${config.host}")
            } else {
                appendControlSequence("\u001B[2J\u001B[H")
                appendStatusLine("Connected to ${config.host}")
            }

            readJob = scope.launch(Dispatchers.IO) {
                pumpRemoteOutput(shell)
            }
        } catch (throwable: Throwable) {
            if (isReconnect) {
                handleReconnectFailure(throwable)
            } else {
                handleConnectionFailure(throwable)
            }
        }
    }

    private suspend fun awaitTerminalReadyForHandshake() {
        while (terminalView == null) {
            delay(16)
        }
    }

    private fun handleReconnectFailure(throwable: Throwable) {
        val reason = classifyDisconnect(throwable, fallback = "SSH reconnect failed.")
        Log.w(TAG, "SSH reconnect failed: ${reason.userMessage}", throwable)
        scope.launch(Dispatchers.IO) {
            cleanupConnection(cancelReadJob = true)
            if (reason.recoverable && reconnectAttempt < MAX_RECONNECT_ATTEMPTS && !manualDisconnectRequested.get()) {
                scheduleReconnect(reason.userMessage)
                return@launch
            }
            finalizeDisconnect(reason)
        }
    }

    private suspend fun cleanupConnection(cancelReadJob: Boolean) {
        if (cancelReadJob) {
            readJob?.cancel()
        }
        readJob = null

        ioMutex.withLock {
            remoteInput = null
            runCatching { sshShell?.close() }
            runCatching { sshSession?.close() }
            sshShell = null
            sshSession = null
        }
    }

    private suspend fun handleUnexpectedDisconnect(
        reason: DisconnectReason,
        cancelReadJob: Boolean,
    ) {
        if (!disconnectHandled.compareAndSet(false, true)) {
            return
        }

        Log.w(TAG, "handleUnexpectedDisconnect: ${reason.userMessage}")
        logTransportDiagnostics("disconnect")
        connected.set(false)
        cleanupConnection(cancelReadJob = cancelReadJob)

        if (reason.recoverable && !manualDisconnectRequested.get()) {
            scheduleReconnect(reason.userMessage)
            return
        }

        finalizeDisconnect(reason)
    }

    private fun scheduleReconnect(baseMessage: String) {
        val config = activeConfig ?: run {
            scope.launch(Dispatchers.IO) {
                finalizeDisconnect(
                    DisconnectReason(
                        userMessage = baseMessage,
                        recoverable = false,
                        finalState = SessionConnectionState.FAILED,
                    ),
                )
            }
            return
        }
        if (reconnectAttempt >= MAX_RECONNECT_ATTEMPTS) {
            scope.launch(Dispatchers.IO) {
                finalizeDisconnect(
                    DisconnectReason(
                        userMessage = "$baseMessage Reconnect failed after $MAX_RECONNECT_ATTEMPTS attempts.",
                        recoverable = false,
                        finalState = SessionConnectionState.FAILED,
                    ),
                )
            }
            return
        }

        reconnectAttempt += 1
        val attempt = reconnectAttempt
        val delayMs = RECONNECT_BACKOFF_MS.getOrElse(attempt - 1) { RECONNECT_BACKOFF_MS.last() }
        Log.w(TAG, "scheduleReconnect: attempt=$attempt/$MAX_RECONNECT_ATTEMPTS reason=$baseMessage")
        reconnectJob?.cancel()
        reconnectJob = scope.launch(Dispatchers.IO) {
            val reconnectMessage = "Connection lost, retrying... ($attempt/$MAX_RECONNECT_ATTEMPTS)"
            updateStatus(SessionConnectionState.RECONNECTING, reconnectMessage)
            appendStatusLine(reconnectMessage)
            delay(delayMs)
            if (manualDisconnectRequested.get()) {
                return@launch
            }
            logTransportDiagnostics("before-reconnect-$attempt")
            runCatching {
                prepareReconnectTransport(config)
                connectionManager.reconnect(config)
            }.onFailure { failure ->
                Log.w(TAG, "scheduleReconnect: transport refresh failed", failure)
            }
            logTransportDiagnostics("after-transport-refresh-$attempt")
            establishConnection(config, isReconnect = true)
        }
    }

    private suspend fun logTransportDiagnostics(stage: String) {
        val snapshot = runCatching {
            diagnoseTransport(activeConfig)
        }.getOrElse { throwable ->
            "diagnostics-error=${throwable.javaClass.simpleName}:${throwable.message.orEmpty()}"
        }
        Log.w(
            TAG,
            "transportDiagnostics stage=$stage connected=${connected.get()} state=$currentState manualDisconnect=${manualDisconnectRequested.get()} reconnectAttempt=$reconnectAttempt details=[$snapshot]",
        )
    }

    private suspend fun finalizeDisconnect(reason: DisconnectReason) {
        reconnectJob?.cancel()
        reconnectJob = null
        updateSessionState(reason.finalState)
        appendStatusLine(reason.userMessage)
        when (reason.finalState) {
            SessionConnectionState.DISCONNECTED -> dispatchCallback {
                callbacks.onDisconnected(reason.userMessage)
            }

            SessionConnectionState.FAILED -> dispatchCallback {
                callbacks.onConnectionFailed(reason.userMessage)
            }

            else -> Unit
        }
    }

    private fun classifyDisconnect(throwable: Throwable, fallback: String): DisconnectReason {
        val authDetails = describeAuthFailure(throwable)
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

    private fun classifyTransportDisconnect(reason: String, message: String?): DisconnectReason {
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

    private fun updateStatus(
        state: SessionConnectionState,
        status: String,
    ) {
        updateSessionState(state)
        dispatchCallback {
            callbacks.onStatusChanged(status)
        }
    }

    private fun updateSessionState(state: SessionConnectionState) {
        currentState = state
        dispatchCallback {
            callbacks.onSessionStateChanged(state)
        }
    }

    private data class DisconnectReason(
        val userMessage: String,
        val recoverable: Boolean,
        val finalState: SessionConnectionState,
    )
}
