package core

import (
	"strings"
	"sync"
)

type ModelManager struct {
	Config *Config
}

var (
	managerInstance *ModelManager
	managerOnce     sync.Once
)

func GetModelManager() *ModelManager {
	managerOnce.Do(func() {
		managerInstance = &ModelManager{Config: GetConfig()}
	})
	return managerInstance
}

func (m *ModelManager) MapClaudeModelToOpenAI(claudeModel string) string {
	// If it's already an OpenAI model, return as-is
	if len(claudeModel) >= 4 && (claudeModel[:4] == "gpt-" || claudeModel[:3] == "o1-") {
		return claudeModel
	}

	// If it's other supported models (ARK/Doubao/DeepSeek), return as-is
	if len(claudeModel) >= 3 && (claudeModel[:3] == "ep-" || claudeModel[:8] == "doubao-" || claudeModel[:9] == "deepseek-") {
		return claudeModel
	}

	// Map based on model naming patterns
	modelLower := strings.ToLower(claudeModel)
	if strings.Contains(modelLower, "haiku") {
		return m.Config.SmallModel
	} else if strings.Contains(modelLower, "sonnet") {
		return m.Config.MiddleModel
	} else if strings.Contains(modelLower, "opus") {
		return m.Config.BigModel
	}

	// Default to big model for unknown models
	return m.Config.BigModel
}
