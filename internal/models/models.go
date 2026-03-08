package models

import (
	"fmt"
	"sort"

	"goose-go/internal/provider"
)

type ProviderID string

type ModelID string

type ProviderSpec struct {
	ID           ProviderID `json:"id"`
	DisplayName  string     `json:"display_name"`
	DefaultModel ModelID    `json:"default_model"`
}

type ModelSpec struct {
	Provider      ProviderID `json:"provider"`
	ID            ModelID    `json:"id"`
	DisplayName   string     `json:"display_name"`
	ContextWindow int        `json:"context_window"`
}

type Availability struct {
	Model     ModelSpec `json:"model"`
	Available bool      `json:"available"`
	Reason    string    `json:"reason,omitempty"`
}

const ProviderOpenAICodex ProviderID = "openai-codex"

const (
	ModelGPT5Codex  ModelID = "gpt-5-codex"
	ModelGPT51Codex ModelID = "gpt-5.1-codex"
	ModelGPT52Codex ModelID = "gpt-5.2-codex"
	ModelGPT53Codex ModelID = "gpt-5.3-codex"
	ModelGPT54Codex ModelID = "gpt-5.4"
)

var providers = map[ProviderID]ProviderSpec{
	ProviderOpenAICodex: {
		ID:           ProviderOpenAICodex,
		DisplayName:  "OpenAI Codex",
		DefaultModel: ModelGPT5Codex,
	},
}

var modelsByProvider = map[ProviderID][]ModelSpec{
	ProviderOpenAICodex: {
		{Provider: ProviderOpenAICodex, ID: ModelGPT5Codex, DisplayName: "GPT-5 Codex", ContextWindow: 128000},
		{Provider: ProviderOpenAICodex, ID: ModelGPT51Codex, DisplayName: "GPT-5.1 Codex", ContextWindow: 128000},
		{Provider: ProviderOpenAICodex, ID: ModelGPT52Codex, DisplayName: "GPT-5.2 Codex", ContextWindow: 128000},
		{Provider: ProviderOpenAICodex, ID: ModelGPT53Codex, DisplayName: "GPT-5.3 Codex", ContextWindow: 128000},
		{Provider: ProviderOpenAICodex, ID: ModelGPT54Codex, DisplayName: "GPT-5.4", ContextWindow: 128000},
	},
}

func ListProviders() []ProviderSpec {
	ids := make([]string, 0, len(providers))
	for id := range providers {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	out := make([]ProviderSpec, 0, len(ids))
	for _, id := range ids {
		out = append(out, providers[ProviderID(id)])
	}
	return out
}

func GetProvider(id string) (ProviderSpec, bool) {
	provider, ok := providers[ProviderID(id)]
	return provider, ok
}

func ListModels(providerID string) []ModelSpec {
	models := modelsByProvider[ProviderID(providerID)]
	out := make([]ModelSpec, len(models))
	copy(out, models)
	return out
}

func FindModel(providerID string, modelID string) (ModelSpec, bool) {
	for _, spec := range modelsByProvider[ProviderID(providerID)] {
		if string(spec.ID) == modelID {
			return spec, true
		}
	}
	return ModelSpec{}, false
}

func DefaultModel(providerID string) (ModelSpec, bool) {
	provider, ok := providers[ProviderID(providerID)]
	if !ok {
		return ModelSpec{}, false
	}
	return FindModel(providerID, string(provider.DefaultModel))
}

func ResolveSelection(providerID string, modelID string) (ProviderSpec, ModelSpec, error) {
	provider, ok := GetProvider(providerID)
	if !ok {
		return ProviderSpec{}, ModelSpec{}, fmt.Errorf("unknown provider %q", providerID)
	}
	if modelID == "" {
		modelID = string(provider.DefaultModel)
	}
	model, ok := FindModel(providerID, modelID)
	if !ok {
		return ProviderSpec{}, ModelSpec{}, fmt.Errorf("unknown model %q for provider %q", modelID, providerID)
	}
	return provider, model, nil
}

func ToModelConfig(spec ModelSpec) provider.ModelConfig {
	return provider.ModelConfig{
		Provider:      string(spec.Provider),
		Model:         string(spec.ID),
		ContextWindow: spec.ContextWindow,
	}
}
