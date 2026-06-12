package com.androidtracker.tracking

import android.content.Context
import androidx.room.*

@Entity(tableName = "location_queue", indices = [Index("status")])
data class LocationPoint(
    @PrimaryKey val id: String,
    val deviceId: String,
    val recordedAt: String,
    val lat: Double,
    val lon: Double,
    val accuracyM: Double,
    val speedMps: Double? = null,
    val bearingDeg: Double? = null,
    val activityType: String? = null,
    val activityConfidence: Double? = null,
    val batteryLevel: Double? = null,
    val source: String = "android_tracker",
    val status: String = "pending",
    val attempts: Int = 0,
    val lastError: String? = null,
    val createdAt: String,
    val updatedAt: String,
)

@Dao
interface LocationQueueDao {
    @Insert(onConflict = OnConflictStrategy.IGNORE)
    suspend fun insert(point: LocationPoint)

    @Query("SELECT * FROM location_queue WHERE status = 'pending' ORDER BY createdAt ASC LIMIT :limit")
    suspend fun getPending(limit: Int = 20): List<LocationPoint>

    @Query("UPDATE location_queue SET status = 'acked', updatedAt = :now WHERE id IN (:ids)")
    suspend fun markAcked(ids: List<String>, now: String)

    @Query("UPDATE location_queue SET status = 'pending', attempts = attempts + 1, updatedAt = :now WHERE id IN (:ids)")
    suspend fun returnToPending(ids: List<String>, now: String)

    @Query("UPDATE location_queue SET status = 'failed', lastError = :reason, updatedAt = :now WHERE id = :id")
    suspend fun markFailed(id: String, reason: String, now: String)

    @Query("SELECT COUNT(*) FROM location_queue WHERE status = 'pending'")
    suspend fun pendingCount(): Int

    @Query("SELECT COUNT(*) FROM location_queue WHERE status = 'failed'")
    suspend fun failedCount(): Int

    @Query("SELECT MAX(updatedAt) FROM location_queue WHERE status = 'acked'")
    suspend fun lastSyncTime(): String?

    @Query("DELETE FROM location_queue WHERE status = 'acked' AND updatedAt < :cutoff")
    suspend fun cleanupOld(cutoff: String)

    @Query("DELETE FROM location_queue WHERE status = 'failed'")
    suspend fun clearFailed()
}

@Database(entities = [LocationPoint::class], version = 1, exportSchema = false)
abstract class LocationQueueDatabase : RoomDatabase() {
    abstract fun dao(): LocationQueueDao

    companion object {
        @Volatile private var instance: LocationQueueDatabase? = null

        fun getInstance(context: Context): LocationQueueDatabase =
            instance ?: synchronized(this) {
                instance ?: Room.databaseBuilder(
                    context.applicationContext,
                    LocationQueueDatabase::class.java,
                    "location_queue.db"
                ).build().also { instance = it }
            }
    }
}
