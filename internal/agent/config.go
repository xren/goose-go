package agent

import "goose-go/internal/provider"

func (a *Agent) SetModelConfig(model provider.ModelConfig) {
	a.config.Model = model
}
