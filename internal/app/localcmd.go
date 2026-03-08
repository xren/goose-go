package app

import "strings"

type LocalCommandResult struct {
	Name   string
	Output string
}

func LocalCommand(prompt string, providerName string, modelName string) (*LocalCommandResult, bool) {
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
