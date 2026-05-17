package openai

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/weave-agent/weave/sdk"
	"github.com/weave-agent/weave/sdk/model"
	"github.com/weave-agent/weave/utils/openaicompat"
)

// OpenAIConfig holds per-provider configuration for the OpenAI provider.
type OpenAIConfig struct {
	Model   string `json:"model" default:"gpt-5.5" env:"OPENAI_MODEL" description:"Model name"`
	BaseURL string `json:"base_url" default:"https://api.openai.com/v1" env:"OPENAI_BASE_URL" description:"API base URL"`
}

// AuthConfig holds authentication credentials for the OpenAI provider.
type AuthConfig struct {
	APIKey string `json:"api_key" env:"OPENAI_API_KEY" description:"API key"`
}

type provider struct {
	client *http.Client
	config openaicompat.ProviderConfig
}

func init() {
	sdk.RegisterProvider("openai", func(cfg sdk.Config, oc OpenAIConfig, a AuthConfig) (sdk.Provider, error) {
		if a.APIKey == "" {
			return nil, errors.New("openai: API key required (set OPENAI_API_KEY or add to ~/.weave/auth.json)")
		}

		return &provider{
			client: &http.Client{},
			config: openaicompat.ProviderConfig{
				BaseURL:       oc.BaseURL,
				APIKey:        a.APIKey,
				Model:         oc.Model,
				ModifyRequest: modifyRequest(oc.Model),
			},
		}, nil
	})
}

// modifyRequest sets OpenAI-specific fields on the request body map.
func modifyRequest(modelName string) func(body map[string]any, so *model.StreamOptions) {
	return func(body map[string]any, so *model.StreamOptions) {
		mdl := so.Model
		if mdl == "" {
			mdl = modelName
		}

		if so.ThinkingLevel == model.ThinkingOff {
			return
		}

		if m, ok := model.GetModelForProvider(mdl, "openai"); ok && !m.Reasoning {
			return
		}

		effortMap := map[model.ThinkingLevel]string{
			model.ThinkingMinimal: "low",
			model.ThinkingLow:     "low",
			model.ThinkingMedium:  "medium",
			model.ThinkingHigh:    "high",
			model.ThinkingXHigh:   "high",
		}

		if effort, ok := effortMap[so.ThinkingLevel]; ok {
			body["reasoning_effort"] = effort
		}
	}
}

func (p *provider) Stream(ctx context.Context, req sdk.ProviderRequest, opts ...model.StreamOption) (<-chan sdk.ProviderEvent, error) {
	ch, err := openaicompat.Stream(ctx, p.client, p.config, req, opts...)
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}

	return ch, nil
}
