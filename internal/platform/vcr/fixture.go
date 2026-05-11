package vcr

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// errENOENT is returned when a fixture file is not found.
var errENOENT = errors.New("fixture not found")

// fixturePath returns the absolute path for a fixture with the given name and hash.
// Format: {fixtureRoot}/fixtures/{name}-{hash}.json
func fixturePath(name, hash string) string {
	return filepath.Join(FixtureRoot(), "fixtures", fmt.Sprintf("%s-%s.json", name, hash))
}

// hashInput creates a deterministic SHA1 hash (first 12 hex chars) of the input.
func hashInput(input any) (string, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("vcr: marshal input for hash: %w", err)
	}
	h := sha1.Sum(data)
	return fmt.Sprintf("%x", h[:6]), nil // first 12 hex chars (6 bytes)
}

// readFixture reads and deserializes a cached fixture by name and hash key.
func readFixture[T any](name, hash string) (*T, error) {
	path := fixturePath(name, hash)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errENOENT
		}
		return nil, fmt.Errorf("vcr: read fixture %s: %w", path, err)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("vcr: unmarshal fixture %s: %w", path, err)
	}
	return &result, nil
}

// writeFixture serializes and writes a fixture by name and hash key.
func writeFixture[T any](name, hash string, value T) error {
	path := fixturePath(name, hash)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("vcr: mkdir fixtures: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("vcr: marshal fixture: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("vcr: write fixture %s: %w", path, err)
	}
	return nil
}

// WithFixture manages generic fixture caching using a hash of the input as the key.
//
// When VCR_RECORD is set, it calls fn(), writes the result to a fixture file,
// and returns the result.
//
// When VCR replay is enabled, it reads the fixture file and returns the cached
// value. A missing fixture in replay mode returns an error.
//
// When VCR is disabled, it simply calls fn() and returns the result without
// any caching.
func WithFixture[T any](name string, input any, fn func() (T, error)) (T, error) {
	if !Recording() && !(Enabled() || ForceVCR()) {
		return fn()
	}

	hash, err := hashInput(input)
	if err != nil {
		var zero T
		return zero, err
	}

	if Recording() || ForceVCR() {
		// Record mode: call fn, save fixture
		result, err := fn()
		if err != nil {
			return result, err
		}
		if err := writeFixture(name, hash, result); err != nil {
			var zero T
			return zero, err
		}
		return result, nil
	}

	// Replay mode: read fixture
	cached, err := readFixture[T](name, hash)
	if err != nil {
		if errors.Is(err, errENOENT) {
			var zero T
			return zero, fmt.Errorf(
				"vcr: fixture missing for %s (hash=%s). "+
					"Re-run tests with VCR_RECORD=true, then commit the result. "+
					"Fixture path: %s", name, hash, fixturePath(name, hash),
			)
		}
		var zero T
		return zero, err
	}
	return *cached, nil
}

// MustFixture works like WithFixture but panics on error.
// Convenient for test helpers where errors indicate test setup failure.
func MustFixture[T any](name string, input any, fn func() (T, error)) T {
	result, err := WithFixture[T](name, input, fn)
	if err != nil {
		panic(fmt.Sprintf("vcr: MustFixture %s: %v", name, err))
	}
	return result
}
