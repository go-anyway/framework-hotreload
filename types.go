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

// ConfigChangeHandler 配置变更处理器
// key: 配置键（如 "server.features.rate_limit.rate"）
// oldValue: 旧值
// newValue: 新值
// 返回错误时，配置变更不会生效，并记录错误日志
type ConfigChangeHandler func(key, oldValue, newValue string) error

// Reloader 配置重载器接口
// 用于实现自定义配置热加载逻辑
type Reloader interface {
	// Patterns 返回配置键模式列表（支持通配符，如 "server.features.*"）
	Patterns() []string

	// OnChange 配置变更回调
	// 当配置中心中的配置发生变更时，会调用此方法
	OnChange(key, oldValue, newValue string) error

	// Validate 验证配置值
	// 在应用配置变更前，会先调用此方法验证配置值的有效性
	Validate(key, value string) error
}

// FieldSetter 字段设置函数
// 用于将配置值设置到目标对象上
// module: 模块名称（如 "server", "gateway", "features"）
// fieldPath: 字段路径（如 "http.port", "features.rate_limit.rate"）
// value: 配置值
type FieldSetter func(module, fieldPath, value string) error
