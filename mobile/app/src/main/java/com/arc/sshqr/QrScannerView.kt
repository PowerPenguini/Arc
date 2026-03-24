package com.arc.sshqr

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.util.Log
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalInspectionMode
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LocalLifecycleOwner
import com.arc.sshqr.ui.theme.ArcTerminalFontFamily
import com.google.mlkit.vision.barcode.BarcodeScannerOptions
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.common.InputImage
import java.util.concurrent.Executor
import java.util.concurrent.atomic.AtomicBoolean

@Composable
fun QrScannerView(
    active: Boolean,
    onCameraAccessDenied: () -> Unit,
    onPayloadScanned: (String) -> Unit,
    onScannerError: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    val inspectionMode = LocalInspectionMode.current
    val mainExecutor = remember(context) { ContextCompat.getMainExecutor(context) }
    var hasCameraPermission by remember { mutableStateOf(context.hasCameraPermission()) }
    val permissionLauncher = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        hasCameraPermission = granted
        if (!granted) {
            onCameraAccessDenied()
        }
    }

    LaunchedEffect(active, hasCameraPermission) {
        if (active && !hasCameraPermission) {
            permissionLauncher.launch(Manifest.permission.CAMERA)
        }
    }

    Box(
        modifier = modifier
            .background(Color(0xFF0A0D0E), RoundedCornerShape(0.dp)),
    ) {
        when {
            !active -> Unit
            inspectionMode -> {
                ScannerPlaceholder("Scanner preview unavailable in inspection mode.")
            }
            !hasCameraPermission -> {
                ScannerPermissionPrompt(
                    onGrantPermission = { permissionLauncher.launch(Manifest.permission.CAMERA) },
                )
            }
            else -> {
                CameraPreview(
                    context = context,
                    lifecycleOwner = lifecycleOwner,
                    mainExecutor = mainExecutor,
                    onPayloadScanned = onPayloadScanned,
                    onScannerError = onScannerError,
                    modifier = Modifier.fillMaxSize(),
                )
            }
        }
    }
}

@Composable
private fun ScannerPermissionPrompt(onGrantPermission: () -> Unit) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF111618)),
        contentAlignment = Alignment.Center,
    ) {
        Button(onClick = onGrantPermission, shape = RoundedCornerShape(0.dp)) {
            Text("Grant camera access", fontFamily = ArcTerminalFontFamily)
        }
    }
}

@Composable
private fun ScannerPlaceholder(message: String) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF111618)),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = message,
            textAlign = TextAlign.Center,
            style = MaterialTheme.typography.bodyMedium,
            fontFamily = ArcTerminalFontFamily,
            color = Color(0xFFD8DADF),
            modifier = Modifier.padding(24.dp),
        )
    }
}

@Composable
private fun CameraPreview(
    context: Context,
    lifecycleOwner: androidx.lifecycle.LifecycleOwner,
    mainExecutor: Executor,
    onPayloadScanned: (String) -> Unit,
    onScannerError: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val previewView = remember(context) {
        PreviewView(context).apply {
            implementationMode = PreviewView.ImplementationMode.COMPATIBLE
            scaleType = PreviewView.ScaleType.FILL_CENTER
        }
    }

    DisposableEffect(previewView, lifecycleOwner, context, mainExecutor) {
        val cameraProviderFuture = ProcessCameraProvider.getInstance(context)
        var analyzer: QrCodeAnalyzer? = null
        var cameraProvider: ProcessCameraProvider? = null
        val listener =
            Runnable {
                runCatching {
                    cameraProviderFuture.get().also { provider ->
                        cameraProvider = provider
                        analyzer = QrCodeAnalyzer(
                            mainExecutor = mainExecutor,
                            onPayloadScanned = onPayloadScanned,
                        )
                        val preview =
                            Preview.Builder().build().apply {
                                setSurfaceProvider(previewView.surfaceProvider)
                            }
                        val analysis =
                            ImageAnalysis.Builder()
                                .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                                .build()
                                .also {
                                    it.setAnalyzer(mainExecutor, requireNotNull(analyzer))
                                }
                        provider.unbindAll()
                        provider.bindToLifecycle(
                            lifecycleOwner,
                            CameraSelector.DEFAULT_BACK_CAMERA,
                            preview,
                            analysis,
                        )
                    }
                }.onFailure { failure ->
                    Log.e(TAG, "Unable to bind CameraX scanner", failure)
                    onScannerError(failure.message ?: "Unable to start camera preview.")
                }
            }
        cameraProviderFuture.addListener(listener, mainExecutor)

        onDispose {
            analyzer?.close()
            cameraProvider?.unbindAll()
        }
    }

    AndroidView(
        factory = { previewView },
        modifier = modifier.clip(RoundedCornerShape(0.dp)),
    )
}

private fun Context.hasCameraPermission(): Boolean {
    return ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED
}

private class QrCodeAnalyzer(
    private val mainExecutor: Executor,
    private val onPayloadScanned: (String) -> Unit,
) : ImageAnalysis.Analyzer {
    private val processing = AtomicBoolean(false)
    private val delivered = AtomicBoolean(false)
    private val scanner =
        BarcodeScanning.getClient(
            BarcodeScannerOptions.Builder()
                .setBarcodeFormats(Barcode.FORMAT_QR_CODE)
                .build(),
        )

    override fun analyze(imageProxy: androidx.camera.core.ImageProxy) {
        val mediaImage = imageProxy.image
        if (delivered.get() || mediaImage == null) {
            imageProxy.close()
            return
        }
        if (!processing.compareAndSet(false, true)) {
            imageProxy.close()
            return
        }

        val image = InputImage.fromMediaImage(mediaImage, imageProxy.imageInfo.rotationDegrees)
        scanner.process(image)
            .addOnSuccessListener { barcodes ->
                val rawValue = barcodes.firstNotNullOfOrNull { it.rawValue?.takeIf(String::isNotBlank) }
                if (rawValue != null && delivered.compareAndSet(false, true)) {
                    mainExecutor.execute {
                        onPayloadScanned(rawValue)
                    }
                }
            }
            .addOnFailureListener { failure ->
                Log.w(TAG, "Ignoring frame analysis failure", failure)
            }
            .addOnCompleteListener {
                processing.set(false)
                imageProxy.close()
            }
    }

    fun close() {
        scanner.close()
    }

    private companion object {
        private const val TAG = "QrCodeAnalyzer"
    }
}

private const val TAG = "QrScannerView"
