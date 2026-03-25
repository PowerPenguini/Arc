package com.arc.sshqr

import org.junit.Assert.assertEquals
import org.junit.Test

class MainViewModelRoutingTest {

    @Test
    fun `preferred connection screen keeps session route`() {
        assertEquals(
            MainScreenRoute.Session,
            preferredConnectionScreen(MainScreenRoute.Session),
        )
    }

    @Test
    fun `preferred connection screen keeps files route`() {
        assertEquals(
            MainScreenRoute.Files,
            preferredConnectionScreen(MainScreenRoute.Files),
        )
    }

    @Test
    fun `preferred connection screen collapses other routes to menu`() {
        assertEquals(
            MainScreenRoute.Menu,
            preferredConnectionScreen(MainScreenRoute.Menu),
        )
        assertEquals(
            MainScreenRoute.Menu,
            preferredConnectionScreen(MainScreenRoute.Scan),
        )
        assertEquals(
            MainScreenRoute.Menu,
            preferredConnectionScreen(MainScreenRoute.Settings),
        )
    }
}
