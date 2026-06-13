package noise

import (
	"math"
	"sort"
	"time"

	"github.com/wrany/tracking-worker/internal/domain"
)

type LocationValidator interface {
	Validate(domain.RawLocationPoint) domain.NoiseReason
}

type OutlierDetector interface {
	Detect(previous *domain.ProcessedLocationPoint, current domain.RawLocationPoint, next *domain.RawLocationPoint) (float64, bool)
}

type LocationSmoother interface {
	Smooth([]domain.ProcessedLocationPoint, domain.RawLocationPoint) (float64, float64)
}

type StationaryDetector interface {
	Detect([]domain.ProcessedLocationPoint) bool
}

type MovementWindowAnalyzer interface {
	Analyze([]domain.ProcessedLocationPoint) bool
}

type BatchResult struct {
	Processed []domain.ProcessedLocationPoint
	Accepted  []domain.ProcessedLocationPoint
}

type Pipeline struct {
	cfg        domain.NoiseConfig
	validator  LocationValidator
	outlier    OutlierDetector
	smoother   LocationSmoother
	stationary StationaryDetector
}

func NewPipeline(cfg domain.NoiseConfig, smoother LocationSmoother) *Pipeline {
	if smoother == nil {
		smoother = WeightedMovingAverage{Points: cfg.SmoothingPoints}
	}
	return &Pipeline{
		cfg: cfg, validator: Validator{Config: cfg},
		outlier:    SpeedOutlierDetector{Config: cfg},
		smoother:   smoother,
		stationary: WindowStationaryDetector{Config: cfg},
	}
}

func (p *Pipeline) ProcessBatch(history []domain.ProcessedLocationPoint, raw []domain.RawLocationPoint, processedThrough *time.Time, now time.Time) BatchResult {
	points := append([]domain.RawLocationPoint(nil), raw...)
	sort.SliceStable(points, func(i, j int) bool {
		if points[i].RecordedAt.Equal(points[j].RecordedAt) {
			return points[i].EventID < points[j].EventID
		}
		return points[i].RecordedAt.Before(points[j].RecordedAt)
	})

	window := acceptedOnly(history)
	result := BatchResult{}
	for index := range points {
		point := points[index]
		var next *domain.RawLocationPoint
		if index+1 < len(points) {
			next = &points[index+1]
		}
		processed := p.processOne(window, point, next, processedThrough, now)
		result.Processed = append(result.Processed, processed)
		if processed.IsAccepted {
			window = append(window, processed)
			window = trimWindow(window, point.RecordedAt, p.cfg.StationaryWindowSec)
			result.Accepted = append(result.Accepted, processed)
			if processed.IsStationary {
				markStationaryWindow(&result, window[0].RecordedAt)
			}
		}
	}
	return result
}

func markStationaryWindow(result *BatchResult, since time.Time) {
	for index := range result.Accepted {
		if result.Accepted[index].RecordedAt.Before(since) {
			continue
		}
		result.Accepted[index].IsStationary = true
		result.Accepted[index].DistanceDeltaM = 0
		result.Accepted[index].NoiseReason = domain.NoiseStationary
		result.Accepted[index].StationarySince = &since
	}
	for index := range result.Processed {
		if result.Processed[index].RecordedAt.Before(since) || !result.Processed[index].IsAccepted {
			continue
		}
		result.Processed[index].IsStationary = true
		result.Processed[index].DistanceDeltaM = 0
		result.Processed[index].NoiseReason = domain.NoiseStationary
		result.Processed[index].StationarySince = &since
	}
}

