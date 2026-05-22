package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/weave-agent/weave/sdk"
	sdkmodel "github.com/weave-agent/weave/sdk/model"
	"github.com/weave-agent/weave/sdk/retry"
	"github.com/weave-agent/weave/utils/openaicompat"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubConfig struct {
	providers map[string]map[string]any
	sdk.NoopConfig
}

func (s *stubConfig) ExtensionConfig(scope, name string, target any) error {
	if scope != "providers" {
		return fmt.Errorf("unknown scope %q", scope)
	}

	section, ok := s.providers[name]
	if !ok {
		return nil
	}

	data, err := json.Marshal(section)
	if err != nil {
		return fmt.Errorf("marshal stub config: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal stub config: %w", err)
	}

	return nil
}

func newTestProvider(server *httptest.Server, model string) sdk.Provider {
	if model == "" {
		model = "gpt-5.5"
	}

	return &provider{
		client: server.Client(),
		retry:  retry.DefaultConfig(),
		config: openaicompat.ProviderConfig{
			BaseURL:       server.URL,
			APIKey:        "test-key",
			Model:         model,
			ModifyRequest: modifyRequest(model),
		},
	}
}

func getTestProvider(t *testing.T, cfg sdk.Config) *provider {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")

	p, err := sdk.GetProvider("openai", cfg)
	require.NoError(t, err)

	op, ok := p.(*provider)
	require.True(t, ok)

	return op
}

func collectEvents(t *testing.T, ch <-chan sdk.ProviderEvent) []sdk.ProviderEvent {
	t.Helper()

	var events []sdk.ProviderEvent

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return events
			}

			events = append(events, evt)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for events")
		}
	}
}

func sseChunk(delta openaicompat.ChunkDelta, finish *string) string {
	chunk := openaicompat.StreamChunk{
		ID: "chatcmpl-test",
		Choices: []struct {
			Index        int                     `json:"index"`
			Delta        openaicompat.ChunkDelta `json:"delta"`
			FinishReason *string                 `json:"finish_reason"`
		}{
			{Index: 0, Delta: delta, FinishReason: finish},
		},
	}
	data, _ := json.Marshal(chunk)

	return "data: " + string(data) + "\n"
}

func sseDone() string {
	return "data: [DONE]\n"
}

func sseStream(events ...string) string {
	return strings.Join(events, "") + "\n"
}

func setupServer(response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = fmt.Fprint(w, response)
	}))
}

func TestStream_TextResponse(t *testing.T) {
	stream := sseStream(
		sseChunk(openaicompat.ChunkDelta{Role: "assistant"}, nil),
		sseChunk(openaicompat.ChunkDelta{Content: "Hello!"}, nil),
		sseChunk(openaicompat.ChunkDelta{}, new("stop")),
		sseDone(),
	)

	server := setupServer(stream)
	defer server.Close()

	p := newTestProvider(server, "gpt-5.5")
	ch, err := p.Stream(context.Background(), sdk.ProviderRequest{
		Messages: []sdk.Message{sdk.NewUserMessage("hi")},
	})
	require.NoError(t, err)

	events := collectEvents(t, ch)

	var textParts []string

	for _, e := range events {
		if e.Type == sdk.ProviderEventTextDelta {
			textParts = append(textParts, e.Content.(string))
		}
	}

	assert.Equal(t, []string{"Hello!"}, textParts)
}

