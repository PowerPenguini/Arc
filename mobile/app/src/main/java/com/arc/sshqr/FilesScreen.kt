package com.arc.sshqr

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.ArrowBack
import androidx.compose.material.icons.outlined.ArrowDownward
import androidx.compose.material.icons.outlined.CreateNewFolder
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material.icons.outlined.DriveFolderUpload
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.FileOpen
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material.icons.outlined.NoteAdd
import androidx.compose.material.icons.outlined.Refresh
import androidx.compose.material.icons.outlined.SubdirectoryArrowLeft
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.arc.sshqr.files.FilesUiState
import com.arc.sshqr.files.RemoteFileEntry
import com.arc.sshqr.ui.theme.ArcTerminalAccent
import com.arc.sshqr.ui.theme.ArcTerminalFontFamily

private val FilesSharpShape = RoundedCornerShape(0.dp)

@Composable
fun FilesView(
    state: FilesUiState,
    onBackToMenu: () -> Unit,
    onRefresh: () -> Unit,
    onNavigateUp: () -> Unit,
    onOpenDirectory: (RemoteFileEntry) -> Unit,
    onCreateDirectory: (String) -> Unit,
    onCreateFile: (String) -> Unit,
    onRename: (RemoteFileEntry, String) -> Unit,
    onDelete: (RemoteFileEntry) -> Unit,
    onUploadClick: () -> Unit,
    onDownloadClick: (RemoteFileEntry) -> Unit,
) {
    BackHandler(onBack = onBackToMenu)
    var createFolderDialog by rememberSaveable { mutableStateOf(false) }
    var createFileDialog by rememberSaveable { mutableStateOf(false) }
    var renameTarget by rememberSaveable(stateSaver = RemoteFileEntrySaver) { mutableStateOf<RemoteFileEntry?>(null) }
    var deleteTarget by rememberSaveable(stateSaver = RemoteFileEntrySaver) { mutableStateOf<RemoteFileEntry?>(null) }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black)
            .padding(20.dp),
    ) {
        Column(
            modifier = Modifier.fillMaxSize(),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Row(
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    IconButton(onClick = onBackToMenu) {
                        Icon(Icons.Outlined.ArrowBack, contentDescription = "Back", tint = Color(0xFFD8DADF))
                    }
                    Column {
                        Text(
                            text = "Files",
                            style = MaterialTheme.typography.headlineMedium,
                            fontFamily = ArcTerminalFontFamily,
                            fontWeight = FontWeight.Bold,
                            color = Color(0xFFD8DADF),
                        )
                        Text(
                            text = state.currentPath,
                            style = MaterialTheme.typography.bodySmall,
                            fontFamily = ArcTerminalFontFamily,
                            color = Color(0xFF9EA4AF),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
                Row {
                    IconButton(onClick = onRefresh, enabled = !state.isLoading) {
                        Icon(Icons.Outlined.Refresh, contentDescription = "Refresh", tint = Color(0xFFD8DADF))
                    }
                    IconButton(onClick = onNavigateUp, enabled = state.currentPath != state.rootPath && !state.isLoading) {
                        Icon(Icons.Outlined.SubdirectoryArrowLeft, contentDescription = "Up", tint = Color(0xFFD8DADF))
                    }
                }
            }

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                FilesToolbarButton(
                    label = "Upload",
                    icon = Icons.Outlined.DriveFolderUpload,
                    onClick = onUploadClick,
                    enabled = !state.isLoading,
                )
                FilesToolbarButton(
                    label = "New dir",
                    icon = Icons.Outlined.CreateNewFolder,
                    onClick = { createFolderDialog = true },
                    enabled = !state.isLoading,
                )
                FilesToolbarButton(
                    label = "New file",
                    icon = Icons.Outlined.NoteAdd,
                    onClick = { createFileDialog = true },
                    enabled = !state.isLoading,
                )
            }

            state.error?.let {
                Text(
                    text = it,
                    style = MaterialTheme.typography.bodyMedium,
                    fontFamily = ArcTerminalFontFamily,
                    color = MaterialTheme.colorScheme.error,
                )
            }

            state.status?.let {
                Text(
                    text = it,
                    style = MaterialTheme.typography.bodySmall,
                    fontFamily = ArcTerminalFontFamily,
                    color = if (state.isLoading) ArcTerminalAccent else Color(0xFF9EA4AF),
                )
            }

            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                if (state.entries.isEmpty() && !state.isLoading) {
                    item {
                        Text(
                            text = "Directory is empty.",
                            style = MaterialTheme.typography.bodyMedium,
                            fontFamily = ArcTerminalFontFamily,
                            color = Color(0xFF9EA4AF),
                        )
                    }
                }
                items(state.entries, key = { it.path }) { entry ->
                    FileRow(
                        entry = entry,
                        enabled = !state.isLoading,
                        onOpenDirectory = onOpenDirectory,
                        onRename = { renameTarget = entry },
                        onDelete = { deleteTarget = entry },
                        onDownload = onDownloadClick,
                    )
                }
                item {
                    Spacer(modifier = Modifier.height(20.dp))
                }
            }
        }
    }

    if (createFolderDialog) {
        NamePromptDialog(
            title = "Create folder",
            confirmLabel = "Create",
            onDismiss = { createFolderDialog = false },
            onConfirm = {
                createFolderDialog = false
                onCreateDirectory(it)
            },
        )
    }

    if (createFileDialog) {
        NamePromptDialog(
            title = "Create file",
            confirmLabel = "Create",
            onDismiss = { createFileDialog = false },
            onConfirm = {
                createFileDialog = false
                onCreateFile(it)
            },
        )
    }

    renameTarget?.let { entry ->
        NamePromptDialog(
            title = "Rename ${entry.name}",
            initialValue = entry.name,
            confirmLabel = "Rename",
            onDismiss = { renameTarget = null },
            onConfirm = {
                renameTarget = null
                onRename(entry, it)
            },
        )
    }

    deleteTarget?.let { entry ->
        AlertDialog(
            onDismissRequest = { deleteTarget = null },
            containerColor = Color(0xFF16191F),
            title = {
                Text(
                    text = "Delete ${entry.name}?",
                    fontFamily = ArcTerminalFontFamily,
                    color = Color(0xFFD8DADF),
                )
            },
            text = {
                Text(
                    text = if (entry.isDirectory) {
                        "Only empty folders can be removed."
                    } else {
                        "This permanently removes the file from the server."
                    },
                    fontFamily = ArcTerminalFontFamily,
                    color = Color(0xFF9EA4AF),
                )
            },
            confirmButton = {
                TextButton(onClick = {
                    deleteTarget = null
                    onDelete(entry)
                }) {
                    Text("Delete", fontFamily = ArcTerminalFontFamily, color = MaterialTheme.colorScheme.error)
                }
            },
            dismissButton = {
                TextButton(onClick = { deleteTarget = null }) {
                    Text("Cancel", fontFamily = ArcTerminalFontFamily, color = Color(0xFFD8DADF))
                }
            },
        )
    }
}

