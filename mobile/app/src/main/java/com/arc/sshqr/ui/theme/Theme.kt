package com.arc.sshqr.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.sp
import androidx.compose.ui.graphics.Color
import com.arc.sshqr.R

val ArcTerminalAccent = Color(0xFFD7FF00)
val ArcTerminalFontFamily = FontFamily(Font(R.font.jetbrainsmono_nerd_font_regular))

private val DarkPalette = darkColorScheme(
    primary = ArcTerminalAccent,
    onPrimary = Color(0xFF000000),
    secondary = Color(0xFF9FB800),
    tertiary = Color(0xFF9FB800),
    background = Color(0xFF000000),
    surface = Color(0xFF000000),
    onSurface = Color(0xFFFFFFFF),
    onSurfaceVariant = Color(0xFF8E9D82),
    onBackground = Color(0xFFFFFFFF),
    outline = Color(0xFF24331E),
    error = Color(0xFFFF7A5C),
)

private val ArcTypography = Typography(
    bodyLarge = TextStyle(
        fontFamily = ArcTerminalFontFamily,
        fontSize = 16.sp,
    ),
    bodyMedium = TextStyle(
        fontFamily = ArcTerminalFontFamily,
        fontSize = 14.sp,
    ),
    labelLarge = TextStyle(
        fontFamily = ArcTerminalFontFamily,
        fontSize = 14.sp,
    ),
    labelMedium = TextStyle(
        fontFamily = ArcTerminalFontFamily,
        fontSize = 12.sp,
    ),
    headlineSmall = TextStyle(
        fontFamily = ArcTerminalFontFamily,
        fontSize = 24.sp,
    ),
    headlineLarge = TextStyle(
        fontFamily = ArcTerminalFontFamily,
        fontSize = 32.sp,
    ),
)

@Composable
fun ArcSshTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = DarkPalette,
        typography = ArcTypography,
        content = content,
    )
}
