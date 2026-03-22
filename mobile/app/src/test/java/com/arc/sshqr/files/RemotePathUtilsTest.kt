package com.arc.sshqr.files

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class RemotePathUtilsTest {

    @Test
    fun `clampToRoot keeps nested child path`() {
        assertEquals(
            "/home/arc/projects/demo",
            RemotePathUtils.clampToRoot("/home/arc/projects/demo"),
        )
    }

    @Test
    fun `clampToRoot rejects escaping above root`() {
        assertEquals(
            ARC_FILES_ROOT_PATH,
            RemotePathUtils.clampToRoot("/home/arc/../../etc"),
        )
    }

    @Test
    fun `parent returns null at root`() {
        assertNull(RemotePathUtils.parent(ARC_FILES_ROOT_PATH))
    }

    @Test
    fun `child appends sanitized segment`() {
        assertEquals(
            "/home/arc/docs/readme.txt",
            RemotePathUtils.child("/home/arc/docs", "readme.txt"),
        )
    }

    @Test(expected = IllegalArgumentException::class)
    fun `sanitizeSegment rejects path separators`() {
        RemotePathUtils.sanitizeSegment("../passwd")
    }

    @Test
    fun `sibling stays inside current parent`() {
        assertEquals(
            "/home/arc/docs/renamed.txt",
            RemotePathUtils.sibling("/home/arc/docs/original.txt", "renamed.txt"),
        )
    }
}