@Composable
private fun FilesToolbarButton(
    label: String,
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    onClick: () -> Unit,
    enabled: Boolean,
) {
    Row(
        modifier = Modifier
            .border(1.dp, Color(0xFF15171C), FilesSharpShape)
            .background(Color(0xFF16191F))
            .clickable(enabled = enabled, onClick = onClick)
            .padding(horizontal = 12.dp, vertical = 10.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, contentDescription = null, tint = Color(0xFFD8DADF), modifier = Modifier.size(18.dp))
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            fontFamily = ArcTerminalFontFamily,
            color = if (enabled) Color(0xFFD8DADF) else Color(0xFF70757F),
        )
    }
}

@Composable
private fun FileRow(
    entry: RemoteFileEntry,
    enabled: Boolean,
    onOpenDirectory: (RemoteFileEntry) -> Unit,
    onRename: () -> Unit,
    onDelete: () -> Unit,
    onDownload: (RemoteFileEntry) -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .border(1.dp, Color(0xFF15171C), FilesSharpShape)
            .background(Color(0xFF16191F))
            .padding(horizontal = 14.dp, vertical = 12.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Row(
            modifier = Modifier
                .weight(1f)
                .clickable(enabled = enabled) {
                    if (entry.isDirectory) {
                        onOpenDirectory(entry)
                    }
                },
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                imageVector = if (entry.isDirectory) Icons.Outlined.Folder else Icons.Outlined.Description,
                contentDescription = null,
                tint = if (entry.isDirectory) ArcTerminalAccent else Color(0xFFD8DADF),
                modifier = Modifier.size(20.dp),
            )
            Column {
                Text(
                    text = entry.name,
                    style = MaterialTheme.typography.titleMedium,
                    fontFamily = ArcTerminalFontFamily,
                    fontWeight = FontWeight.Bold,
                    color = Color(0xFFD8DADF),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    text = formatMeta(entry),
                    style = MaterialTheme.typography.bodySmall,
                    fontFamily = ArcTerminalFontFamily,
                    color = Color(0xFF9EA4AF),
                )
            }
        }
        Row {
            if (entry.isDirectory) {
                IconButton(onClick = { onOpenDirectory(entry) }, enabled = enabled) {
                    Icon(Icons.Outlined.FileOpen, contentDescription = "Open", tint = Color(0xFFD8DADF))
                }
            } else {
                IconButton(onClick = { onDownload(entry) }, enabled = enabled) {
                    Icon(Icons.Outlined.ArrowDownward, contentDescription = "Download", tint = Color(0xFFD8DADF))
                }
            }
            IconButton(onClick = onRename, enabled = enabled) {
                Icon(Icons.Outlined.Edit, contentDescription = "Rename", tint = Color(0xFFD8DADF))
            }
            IconButton(onClick = onDelete, enabled = enabled) {
                Icon(Icons.Outlined.Delete, contentDescription = "Delete", tint = Color(0xFFD8DADF))
            }
        }
    }
}

