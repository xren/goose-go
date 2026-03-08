package models

import (
	"context"
	"fmt"

	"goose-go/internal/auth/codex"
)

type Resolver interface {
	ListAvailable(ctx context.Context, providerID string) ([]Availability, error)
}

type codexLoader interface {
	Resolve(context.Context) (codex.Credentials, error)
}

type availabilityResolver struct {
	codex codexLoader
}

func NewResolver() Resolver {
	reader, _ := codex.NewReader()
	return &availabilityResolver{codex: reader}
}

func NewResolverWithCodex(loader codexLoader) Resolver {
	return &availabilityResolver{codex: loader}
}

func (r *availabilityResolver) ListAvailable(ctx context.Context, providerID string) ([]Availability, error) {
	if _, ok := GetProvider(providerID); !ok {
		return nil, fmt.Errorf("unknown provider %q", providerID)
	}
	models := ListModels(providerID)
	availability := make([]Availability, 0, len(models))
	available, reason := r.providerAvailable(ctx, providerID)
	for _, model := range models {
		availability = append(availability, Availability{Model: model, Available: available, Reason: reason})
	}
	return availability, nil
}

func (r *availabilityResolver) providerAvailable(ctx context.Context, providerID string) (bool, string) {
	switch ProviderID(providerID) {
	case ProviderOpenAICodex:
		if _, err := r.codex.Resolve(ctx); err != nil {
			return false, err.Error()
		}
		return true, ""
	default:
		return false, "unsupported provider"
	}
}
