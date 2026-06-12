package noise

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wrany/tracking-worker/internal/domain"
)

func rawPoint(lat, lon, accuracy, speed float64, activity string, at time.Time) domain.RawLocationPoint {
	return domain.RawLocationPoint{
		UserID:   uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		DeviceID: uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		EventID:  at.Format(time.RFC3339Nano), RecordedAt: at, ReceivedAt: at,
		Lat: lat, Lon: lon, AccuracyM: accuracy, SpeedMps: &speed,
		ActivityType: activity, Source: "test",
	}
}

func testNoiseConfig() domain.NoiseConfig {
	config := domain.DefaultNoiseConfig()
	config.StationaryMinDurationSec = 30
	config.StationaryWindowSec = 60
	config.StationaryMinPoints = 4
	config.SmoothingPoints = 1
	return config
}

func TestStationaryJitterDoesNotIncreaseDistance(t *testing.T) {
	pipeline := NewPipeline(testNoiseConfig(), nil)
	start := time.Now()
	points := []domain.RawLocationPoint{
		rawPoint(55.75, 37.62, 10, 0, "unknown", start),
		rawPoint(55.75003, 37.62, 10, 0, "unknown", start.Add(10*time.Second)),
		rawPoint(55.74998, 37.62002, 10, 0, "unknown", start.Add(20*time.Second)),
		rawPoint(55.75002, 37.61998, 10, 0, "unknown", start.Add(30*time.Second)),
	}

	result := pipeline.ProcessBatch(nil, points, nil, start.Add(time.Minute))

	require.Len(t, result.Accepted, 4)
	assert.True(t, result.Accepted[3].IsStationary)
	for _, point := range result.Accepted {
		assert.Zero(t, point.DistanceDeltaM)
	}
}

func TestRealWalkingIsAccepted(t *testing.T) {
	pipeline := NewPipeline(testNoiseConfig(), nil)
	start := time.Now()
	var points []domain.RawLocationPoint
	for index := 0; index < 5; index++ {
		points = append(points, rawPoint(
			55.75+float64(index)*0.00015, 37.62, 10, 1.6,
			"walking", start.Add(time.Duration(index)*10*time.Second),
		))
	}

	result := pipeline.ProcessBatch(nil, points, nil, start.Add(time.Minute))

	require.Len(t, result.Accepted, 5)
	assert.Greater(t, result.Accepted[4].DistanceDeltaM, 0.0)
	assert.False(t, result.Accepted[4].IsStationary)
}

func TestSingleTeleportRejected(t *testing.T) {
	pipeline := NewPipeline(testNoiseConfig(), nil)
	start := time.Now()
	points := []domain.RawLocationPoint{
		rawPoint(55.75, 37.62, 10, 0, "walking", start),
		rawPoint(56.75, 37.62, 10, 2, "walking", start.Add(time.Second)),
	}

	result := pipeline.ProcessBatch(nil, points, nil, start.Add(time.Minute))

	require.Len(t, result.Processed, 2)
	assert.True(t, result.Processed[1].IsOutlier)
	assert.False(t, result.Processed[1].IsAccepted)
	assert.Equal(t, domain.NoiseTeleport, result.Processed[1].NoiseReason)
}

func TestAccuracyBands(t *testing.T) {
	pipeline := NewPipeline(testNoiseConfig(), nil)
	start := time.Now()
	points := []domain.RawLocationPoint{
		rawPoint(55.75, 37.62, 40, 0, "unknown", start),
		rawPoint(55.75, 37.62, 60, 0, "unknown", start.Add(time.Second)),
		rawPoint(55.75, 37.62, 110, 0, "unknown", start.Add(2*time.Second)),
	}

	result := pipeline.ProcessBatch(nil, points, nil, start.Add(time.Minute))

	assert.True(t, result.Processed[0].IsAccepted)
	assert.Equal(t, domain.NoisePoorAccuracy, result.Processed[1].NoiseReason)
	assert.Equal(t, domain.NoiseGarbageAccuracy, result.Processed[2].NoiseReason)
}

func TestOutOfOrderPointsAreSorted(t *testing.T) {
	pipeline := NewPipeline(testNoiseConfig(), nil)
	start := time.Now()
	points := []domain.RawLocationPoint{
		rawPoint(55.7502, 37.62, 10, 1, "walking", start.Add(20*time.Second)),
		rawPoint(55.75, 37.62, 10, 1, "walking", start),
		rawPoint(55.7501, 37.62, 10, 1, "walking", start.Add(10*time.Second)),
	}

	result := pipeline.ProcessBatch(nil, points, nil, start.Add(time.Minute))

	require.Len(t, result.Processed, 3)
	assert.True(t, result.Processed[0].RecordedAt.Before(result.Processed[1].RecordedAt))
	assert.True(t, result.Processed[1].RecordedAt.Before(result.Processed[2].RecordedAt))
}

func TestLatePointIsStoredRejected(t *testing.T) {
	pipeline := NewPipeline(testNoiseConfig(), nil)
	start := time.Now()
	processedThrough := start.Add(10 * time.Second)

	result := pipeline.ProcessBatch(nil, []domain.RawLocationPoint{
		rawPoint(55.75, 37.62, 10, 0, "unknown", start),
	}, &processedThrough, start.Add(time.Minute))

	require.Len(t, result.Processed, 1)
	assert.Equal(t, domain.NoiseLateArrival, result.Processed[0].NoiseReason)
	assert.False(t, result.Processed[0].IsAccepted)
}
