package vcr

import (
	"context"
	"fmt"
	"sort"

	"github.com/sheepzhao/claude-code-go/internal/core/message"
	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

// streamRecording is the fixture format for recorded streaming responses.
type streamRecording struct {
	Request streamRequest  `json:"request"`
	Events  []model.Event `json:"events"`
}

// streamRequest is the serializable form of a model request for fixture keying.
type streamRequest struct {
	Model    string                 `json:"model"`
	Messages []message.Message      `json:"messages"`
	Tools    []model.ToolDefinition `json:"tools,omitempty"`
	System   string                 `json:"system,omitempty"`
}

// WrapModelClient wraps a model.Client with VCR recording/replay of streaming
// responses. Unlike the HTTP transport Recorder (which is best for non-streaming
// calls), this wrapper records at the model.Event level, making it suitable for
// SSE streaming responses.
//
// # Example
//
//	import "github.com/sheepzhao/claude-code-go/internal/platform/vcr"
//
//	inner := anthropic.NewClient(cfg)
//	wrapped := vcr.WrapModelClient("my-fixture", inner)
//
//	stream, err := wrapped.Stream(ctx, model.Request{
//	    Model: os.Getenv("ANTHROPIC_MODEL"),
//	    Messages: []message.Message{...},
//	})
//
// # Environment (same for all vcr consumers)
//
//	VCR_RECORD=true  — record real API responses to fixtures/fixture-name-*.json
//	VCR_ENABLED=true — replay from fixtures, no network calls
//
// # Fixture file location
//
//	{CLAUDE_CODE_TEST_FIXTURES_ROOT}/fixtures/{fixture-name}-stream-{sha1hash}.json
//
// Record mode (VCR_RECORD=true):
//   - Calls inner Stream(), buffers ALL events from the channel into a slice
//   - Saves the event slice to a JSON fixture keyed by the dehydrated request
//   - Creates a new channel that re-yields the same events
//
// Replay mode (VCR_ENABLED=true):
//   - Reads the event slice from the JSON fixture
//   - Creates a channel that yields the events in order
//   - Returns immediately without making any network calls
//
// Passthrough mode (default):
//   - Delegates directly to the inner client
func WrapModelClient(name string, inner model.Client) model.Client {
	return &wrappedClient{name: name, inner: inner}
}

type wrappedClient struct {
	name  string
	inner model.Client
}

func (c *wrappedClient) Stream(ctx context.Context, req model.Request) (model.Stream, error) {
	if !Enabled() {
		return c.inner.Stream(ctx, req)
	}

	fixtureInput := c.buildFixtureInput(req)

	if Recording() {
		return c.recordStream(ctx, req, fixtureInput)
	}
	return c.replayStream(fixtureInput)
}

func (c *wrappedClient) buildFixtureInput(req model.Request) streamRequest {
	input := streamRequest{
		Model:    req.Model,
		Messages: append([]message.Message(nil), req.Messages...),
		System:   req.System,
	}
	if len(req.Tools) > 0 {
		input.Tools = append([]model.ToolDefinition(nil), req.Tools...)
		sort.Slice(input.Tools, func(i, j int) bool {
			return input.Tools[i].Name < input.Tools[j].Name
		})
	}
	return input
}

// streamFixtName is the fixture name prefix for streaming recordings.
func (c *wrappedClient) streamFixtName() string { return c.name + "-stream" }

func (c *wrappedClient) recordStream(
	ctx context.Context, req model.Request, input streamRequest,
) (model.Stream, error) {
	innerStream, err := c.inner.Stream(ctx, req)
	if err != nil {
		return nil, err
	}

	var events []model.Event
loop:
	for {
		select {
		case evt, ok := <-innerStream:
			if !ok {
				break loop
			}
			events = append(events, evt)
			if evt.Type == model.EventTypeError {
				break loop
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	recording := streamRecording{Request: input, Events: events}
	hash, err := hashInput(input)
	if err != nil {
		return nil, err
	}
	if err := writeFixture(c.streamFixtName(), hash, recording); err != nil {
		return nil, err
	}

	return sliceToStream(events), nil
}

func (c *wrappedClient) replayStream(input streamRequest) (model.Stream, error) {
	hash, err := hashInput(input)
	if err != nil {
		return nil, err
	}

	cached, err := readFixture[streamRecording](c.streamFixtName(), hash)
	if err != nil {
		return nil, fmt.Errorf(
			"vcr: fixture missing for %s (hash=%s). "+
				"Re-run tests with VCR_RECORD=true, then commit the result. "+
				"Fixture path: %s",
			c.name, hash, fixturePath(c.streamFixtName(), hash),
		)
	}

	return sliceToStream(cached.Events), nil
}

// sliceToStream converts a slice of events to a model.Stream (channel).
func sliceToStream(events []model.Event) model.Stream {
	ch := make(chan model.Event, len(events))
	for _, e := range events {
		ch <- e
	}
	close(ch)
	return ch
}

// Compile-time interface check.
var _ model.Client = (*wrappedClient)(nil)