func TestStream_ToolCall(t *testing.T) {
	stream := sseStream(
		sseChunk(openaicompat.ChunkDelta{Role: "assistant"}, nil),
		sseChunk(openaicompat.ChunkDelta{
			ToolCalls: []openaicompat.ToolCallDelta{
				{Index: 0, ID: "call_abc", Type: "function", Function: &openaicompat.FunctionCallDelta{Name: "bash"}},
			},
		}, nil),
		sseChunk(openaicompat.ChunkDelta{
			ToolCalls: []openaicompat.ToolCallDelta{
				{Index: 0, Function: &openaicompat.FunctionCallDelta{Arguments: `{"command":"ls"}`}},
			},
		}, nil),
		sseChunk(openaicompat.ChunkDelta{}, new("tool_calls")),
		sseDone(),
	)

	server := setupServer(stream)
	defer server.Close()

	p := newTestProvider(server, "gpt-5.5")
	ch, err := p.Stream(context.Background(), sdk.ProviderRequest{
		Messages: []sdk.Message{sdk.NewUserMessage("run ls")},
	})
	require.NoError(t, err)

	events := collectEvents(t, ch)

	var toolCalls []sdk.ToolCall

	for _, e := range events {
		if e.Type == sdk.ProviderEventToolCall {
			toolCalls = append(toolCalls, e.Content.(sdk.ToolCall))
		}
	}

	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call_abc", toolCalls[0].ID)
	assert.Equal(t, "bash", toolCalls[0].Name)
	assert.Equal(t, map[string]any{"command": "ls"}, toolCalls[0].Arguments)
}

func TestStream_WithSystemPrompt(t *testing.T) {
	var receivedBody openaicompat.ChatRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, sseStream(
			sseChunk(openaicompat.ChunkDelta{Content: "ok"}, nil),
			sseChunk(openaicompat.ChunkDelta{}, new("stop")),
			sseDone(),
		))
	}))
	defer server.Close()

	p := newTestProvider(server, "gpt-5.5")
	ch, err := p.Stream(context.Background(), sdk.ProviderRequest{
		SystemPrompt: "You are helpful.",
		Messages:     []sdk.Message{sdk.NewUserMessage("hi")},
	})
	require.NoError(t, err)
	collectEvents(t, ch)

	require.NotEmpty(t, receivedBody.Messages)
	assert.Equal(t, "system", receivedBody.Messages[0].Role)
	assert.Equal(t, "You are helpful.", receivedBody.Messages[0].Content)
}