func (p *Pipeline) processOne(history []domain.ProcessedLocationPoint, raw domain.RawLocationPoint, next *domain.RawLocationPoint, processedThrough *time.Time, now time.Time) domain.ProcessedLocationPoint {
	out := domain.ProcessedLocationPoint{
		UserID: raw.UserID, DeviceID: raw.DeviceID, EventID: raw.EventID,
		RawLat: raw.Lat, RawLon: raw.Lon, AccuracyM: raw.AccuracyM,
		SpeedMps: raw.SpeedMps, ActivityType: raw.ActivityType,
		ActivityConfidence: raw.ActivityConfidence,
		RecordedAt:         raw.RecordedAt, ReceivedAt: raw.ReceivedAt, ProcessedAt: now,
		AlgorithmVersion: domain.CurrentAlgorithmVersion,
	}
	if processedThrough != nil && !raw.RecordedAt.After(*processedThrough) {
		out.NoiseReason = domain.NoiseLateArrival
		return out
	}
	if reason := p.validator.Validate(raw); reason != domain.NoiseNone {
		out.NoiseReason = reason
		return out
	}

	var previous *domain.ProcessedLocationPoint
	if len(history) > 0 {
		previous = &history[len(history)-1]
	}
	implied, isOutlier := p.outlier.Detect(previous, raw, next)
	out.ImpliedSpeedMps = implied
	if isOutlier {
		out.IsOutlier = true
		out.NoiseReason = domain.NoiseTeleport
		return out
	}

	// Smoothing only feeds the stored filtered_* coordinates (used for display).
	// It must not mix points across a long gap, so the smoother sees only history
	// within SegmentMaxGapSec of this point.
	smoothHistory := trimWindow(history, raw.RecordedAt, p.cfg.SegmentMaxGapSec)
	lat, lon := p.smoother.Smooth(smoothHistory, raw)
	out.FilteredLat, out.FilteredLon = &lat, &lon
	out.IsAccepted = true

	if previous != nil {
		gap := raw.RecordedAt.Sub(previous.RecordedAt)
		switch {
		case gap > time.Duration(p.cfg.SegmentMaxGapSec)*time.Second:
			// New segment after a long gap: do not attribute distance across it.
			// This point becomes the next anchor (it is neither jitter nor
			// stationary), so distance restarts fresh on the other side of the gap.
			out.NoiseReason = domain.NoiseSegmentBreak
		default:
			// Distance and jitter are decided on the RAW track against the last
			// position-establishing anchor, not the immediately previous sample.
			// At high sample rates every step is shorter than the jitter radius,
			// so per-sample gating marks real walking as jitter and silently loses
			// the whole trip. Accumulating displacement from a fixed anchor keeps
			// the distance while still suppressing in-place GPS dithering.
			anchor := lastAnchor(history)
			if anchor == nil {
				anchor = previous
			}
			distance := HaversineM(anchor.RawLat, anchor.RawLon, raw.Lat, raw.Lon)
			radius := clamp((anchor.AccuracyM+raw.AccuracyM)/2, p.cfg.NoiseMinRadiusM, p.cfg.NoiseMaxRadiusM)
			if distance < radius {
				out.NoiseReason = domain.NoiseJitter
			} else {
				out.DistanceDeltaM = distance
			}
		}
	}

	candidateWindow := append(append([]domain.ProcessedLocationPoint(nil), history...), out)
	candidateWindow = trimWindow(candidateWindow, raw.RecordedAt, p.cfg.StationaryWindowSec)
	if p.stationary.Detect(candidateWindow) {
		out.IsStationary = true
		out.DistanceDeltaM = 0
		out.NoiseReason = domain.NoiseStationary
		since := candidateWindow[0].RecordedAt
		out.StationarySince = &since
	}
	return out
}

type Validator struct{ Config domain.NoiseConfig }

func (v Validator) Validate(point domain.RawLocationPoint) domain.NoiseReason {
	if point.RecordedAt.IsZero() || point.Lat < -90 || point.Lat > 90 ||
		point.Lon < -180 || point.Lon > 180 || point.AccuracyM < 0 {
		return domain.NoiseInvalidPoint
	}
	if point.AccuracyM > v.Config.GarbageAccuracyM {
		return domain.NoiseGarbageAccuracy
	}
	if point.AccuracyM > v.Config.UsableAccuracyM {
		return domain.NoisePoorAccuracy
	}
	return domain.NoiseNone
}

type SpeedOutlierDetector struct{ Config domain.NoiseConfig }

// Detect returns the implied speed from the previous anchor to current and
// whether current is a teleport outlier.
//
// A single high-speed sample is ambiguous: it can be a one-off GPS spike OR the
// onset of real travel faster than walking. Because the Android client currently
// always reports activity "unknown" (so maxSpeed falls back to the walking
// ceiling), treating every faster-than-walking sample as a teleport drops whole
// bus/car rides. We disambiguate with the next sample:
//   - speed within the activity ceiling                     -> accept;
//   - speed above the absolute vehicle ceiling              -> teleport (impossible);
//   - in between, the next sample keeps progressing away    -> real travel, accept;
//   - in between, the next sample snaps back to the anchor  -> spike, teleport.
func (d SpeedOutlierDetector) Detect(previous *domain.ProcessedLocationPoint, current domain.RawLocationPoint, next *domain.RawLocationPoint) (float64, bool) {
	if previous == nil {
		return 0, false
	}
	elapsed := current.RecordedAt.Sub(previous.RecordedAt).Seconds()
	if elapsed <= 0 {
		return 0, true
	}
	distPC := HaversineM(previous.RawLat, previous.RawLon, current.Lat, current.Lon)
	speed := distPC / elapsed
	if speed <= d.maxSpeed(current.ActivityType) {
		return speed, false
	}
	if speed > d.Config.VehicleMaxSpeedMps {
		return speed, true // physically impossible jump: always a teleport
	}
	if next == nil {
		return speed, false // cannot disprove sustained travel under the vehicle ceiling
	}
	// Sustained travel advances: next is at least as far from the anchor as current.
	// An out-and-back spike collapses back toward the anchor.
	distPN := HaversineM(previous.RawLat, previous.RawLon, next.Lat, next.Lon)
	return speed, distPN < distPC
}

