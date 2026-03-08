package app

import (
	"strings"

	"goose-go/internal/models"
)

type LocalCommandResult struct {
	Name   string
	Output string
}

func LocalCommand(prompt string, providerName string, modelName string) (*LocalCommandResult, bool) {
	providerName = strings.TrimSpace(providerName)
	modelName = strings.TrimSpace(modelName)
	if providerName == "" {
		providerName = defaultProviderName
	}
	if modelName == "" {
		if _, spec, err := models.ResolveSelection(providerName, ""); err == nil {
			modelName = string(spec.ID)
		}
	}

	trimmed := strings.TrimSpace(prompt)
	switch trimmed {
	case "/model":
		return &LocalCommandResult{
			Name:   "model",
			Output: "provider: " + providerName + "\nmodel: " + modelName,
		}, true
	default:
		return nil, false
	}
}
