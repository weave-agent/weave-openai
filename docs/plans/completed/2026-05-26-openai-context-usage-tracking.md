# Improve OpenAI Context Usage Tracking

Status: Completed 2026-05-26.

## Overview
- Improve OpenAI provider context tracking by exposing richer usage data from OpenAI-compatible responses.
- Prepare the provider to participate in optional preflight token counting when SDK support exists.
- Keep request behavior stable while improving context and cache telemetry.

## Context (from discovery)
- Files/components involved:
  - `openai.go`
  - `openai_test.go`
  - `models.go`
  - root repo `utils/openaicompat` shared transport
- Related patterns found:
  - OpenAI provider delegates streaming to `utils/openaicompat.Stream`.
  - OpenAI-specific `modifyRequest` adds `reasoning_effort` for reasoning models.
  - Model registry includes context windows and max output tokens.
  - Shared transport currently emits prompt/completion usage, but cached-token detail needs root repo support.
- Dependencies identified:
  - Root repo SDK and `utils/openaicompat` changes should land first.
  - Exact OpenAI preflight counting may depend on provider API availability or tokenizer support.

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next.
- Make small, focused changes.
- **CRITICAL: every task MUST include new/updated tests** for code changes in that task.
- **CRITICAL: all tests must pass before starting next task** - no exceptions.
- **CRITICAL: update this plan file when scope changes during implementation**.
- Run tests after each change.
- Maintain backward compatibility.

## Testing Strategy
- **Unit tests**: required for every task.
- **E2E tests**: not expected for provider accounting changes.

## Progress Tracking
- Mark completed items with `[x]` immediately when done.
- Add newly discovered tasks with ➕ prefix.
- Document issues/blockers with ⚠️ prefix.
- Update plan if implementation deviates from original scope.
- Keep plan in sync with actual work done.

## What Goes Where
- **Implementation Steps** (`[ ]` checkboxes): tasks achievable within this codebase - code changes, tests, documentation updates.
- **Post-Completion** (no checkboxes): items requiring external action - manual testing, changes in consuming projects, deployment configs, third-party verifications.
- **Checkbox placement**: Checkboxes belong only in Task sections.

## Implementation Steps

### Task 1: Consume richer OpenAI-compatible usage fields
- [x] update dependency on root repo changes that parse cached prompt tokens
- [x] verify OpenAI provider maps cached-token detail through `sdk.ProviderUsage`
- [x] preserve existing streamed text, tool-call, and error behavior
- [x] write tests for provider usage event with cached-token detail
- [x] write tests for usage event without cached-token detail
- [x] run `go test ./...` - must pass before next task

### Task 2: Add OpenAI preflight count strategy if supported
- [x] evaluate whether current OpenAI API path supports a no-generation count request in this provider shape
- [x] if supported, implement `sdk.TokenCounter` using provider count endpoint or minimal-output request that does not stream user-visible content (not supported for current Chat Completions request shape)
- [x] if not supported, document fallback to agent calibrated heuristic and do not add fake exact counts
- [x] write tests for supported count success/error path or documented unsupported behavior
- [x] write tests preserving model override and reasoning-effort handling in count path if implemented (not applicable - no count path implemented)
- [x] run `go test ./...` - must pass before next task

### Task 3: Verify model budget metadata
- [x] review OpenAI model context windows and max output token metadata for budget accuracy
- [x] update stale model metadata only if verified against current provider docs
- [x] write tests for default model metadata and reasoning support flags
- [x] write tests for `SupportsXHigh` clamping expectations where relevant
- [x] run `go test ./...` - must pass before next task

### Task 4: Verify acceptance criteria
- [x] verify OpenAI usage telemetry includes cache read tokens when provider sends them
- [x] verify no unsupported exact-count claims are exposed
- [x] run full provider tests with `go test ./...`
- [x] run `golangci-lint run` or repo lint command
- [x] verify no prompts or credentials are logged in token accounting paths

### Task 5: Update documentation
- [x] update README or provider docs with token accounting support level: exact, tokenizer, or calibrated heuristic

## Technical Details
- Prefer honest count source reporting over pretending heuristic counts are exact.
- Do not introduce a required tokenizer dependency unless the root SDK/provider architecture accepts it.
- Keep OpenAI-compatible transport changes in the root repo shared utility.
- Model metadata was verified on 2026-05-26 against OpenAI API model documentation:
  - `gpt-5.5`: 1,050,000 context window, 128,000 max output tokens, reasoning with xhigh.
  - `gpt-5.4`: 1,050,000 context window, 128,000 max output tokens, reasoning with xhigh.
  - `gpt-5.2`: 400,000 context window, 128,000 max output tokens, reasoning with xhigh.
  - `gpt-4.1`: 1,047,576 context window, 32,768 max output tokens, no reasoning.
  - `o4-mini`: 200,000 context window, 100,000 max output tokens, reasoning.
  - `o3`: 200,000 context window, 100,000 max output tokens, reasoning.

## Post-Completion

**Manual verification**:
- Run an OpenAI session and inspect `agent.usage` cache/context telemetry.

**External system updates**:
- Agent repo should consume the richer usage fields through `sdk.ProviderUsage`.
