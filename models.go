package openai

import (
	"github.com/weave-agent/weave/sdk/model"
)

func init() {
	RegisterModels()
}

// RegisterModels registers OpenAI model definitions in the global model registry.
func RegisterModels() {
	model.RegisterModel(model.ModelDef{
		ID: "gpt-5.5", Provider: "openai",
		DisplayName: "GPT-5.5", Reasoning: true, SupportsXHigh: true,
		ContextWindow: 1050000, MaxTokens: 128000, Default: true,
	})
	model.RegisterModel(model.ModelDef{
		ID: "gpt-5.4", Provider: "openai",
		DisplayName: "GPT-5.4", Reasoning: true, SupportsXHigh: true,
		ContextWindow: 272000, MaxTokens: 128000,
	})
	model.RegisterModel(model.ModelDef{
		ID: "gpt-5.2", Provider: "openai",
		DisplayName: "GPT-5.2", Reasoning: true, SupportsXHigh: true,
		ContextWindow: 400000, MaxTokens: 128000,
	})
	model.RegisterModel(model.ModelDef{
		ID: "gpt-4.1", Provider: "openai",
		DisplayName: "GPT-4.1", ContextWindow: 1047576, MaxTokens: 32768,
	})
	model.RegisterModel(model.ModelDef{
		ID: "o4-mini", Provider: "openai",
		DisplayName: "o4-mini", Reasoning: true,
		ContextWindow: 200000, MaxTokens: 100000,
	})
	model.RegisterModel(model.ModelDef{
		ID: "o3", Provider: "openai",
		DisplayName: "o3", Reasoning: true,
		ContextWindow: 200000, MaxTokens: 100000,
	})
}