@Composable
private fun NamePromptDialog(
    title: String,
    initialValue: String = "",
    confirmLabel: String,
    onDismiss: () -> Unit,
    onConfirm: (String) -> Unit,
) {
    var value by rememberSaveable(title, initialValue) { mutableStateOf(initialValue) }
    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = Color(0xFF16191F),
        title = {
            Text(
                text = title,
                fontFamily = ArcTerminalFontFamily,
                color = Color(0xFFD8DADF),
            )
        },
        text = {
            OutlinedTextField(
                value = value,
                onValueChange = { value = it },
                singleLine = true,
                textStyle = MaterialTheme.typography.bodyMedium.copy(
                    fontFamily = ArcTerminalFontFamily,
                    color = Color(0xFFD8DADF),
                ),
                label = {
                    Text("Name", fontFamily = ArcTerminalFontFamily)
                },
            )
        },
        confirmButton = {
            TextButton(onClick = { onConfirm(value) }) {
                Text(confirmLabel, fontFamily = ArcTerminalFontFamily, color = ArcTerminalAccent)
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text("Cancel", fontFamily = ArcTerminalFontFamily, color = Color(0xFFD8DADF))
            }
        },
    )
}

private fun formatMeta(entry: RemoteFileEntry): String {
    if (entry.isDirectory) {
        return "directory"
    }
    val size = entry.sizeBytes ?: return "file"
    return when {
        size >= 1_048_576L -> String.format("%.1f MB", size / 1_048_576f)
        size >= 1024L -> String.format("%.1f KB", size / 1024f)
        else -> "$size B"
    }
}

private val RemoteFileEntrySaver = androidx.compose.runtime.saveable.listSaver<RemoteFileEntry?, Any?>(
    save = { entry ->
        if (entry == null) {
            emptyList()
        } else {
            listOf(entry.path, entry.name, entry.isDirectory, entry.sizeBytes, entry.modifiedEpochSeconds)
        }
    },
    restore = { raw ->
        if (raw.isEmpty()) {
            null
        } else {
            RemoteFileEntry(
                path = raw[0] as String,
                name = raw[1] as String,
                isDirectory = raw[2] as Boolean,
                sizeBytes = raw[3] as Long?,
                modifiedEpochSeconds = raw[4] as Long?,
            )
        }
    },
)
