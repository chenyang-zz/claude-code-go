package growthbook

import (
	"time"

	"github.com/sheepzhao/claude-code-go/internal/platform/analytics"
)

// NewExposureLogger creates an ExposureLogger that bridges GrowthBook experiment
// exposure events to the analytics Emitter pipeline. Exposure events flow
// through the configured sink chain (ConsoleSink/DatadogSink/etc.).
//
// When the 1P event logging pipeline is fully bootstrapped, switch to
// NewFPExposureLogger for direct FirstPartyEventLogger integration.
func NewExposureLogger(emitter *analytics.Emitter) ExposureLogger {
	return &exposureBridge{emitter: emitter}
}

// exposureBridge implements ExposureLogger via the analytics Emitter.
type exposureBridge struct {
	emitter *analytics.Emitter
}

// LogExposure logs a GrowthBook experiment exposure through the analytics Emitter.
func (b *exposureBridge) LogExposure(feature string, data StoredExperimentData, attrs UserAttributes) {
	if b.emitter == nil {
		return
	}

	meta := analytics.Metadata{
		Timestamp: time.Now(),
		Labels: map[string]any{
			"feature":       feature,
			"experiment_id": data.ExperimentID,
			"variation_id":  data.VariationID,
		},
	}

	b.emitter.EmitRaw(meta, "growthbook.exposure", map[string]any{
		"experiment_id":  data.ExperimentID,
		"variation_id":   data.VariationID,
		"in_experiment":  data.InExperiment,
		"hash_attribute": data.HashAttribute,
		"hash_value":     data.HashValue,
		"device_id":      attrs.DeviceID,
		"session_id":     attrs.SessionID,
		"platform":       attrs.Platform,
	})
}
