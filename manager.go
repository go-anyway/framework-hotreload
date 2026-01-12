// Copyright 2025 zampo.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// @contact  zampo3380@gmail.com

package hotreload

import (
	"fmt"
	"strings"
	"sync"

	"github.com/go-anyway/framework-log"

	"go.uber.org/zap"
)

// Manager 热加载管理器
// 负责管理配置变更监听和组件热更新
type Manager struct {
	// 重载器列表
	reloaders []Reloader

	// 配置变更处理器（按模式索引）
	handlers map[string][]ConfigChangeHandler

	// 字段设置器（用于系统配置热加载）
	fieldSetter FieldSetter

	// 允许的配置前缀（用于系统配置热加载）
	allowedPrefixes []string

	mu sync.RWMutex
}

// NewManager 创建新的热加载管理器
func NewManager() *Manager {
	return &Manager{
		reloaders:       make([]Reloader, 0),
		handlers:        make(map[string][]ConfigChangeHandler),
		allowedPrefixes: make([]string, 0),
	}
}

// RegisterReloader 注册配置重载器
// 重载器用于实现自定义配置热加载逻辑（如限流器、熔断器等）
func (m *Manager) RegisterReloader(reloader Reloader) error {
	if m == nil || reloader == nil {
		return fmt.Errorf("manager or reloader is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.reloaders = append(m.reloaders, reloader)

	// 为每个模式注册处理器
	patterns := reloader.Patterns()
	for _, pattern := range patterns {
		if m.handlers[pattern] == nil {
			m.handlers[pattern] = make([]ConfigChangeHandler, 0)
		}
		m.handlers[pattern] = append(m.handlers[pattern], func(key, oldValue, newValue string) error {
			// 验证配置值
			if err := reloader.Validate(key, newValue); err != nil {
				return fmt.Errorf("validation failed for key %s: %w", key, err)
			}
			// 调用重载器
			return reloader.OnChange(key, oldValue, newValue)
		})
	}

	log.Info("Config reloader registered",
		zap.Int("pattern_count", len(patterns)),
		zap.Strings("patterns", patterns))

	return nil
}

// RegisterHandler 注册配置变更处理器
// 用于直接处理配置变更，不经过重载器
func (m *Manager) RegisterHandler(pattern string, handler ConfigChangeHandler) error {
	if m == nil || handler == nil {
		return fmt.Errorf("manager or handler is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.handlers[pattern] == nil {
		m.handlers[pattern] = make([]ConfigChangeHandler, 0)
	}
	m.handlers[pattern] = append(m.handlers[pattern], handler)

	return nil
}

// SetFieldSetter 设置字段设置器
// 用于系统配置热加载（将配置中心的配置加载到目标结构体）
func (m *Manager) SetFieldSetter(setter FieldSetter, allowedPrefixes []string) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.fieldSetter = setter
	m.allowedPrefixes = allowedPrefixes
}

// HandleChange 处理配置变更
// 由配置中心调用，当配置发生变更时触发
func (m *Manager) HandleChange(key, oldValue, newValue string) error {
	if m == nil {
		return fmt.Errorf("manager is nil")
	}

	m.mu.RLock()
	// 收集所有匹配的处理器
	matchedHandlers := make([]ConfigChangeHandler, 0)

	// 1. 匹配精确模式
	if handlers, ok := m.handlers[key]; ok {
		matchedHandlers = append(matchedHandlers, handlers...)
	}

	// 2. 匹配通配符模式
	for pattern, handlers := range m.handlers {
		if pattern != key && matchPattern(pattern, key) {
			matchedHandlers = append(matchedHandlers, handlers...)
		}
	}

	// 3. 系统配置热加载（如果设置了字段设置器）
	fieldSetter := m.fieldSetter
	allowedPrefixes := m.allowedPrefixes
	m.mu.RUnlock()

	// 检查是否匹配允许的前缀
	if fieldSetter != nil && len(allowedPrefixes) > 0 {
		for _, prefix := range allowedPrefixes {
			if hasPrefix(key, prefix) {
				matchedHandlers = append(matchedHandlers, func(k, oldVal, newVal string) error {
					return m.handleSystemConfig(k, oldVal, newVal, fieldSetter)
				})
				break
			}
		}
	}

	// 调用所有匹配的处理器
	for _, handler := range matchedHandlers {
		if err := handler(key, oldValue, newValue); err != nil {
			log.Error("Failed to handle config change",
				zap.String("key", key),
				zap.String("old_value", oldValue),
				zap.String("new_value", newValue),
				zap.Error(err))
			return err
		}
	}

	return nil
}

// handleSystemConfig 处理系统配置热加载
func (m *Manager) handleSystemConfig(key, oldValue, newValue string, setter FieldSetter) error {
	// 检查值是否真的变化了，如果没有变化则跳过处理
	if oldValue == newValue {
		return nil
	}

	// 解析配置键，格式：module.field.path
	// 例如：server.features.rate_limit.rate -> module="server", fieldPath="features.rate_limit.rate"
	parts := splitKey(key)
	if len(parts) < 2 {
		return fmt.Errorf("invalid config key format: %s", key)
	}

	module := parts[0]
	fieldPath := joinKey(parts[1:])

	if err := setter(module, fieldPath, newValue); err != nil {
		return fmt.Errorf("failed to set field %s.%s: %w", module, fieldPath, err)
	}

	log.Info("System config updated via hot-reload",
		zap.String("key", key),
		zap.String("module", module),
		zap.String("field_path", fieldPath),
		zap.String("old_value", oldValue),
		zap.String("new_value", newValue))

	return nil
}

// hasPrefix 检查配置键是否匹配前缀（支持通配符）
func hasPrefix(key, prefix string) bool {
	return matchPattern(prefix+"*", key) || strings.HasPrefix(key, prefix)
}

// splitKey 分割配置键
func splitKey(key string) []string {
	return strings.Split(key, ".")
}

// joinKey 连接配置键
func joinKey(parts []string) string {
	return strings.Join(parts, ".")
}

// matchPattern 匹配配置键模式（避免依赖 pkg/config，防止循环导入）
// 支持通配符：如 "server.features.*" 匹配 "server.features.rate_limit.rate"
func matchPattern(pattern, key string) bool {
	if pattern == "" {
		return false
	}

	// 精确匹配
	if pattern == key {
		return true
	}

	// 通配符匹配：pattern 以 ".*" 结尾
	if len(pattern) > 2 && pattern[len(pattern)-2:] == ".*" {
		prefix := pattern[:len(pattern)-2]
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}

	// 通配符匹配：pattern 包含 "*"
	if strings.Contains(pattern, "*") {
		// 简单的通配符匹配：支持单级通配符
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			if strings.HasPrefix(key, prefix) && strings.HasSuffix(key, suffix) {
				// 检查中间部分不包含额外的分隔符（单级通配符）
				middle := key[len(prefix) : len(key)-len(suffix)]
				return !strings.Contains(middle, ".")
			}
		}
	}

	return false
}
