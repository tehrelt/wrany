package com.androidtracker.tracking

import androidx.room.Room
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import java.time.Instant
import java.util.UUID

@RunWith(AndroidJUnit4::class)
class LocationQueueTest {

    private lateinit var db: LocationQueueDatabase
    private lateinit var dao: LocationQueueDao

    @Before
    fun setUp() {
        db = Room.inMemoryDatabaseBuilder(
            ApplicationProvider.getApplicationContext(),
            LocationQueueDatabase::class.java,
        ).allowMainThreadQueries().build()
        dao = db.dao()
    }

    @After
    fun tearDown() = db.close()

    private fun makePoint(id: String = UUID.randomUUID().toString()): LocationPoint {
        val now = Instant.now().toString()
        return LocationPoint(
            id = id,
            deviceId = "device-test",
            recordedAt = now,
            lat = 55.751244,
            lon = 37.618423,
            accuracyM = 10.0,
            createdAt = now,
            updatedAt = now,
        )
    }

    @Test
    fun insertAndReadPending() = runBlocking {
        val point = makePoint()
        dao.insert(point)
        val pending = dao.getPending()
        assertEquals(1, pending.size)
        assertEquals(point.id, pending[0].id)
        assertEquals("pending", pending[0].status)
    }

    @Test
    fun stableEventIdOnInsertIgnore() = runBlocking {
        val point = makePoint("stable-id-001")
        dao.insert(point)
        // Second insert with same id is ignored (IGNORE conflict strategy)
        dao.insert(point.copy(lat = 99.0))
        val pending = dao.getPending()
        assertEquals(1, pending.size)
        assertEquals(55.751244, pending[0].lat, 0.0001)
    }

    @Test
    fun markAckedRemovesFromPending() = runBlocking {
        val p1 = makePoint()
        val p2 = makePoint()
        dao.insert(p1)
        dao.insert(p2)
        val now = Instant.now().toString()
        dao.markAcked(listOf(p1.id, p2.id), now)
        assertEquals(0, dao.pendingCount())
    }

    @Test
    fun returnToPendingRestoresPoint() = runBlocking {
        val point = makePoint()
        dao.insert(point)
        val now = Instant.now().toString()
        // Simulate sending + network error
        dao.returnToPending(listOf(point.id), now)
        val pending = dao.getPending()
        assertEquals(1, pending.size)
        assertEquals(1, pending[0].attempts)
    }

    @Test
    fun markFailedSetsError() = runBlocking {
        val point = makePoint()
        dao.insert(point)
        val now = Instant.now().toString()
        dao.markFailed(point.id, "invalid_latitude", now)
        assertEquals(0, dao.pendingCount())
        assertEquals(1, dao.failedCount())
    }

    @Test
    fun clearFailedDeletesOnlyFailed() = runBlocking {
        val pending = makePoint()
        val failed = makePoint()
        dao.insert(pending)
        dao.insert(failed)
        dao.markFailed(failed.id, "invalid_latitude", Instant.now().toString())
        dao.clearFailed()
        assertEquals(1, dao.pendingCount())
        assertEquals(0, dao.failedCount())
    }

    @Test
    fun cleanupOldAcked() = runBlocking {
        val point = makePoint()
        dao.insert(point)
        val past = Instant.now().minusSeconds(8 * 24 * 3600).toString()
        // Mark as acked with old timestamp
        dao.markAcked(listOf(point.id), past)
        // Cleanup older than 7 days
        val cutoff = Instant.now().minusSeconds(7 * 24 * 3600).toString()
        dao.cleanupOld(cutoff)
        // Should be deleted
        val lastSync = dao.lastSyncTime()
        assertNull(lastSync)
    }

    @Test
    fun duplicatedAlsoAcked() = runBlocking {
        val p1 = makePoint()
        val p2 = makePoint()
        dao.insert(p1)
        dao.insert(p2)
        val now = Instant.now().toString()
        // Simulate backend: p1 accepted, p2 duplicated — both should be acked
        dao.markAcked(listOf(p1.id, p2.id), now)
        assertEquals(0, dao.pendingCount())
    }

    @Test
    fun getPendingRespectsBatchSize() = runBlocking {
        repeat(30) { dao.insert(makePoint()) }
        val batch = dao.getPending(20)
        assertEquals(20, batch.size)
    }

    @Test
    fun insertOrderPreserved() = runBlocking {
        val ids = List(5) { UUID.randomUUID().toString() }
        ids.forEach { dao.insert(makePoint(it)) }
        val pending = dao.getPending(5)
        // Oldest first (ORDER BY createdAt ASC)
        assertEquals(ids, pending.map { it.id })
    }
}
