<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **claude-code-go** (23418 symbols, 59864 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/claude-code-go/context` | Codebase overview, check index freshness |
| `gitnexus://repo/claude-code-go/clusters` | All functional areas |
| `gitnexus://repo/claude-code-go/processes` | All execution flows |
| `gitnexus://repo/claude-code-go/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->

## Testing with VCR

API integration tests support a VCR (Video Cassette Recording) mode: record real API responses once, then replay them deterministically in CI without real API calls.

### Quick start

```bash
# 1. Record (requires API key)
VCR_RECORD=true ANTHROPIC_API_KEY=sk-... ANTHROPIC_MODEL=deepseek-v4-pro \
  go test ./internal/platform/api/anthropic/ -run TestVCR -v

# 2. Replay (no API key needed, zero network)
VCR_ENABLED=true ANTHROPIC_MODEL=deepseek-v4-pro \
  go test ./internal/platform/api/anthropic/ -run TestVCR -v
```

### Environment variables

| Variable | Purpose |
|----------|---------|
| `VCR_ENABLED=true` | Enable replay mode — read fixtures, no network calls |
| `VCR_RECORD=true` | Enable record mode — call real API, save fixtures |
| `CLAUDE_CODE_TEST_FIXTURES_ROOT` | Fixture directory; set to project root for stable paths |
| `ANTHROPIC_BASE_URL` | API gateway URL override (for proxies / relays) |
| `ANTHROPIC_MODEL` | Model name (default: `claude-sonnet-4-5-20250514`) |

### Writing a new VCR test

```go
import "github.com/sheepzhao/claude-code-go/internal/platform/vcr"

func TestMyFeature(t *testing.T) {
    if !vcr.Enabled() && !vcr.Recording() {
        t.Skip("set VCR_ENABLED=true or VCR_RECORD=true")
    }

    inner := anthropic.NewClient(anthropic.Config{
        APIKey:  os.Getenv("ANTHROPIC_API_KEY"),
        BaseURL: os.Getenv("ANTHROPIC_BASE_URL"),
    })

    wrapped := vcr.WrapModelClient("my-fixture-name", inner)
    stream, _ := wrapped.Stream(ctx, model.Request{
        Model: os.Getenv("ANTHROPIC_MODEL"),
        Messages: []message.Message{
            {Role: message.RoleUser, Content: []message.ContentPart{message.TextPart("hi")}},
        },
    })

    for evt := range stream {
        // Verify events
    }
}
```

### Fixture files

Fixtures are stored at `{CLAUDE_CODE_TEST_FIXTURES_ROOT}/fixtures/{name}-stream-{sha1}.json` and should be committed to the repository. On replay, existing fixtures are returned immediately; missing fixtures produce an error asking to re-record.