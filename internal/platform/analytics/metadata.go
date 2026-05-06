package analytics

import "time"

// NewMetadata creates a Metadata with the given session ID and timestamp set
// to the current time. Labels is initialised as an empty map.
func NewMetadata(sessionID string) Metadata {
	return Metadata{
		Timestamp: time.Now(),
		SessionID: sessionID,
		Labels:    make(map[string]any),
	}
}

// WithLabel returns a copy of m with the label key set to val.
func (m Metadata) WithLabel(key string, val any) Metadata {
	dst := make(map[string]any, len(m.Labels)+1)
	for k, v := range m.Labels {
		dst[k] = v
	}
	dst[key] = val
	m.Labels = dst
	return m
}

// WithLabels returns a copy of m with all labels from vals added.
func (m Metadata) WithLabels(vals map[string]any) Metadata {
	dst := make(map[string]any, len(m.Labels)+len(vals))
	for k, v := range m.Labels {
		dst[k] = v
	}
	for k, v := range vals {
		dst[k] = v
	}
	m.Labels = dst
	return m
}
