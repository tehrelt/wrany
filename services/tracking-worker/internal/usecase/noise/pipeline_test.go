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

// Regression for the production bug: a real walk that starts after a rest must
// have its distance counted. With multi-point smoothing the filtered position
// lags at the rest→motion transition and the steps collapse below the jitter
// radius; distance must be measured on the raw track, not the smoothed one.
func TestWalkAfterRestCountsDistance(t *testing.T) {
	cfg := domain.DefaultNoiseConfig() // SmoothingPoints=5 reproduces the lag
	cfg.StationaryMinDurationSec = 30
	pipeline := NewPipeline(cfg, nil)
	start := time.Now()

	const stepDeg = 0.000162 // ~18 m in latitude
	var points []domain.RawLocationPoint
	for i := 0; i < 4; i++ { // rest at A
		points = append(points, rawPoint(55.75, 37.62, 12, 0, "still", start.Add(time.Duration(i*12)*time.Second)))
	}
	for i := 0; i < 8; i++ { // walk away, ~18 m every 12 s
		points = append(points, rawPoint(
			55.75+float64(i+1)*stepDeg, 37.62, 10, 1.5, "walking",
			start.Add(time.Duration((4+i)*12)*time.Second),
		))
	}

	result := pipeline.ProcessBatch(nil, points, nil, start.Add(3*time.Minute))

	var total float64
	for _, p := range result.Accepted {
		total += p.DistanceDeltaM
	}
	// ~8 steps * ~18 m = ~144 m of real walking. The smoothing bug counted far less.
	assert.Greater(t, total, 120.0, "smoothing must not eat walking distance")
}

// Distance must never be attributed across a long time gap: two points an hour
// apart belong to different segments even if geographically close.
func TestLongGapDoesNotAccumulateDistance(t *testing.T) {
	pipeline := NewPipeline(domain.DefaultNoiseConfig(), nil)
	start := time.Now()
	points := []domain.RawLocationPoint{
		rawPoint(55.75, 37.62, 10, 0, "still", start),
		rawPoint(55.752, 37.62, 10, 0, "still", start.Add(60*time.Minute)), // ~222 m, 1 h later
	}

	result := pipeline.ProcessBatch(nil, points, nil, start.Add(2*time.Hour))

	require.Len(t, result.Accepted, 2)
	assert.Zero(t, result.Accepted[1].DistanceDeltaM,
		"distance must not be attributed across a long time gap")
}

// A teleport point must never contribute distance: it is rejected before the
// distance calculation and its DistanceDeltaM stays zero.
func TestTeleportPointHasZeroDistance(t *testing.T) {
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
	assert.Zero(t, result.Processed[1].DistanceDeltaM)
	// Outliers never reach Accepted, so they can never feed trip distance.
	for _, point := range result.Accepted {
		assert.False(t, point.IsOutlier)
	}
}

// Every processing result must be stamped with the current algorithm version so
// the reprocess path can later detect rows produced by an older version.
func TestAlgorithmVersionIsStamped(t *testing.T) {
	pipeline := NewPipeline(testNoiseConfig(), nil)
	start := time.Now()
	points := []domain.RawLocationPoint{
		rawPoint(55.75, 37.62, 10, 1.6, "walking", start),
		rawPoint(56.75, 37.62, 10, 2, "walking", start.Add(time.Second)), // outlier
		rawPoint(55.75, 37.62, 200, 0, "unknown", start.Add(2*time.Second)), // garbage accuracy
	}

	result := pipeline.ProcessBatch(nil, points, nil, start.Add(time.Minute))

	require.Len(t, result.Processed, 3)
	for _, point := range result.Processed {
		assert.Equal(t, domain.CurrentAlgorithmVersion, point.AlgorithmVersion,
			"accepted and rejected results alike must carry the algorithm version")
	}
}

// Regression: stationary retroactive zeroing (markStationaryWindow) must only
// touch points produced in the current batch. Points already accepted in a prior
// batch are passed in as read-only history and must never be mutated or re-emitted,
// otherwise their already-committed trip distance would silently change.
func TestStationaryDoesNotMutateHistory(t *testing.T) {
	pipeline := NewPipeline(testNoiseConfig(), nil)
	start := time.Now()

	// Prior batch: real walking, already processed and committed (distance > 0).
	movingLat, movingLon := 55.75, 37.62
	committedDistance := 14.0
	history := []domain.ProcessedLocationPoint{
		{
			UserID:   uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			DeviceID: uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			EventID:  "history-1",
			RawLat:   movingLat, RawLon: movingLon,
			FilteredLat: &movingLat, FilteredLon: &movingLon,
			AccuracyM:      10,
			DistanceDeltaM: committedDistance,
			IsAccepted:     true,
			RecordedAt:     start,
			AlgorithmVersion: domain.CurrentAlgorithmVersion,
		},
	}

	// New batch: user now stands still, GPS jitters around one spot.
	points := []domain.RawLocationPoint{
		rawPoint(55.7502, 37.62, 10, 0, "unknown", start.Add(20*time.Second)),
		rawPoint(55.75022, 37.62002, 10, 0, "unknown", start.Add(30*time.Second)),
		rawPoint(55.75019, 37.61998, 10, 0, "unknown", start.Add(40*time.Second)),
		rawPoint(55.75021, 37.62001, 10, 0, "unknown", start.Add(50*time.Second)),
	}

	result := pipeline.ProcessBatch(history, points, nil, start.Add(2*time.Minute))

	// History must be returned to the caller untouched and must not leak into the
	// batch result (which is the only thing that gets persisted/fed to detection).
	assert.Equal(t, committedDistance, history[0].DistanceDeltaM,
		"prior committed point distance must not be retroactively zeroed")
	for _, point := range result.Accepted {
		assert.NotEqual(t, "history-1", point.EventID,
			"history points must never be re-emitted in the batch result")
	}
	for _, point := range result.Processed {
		assert.NotEqual(t, "history-1", point.EventID)
	}
}