func TestStream_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":{"message":"Invalid API key","type":"invalid_request_error"}}`)
	}))
	defer server.Close()

	p := newTestProvider(server, "gpt-5.5")
	_, err := p.Stream(context.Background(), sdk.ProviderRequest{
		Messages: []sdk.Message{sdk.NewUserMessage("hi")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid API key")
}

func TestStream_UsesConfiguredRetryPolicy(t *testing.T) {
	var attempts int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"error":{"message":"try again","type":"server_error"}}`)

			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, sseStream(
			sseChunk(openaicompat.ChunkDelta{Content: "ok"}, nil),
			sseChunk(openaicompat.ChunkDelta{}, new("stop")),
			sseDone(),
		))
	}))
	defer server.Close()

	p := newTestProvider(server, "gpt-5.5").(*provider)
	p.retry = retry.Config{
		MaxRetries: 0,
		BaseDelay:  0,
		MaxDelay:   0,
		Multiplier: 1,
		Jitter:     retry.JitterNone,
	}

	_, err := p.Stream(context.Background(), sdk.ProviderRequest{
		Messages: []sdk.Message{sdk.NewUserMessage("hi")},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "max retries exceeded (0)")
	assert.Equal(t, 1, attempts)
}

func TestStream_WithTools(t *testing.T) {
	var receivedBody openaicompat.ChatRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, sseStream(
			sseChunk(openaicompat.ChunkDelta{Content: "ok"}, nil),
			sseChunk(openaicompat.ChunkDelta{}, new("stop")),
			sseDone(),
		))
	}))
	defer server.Close()

	p := newTestProvider(server, "gpt-5.5")
	ch, err := p.Stream(context.Background(), sdk.ProviderRequest{
		Messages: []sdk.Message{sdk.NewUserMessage("hi")},
		Tools: []sdk.ToolDef{
			{
				Name:        "bash",
				Description: "Run a command",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{"type": "string"},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	collectEvents(t, ch)

	require.Len(t, receivedBody.Tools, 1)
	assert.Equal(t, "bash", receivedBody.Tools[0].Function.Name)
}

func TestStream_MultipleToolCalls(t *testing.T) {
	stream := sseStream(
		sseChunk(openaicompat.ChunkDelta{Role: "assistant"}, nil),
		sseChunk(openaicompat.ChunkDelta{
			ToolCalls: []openaicompat.ToolCallDelta{
				{Index: 0, ID: "call_1", Type: "function", Function: &openaicompat.FunctionCallDelta{Name: "bash"}},
				{Index: 1, ID: "call_2", Type: "function", Function: &openaicompat.FunctionCallDelta{Name: "read"}},
			},
		}, nil),
		sseChunk(openaicompat.ChunkDelta{
			ToolCalls: []openaicompat.ToolCallDelta{
				{Index: 0, Function: &openaicompat.FunctionCallDelta{Arguments: `{"command":"ls"}`}},
				{Index: 1, Function: &openaicompat.FunctionCallDelta{Arguments: `{"path":"/tmp/file"}`}},
			},
		}, nil),
		sseChunk(openaicompat.ChunkDelta{}, new("tool_calls")),
		sseDone(),
	)

	server := setupServer(stream)
	defer server.Close()

	p := newTestProvider(server, "gpt-5.5")
	ch, err := p.Stream(context.Background(), sdk.ProviderRequest{
		Messages: []sdk.Message{sdk.NewUserMessage("do stuff")},
	})
	require.NoError(t, err)

	events := collectEvents(t, ch)

	var toolCalls []sdk.ToolCall

	for _, e := range events {
		if e.Type == sdk.ProviderEventToolCall {
			toolCalls = append(toolCalls, e.Content.(sdk.ToolCall))
		}
	}

	require.Len(t, toolCalls, 2)
	names := []string{toolCalls[0].Name, toolCalls[1].Name}
	assert.Contains(t, names, "bash")
	assert.Contains(t, names, "read")
}

func TestStream_DefaultModel(t *testing.T) {
	var receivedBody openaicompat.ChatRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, sseStream(
			sseChunk(openaicompat.ChunkDelta{Content: "ok"}, nil),
			sseChunk(openaicompat.ChunkDelta{}, new("stop")),
			sseDone(),
		))
	}))
	defer server.Close()

	p := newTestProvider(server, "")
	ch, err := p.Stream(context.Background(), sdk.ProviderRequest{
		Messages: []sdk.Message{sdk.NewUserMessage("hi")},
	})
	require.NoError(t, err)
	collectEvents(t, ch)

	assert.Equal(t, "gpt-5.5", receivedBody.Model)
}

func TestStream_SendsCorrectBaseURL(t *testing.T) {
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, sseStream(
			sseChunk(openaicompat.ChunkDelta{Content: "ok"}, nil),
			sseChunk(openaicompat.ChunkDelta{}, new("stop")),
			sseDone(),
		))
	}))
	defer server.Close()

	p := newTestProvider(server, "gpt-5.5")
	ch, err := p.Stream(context.Background(), sdk.ProviderRequest{
		Messages: []sdk.Message{sdk.NewUserMessage("hi")},
	})
	require.NoError(t, err)
	collectEvents(t, ch)

	assert.Equal(t, "/chat/completions", receivedPath)
}

func TestStream_WithThinkingLevel_SetsReasoningEffort(t *testing.T) {
	tests := []struct {
		level   sdkmodel.ThinkingLevel
		want    string
		wantNot string
	}{
		{sdkmodel.ThinkingOff, "", "reasoning_effort"},
		{sdkmodel.ThinkingMinimal, "low", ""},
		{sdkmodel.ThinkingLow, "low", ""},
		{sdkmodel.ThinkingMedium, "medium", ""},
		{sdkmodel.ThinkingHigh, "high", ""},
		{sdkmodel.ThinkingXHigh, "high", ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			var receivedBody map[string]any

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&receivedBody)

				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprint(w, sseStream(
					sseChunk(openaicompat.ChunkDelta{Content: "ok"}, nil),
					sseChunk(openaicompat.ChunkDelta{}, new("stop")),
					sseDone(),
				))
			}))
			defer server.Close()

			// Use a reasoning model so reasoning_effort is actually set.
			sdkmodel.ResetModelRegistry()
			sdkmodel.RegisterModel(sdkmodel.ModelDef{ID: "test-reasoning", Reasoning: true})
			t.Cleanup(func() { sdkmodel.ResetModelRegistry() })

			p := newTestProvider(server, "test-reasoning")
			ch, err := p.Stream(context.Background(), sdk.ProviderRequest{
				Messages: []sdk.Message{sdk.NewUserMessage("hi")},
			}, sdkmodel.WithThinkingLevel(tt.level))
			require.NoError(t, err)
			collectEvents(t, ch)

			if tt.want != "" {
				assert.Equal(t, tt.want, receivedBody["reasoning_effort"])
			}

			if tt.wantNot != "" {
				raw, _ := json.Marshal(receivedBody)
				assert.NotContains(t, string(raw), tt.wantNot)
			}
		})
	}
}

func TestProviderInit_WithCustomHTTPAndRetryConfig(t *testing.T) {
	cfg := &stubConfig{
		providers: map[string]map[string]any{
			"defaults": {
				"http": map[string]any{
					"response_header_timeout": "11s",
					"idle_conn_timeout":       "22s",
				},
				"retry": map[string]any{
					"max_retries": 2,
					"base_delay":  "3s",
					"max_delay":   "9s",
					"multiplier":  1.5,
					"jitter":      "none",
				},
			},
			"openai": {
				"model":    "gpt-custom",
				"base_url": "https://example.test/v1",
				"http": map[string]any{
					"response_header_timeout": "4s",
				},
				"retry": map[string]any{
					"max_retries": 7,
					"jitter":      "full",
				},
			},
		},
	}

	p := getTestProvider(t, cfg)

	require.NotNil(t, p.client)
	transport, ok := p.client.Transport.(*http.Transport)
	require.True(t, ok)

	assert.Equal(t, 4*time.Second, transport.ResponseHeaderTimeout)
	assert.Equal(t, 22*time.Second, transport.IdleConnTimeout)
	assert.Equal(t, 0*time.Second, p.client.Timeout)
	assert.Equal(t, 7, p.retry.MaxRetries)
	assert.Equal(t, 3*time.Second, p.retry.BaseDelay)
	assert.Equal(t, 9*time.Second, p.retry.MaxDelay)
	assert.InDelta(t, 1.5, p.retry.Multiplier, 0.0001)
	assert.Equal(t, retry.JitterFull, p.retry.Jitter)
	assert.Equal(t, "https://example.test/v1", p.config.BaseURL)
	assert.Equal(t, "test-key", p.config.APIKey)
	assert.Equal(t, "gpt-custom", p.config.Model)
	assert.NotNil(t, p.config.ModifyRequest)
}

func TestProviderInit_InvalidHTTPConfigFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")

	cfg := &stubConfig{
		providers: map[string]map[string]any{
			"openai": {
				"http": map[string]any{
					"response_header_timeout": "not-a-duration",
				},
			},
		},
	}

	_, err := sdk.GetProvider("openai", cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "openai: resolve HTTP config")
	assert.Contains(t, err.Error(), "invalid response_header_timeout")
}

func TestProviderInit_InvalidRetryConfigFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "test-key")

	cfg := &stubConfig{
		providers: map[string]map[string]any{
			"openai": {
				"retry": map[string]any{
					"jitter": "sometimes",
				},
			},
		},
	}

	_, err := sdk.GetProvider("openai", cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "openai: resolve retry config")
	assert.Contains(t, err.Error(), "invalid jitter")
}

func TestRegister(t *testing.T) {
	assert.True(t, sdk.ProviderRegistered("openai"))
}