func (d SpeedOutlierDetector) maxSpeed(activity string) float64 {
	switch activity {
	case "running":
		return d.Config.RunningMaxSpeedMps
	case "bicycle", "bike":
		return d.Config.BikeMaxSpeedMps
	case "vehicle", "automotive":
		return d.Config.VehicleMaxSpeedMps
	default:
		return d.Config.WalkingMaxSpeedMps
	}
}

type WeightedMovingAverage struct{ Points int }

func (s WeightedMovingAverage) Smooth(history []domain.ProcessedLocationPoint, current domain.RawLocationPoint) (float64, float64) {
	limit := s.Points - 1
	if limit < 0 {
		limit = 0
	}
	if len(history) > limit {
		history = history[len(history)-limit:]
	}
	weight := 1 / math.Max(current.AccuracyM, 1)
	latSum, lonSum, weightSum := current.Lat*weight, current.Lon*weight, weight
	for _, point := range history {
		lat, lon, ok := point.Coordinates()
		if !ok {
			continue
		}
		pointWeight := 1 / math.Max(point.AccuracyM, 1)
		latSum += lat * pointWeight
		lonSum += lon * pointWeight
		weightSum += pointWeight
	}
	return latSum / weightSum, lonSum / weightSum
}

type WindowStationaryDetector struct{ Config domain.NoiseConfig }

func (d WindowStationaryDetector) Detect(window []domain.ProcessedLocationPoint) bool {
	if len(window) < d.Config.StationaryMinPoints {
		return false
	}
	duration := window[len(window)-1].RecordedAt.Sub(window[0].RecordedAt)
	if duration < time.Duration(d.Config.StationaryMinDurationSec)*time.Second {
		return false
	}
	centerLat, centerLon, ok := window[0].Coordinates()
	if !ok {
		return false
	}
	for _, point := range window {
		lat, lon, pointOK := point.Coordinates()
		if !pointOK || HaversineM(centerLat, centerLon, lat, lon) > d.Config.StationaryRadiusM {
			return false
		}
		if point.ActivityType == "walking" || point.ActivityType == "running" {
			return false
		}
		speed := point.ImpliedSpeedMps
		if point.SpeedMps != nil {
			speed = *point.SpeedMps
		}
		if speed > d.Config.StationaryMaxSpeedMps {
			return false
		}
	}
	return true
}

type WindowMovementAnalyzer struct{ Config domain.NoiseConfig }

func (a WindowMovementAnalyzer) Analyze(window []domain.ProcessedLocationPoint) bool {
	good := 0
	for _, point := range window {
		if point.AccuracyM <= a.Config.GoodAccuracyM &&
			point.IsMovementEvidence(a.Config.MovementMinSpeedMps, a.Config.RunningMaxSpeedMps, a.Config.ActivityConfidence) {
			good++
		}
	}
	return good >= a.Config.MovementGoodPoints
}

// lastAnchor returns the most recent accepted point that established a real
// position: it skips jitter and stationary points so that a run of sub-radius
// steps is measured against the last point that actually moved the device,
// letting small steps accumulate into real displacement.
func lastAnchor(history []domain.ProcessedLocationPoint) *domain.ProcessedLocationPoint {
	for index := len(history) - 1; index >= 0; index-- {
		switch history[index].NoiseReason {
		case domain.NoiseJitter, domain.NoiseStationary:
			continue
		}
		return &history[index]
	}
	return nil
}

func acceptedOnly(points []domain.ProcessedLocationPoint) []domain.ProcessedLocationPoint {
	out := make([]domain.ProcessedLocationPoint, 0, len(points))
	for _, point := range points {
		if point.IsAccepted {
			out = append(out, point)
		}
	}
	return out
}

func trimWindow(points []domain.ProcessedLocationPoint, at time.Time, seconds int) []domain.ProcessedLocationPoint {
	cutoff := at.Add(-time.Duration(seconds) * time.Second)
	index := 0
	for index < len(points) && points[index].RecordedAt.Before(cutoff) {
		index++
	}
	return points[index:]
}

func clamp(value, minValue, maxValue float64) float64 {
	return math.Max(minValue, math.Min(maxValue, value))
}

func HaversineM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6_371_000.0
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	deltaPhi := (lat2 - lat1) * math.Pi / 180
	deltaLambda := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	return earthRadius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
