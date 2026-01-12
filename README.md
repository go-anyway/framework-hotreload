# 热加载系统使用指南

## 概述

热加载系统 (`pkg/hotreload`) 负责管理配置变更的热更新，与配置中心 (`pkg/configcenter`) 和组件管理器 (`pkg/config/component_manager.go`) 配合工作。

## 架构设计

```
配置中心 (pkg/configcenter)
    ↓ (配置变更)
热加载管理器 (pkg/hotreload.Manager)
    ↓ (处理变更)
组件管理器 (pkg/config.ComponentManager) / 用户自定义重载器
```

## 快速开始

### 1. 系统配置热加载

系统配置热加载用于将配置中心的配置自动加载到目标结构体。

```go
import (
    "ai-api-market/pkg/configcenter"
    "ai-api-market/pkg/hotreload"
    "ai-api-market/internal/shared"
)

// 1. 创建配置中心客户端
client, err := configcenter.NewFromConfig(cfg)
if err != nil {
    log.Fatal("Failed to create config center client", zap.Error(err))
}
defer client.Close()

// 2. 创建热加载管理器
hotReloadManager := hotreload.NewManager()

// 3. 设置字段设置器（用于系统配置热加载）
hotReloadManager.SetFieldSetter(
    func(module, fieldPath, value string) error {
        return shared.SetFieldByPath(registry, module, fieldPath, value)
    },
    []string{"server.features.", "gateway.features."}, // 允许的前缀
)

// 4. 连接配置中心和热加载管理器
if err := client.ConnectHotReload(hotReloadManager); err != nil {
    log.Fatal("Failed to connect hot reload", zap.Error(err))
}

// 5. 加载初始配置
if err := client.LoadToTarget(
    func(module, fieldPath, value string) error {
        return shared.SetFieldByPath(registry, module, fieldPath, value)
    },
    []string{"server.features.", "gateway.features."},
); err != nil {
    log.Warn("Failed to load initial config", zap.Error(err))
}

// 6. 启动配置监听
if err := client.WatchChanges(); err != nil {
    log.Fatal("Failed to start watching", zap.Error(err))
}
```

### 2. 用户自定义配置热加载

用户自定义配置热加载用于实现自定义组件的热更新（如限流器、熔断器等）。

#### 2.1 实现重载器接口

```go
type MyReloader struct {
    componentManager *config.ComponentManager
}

func (r *MyReloader) Patterns() []string {
    return []string{
        "server.features.rate_limit.rate",
        "server.features.rate_limit.burst",
    }
}

func (r *MyReloader) Validate(key, value string) error {
    if strings.HasSuffix(key, ".rate") {
        rate, err := strconv.ParseFloat(value, 64)
        if err != nil || rate <= 0 {
            return fmt.Errorf("invalid rate: %s", value)
        }
    } else if strings.HasSuffix(key, ".burst") {
        burst, err := strconv.Atoi(value)
        if err != nil || burst <= 0 {
            return fmt.Errorf("invalid burst: %s", value)
        }
    }
    return nil
}

func (r *MyReloader) OnChange(key, oldValue, newValue string) error {
    // 更新组件
    parts := strings.Split(key, ".")
    name := parts[0] // "server" or "gateway"

    limiter := r.componentManager.GetRateLimiter(name)
    if limiter == nil {
        return fmt.Errorf("rate limiter not found: %s", name)
    }

    // 更新限流器配置
    // ...

    return nil
}
```

#### 2.2 注册重载器

```go
// 创建热加载管理器
hotReloadManager := hotreload.NewManager()

// 创建组件管理器
componentManager := config.GetComponentManager()

// 创建并注册重载器
reloader := &MyReloader{
    componentManager: componentManager,
}
if err := hotReloadManager.RegisterReloader(reloader); err != nil {
    log.Fatal("Failed to register reloader", zap.Error(err))
}

// 连接配置中心
if err := client.ConnectHotReload(hotReloadManager); err != nil {
    log.Fatal("Failed to connect hot reload", zap.Error(err))
}
```

### 3. 直接注册处理器

如果不需要验证，可以直接注册处理器：

```go
hotReloadManager := hotreload.NewManager()

hotReloadManager.RegisterHandler("server.features.*", func(key, oldValue, newValue string) error {
    // 处理配置变更
    log.Info("Config changed", zap.String("key", key))
    return nil
})
```

## 配置模式

支持以下配置模式：

- **精确匹配**: `"server.features.rate_limit.rate"`
- **通配符匹配**: `"server.features.*"` - 匹配所有以 `server.features.` 开头的配置
- **全局匹配**: `"*"` - 匹配所有配置

## 注意事项

1. **配置中心职责**: `pkg/configcenter` 只负责与配置中心通信，不处理热加载逻辑
2. **热加载职责**: `pkg/hotreload` 只负责处理配置变更，不管理组件
3. **组件管理职责**: `pkg/config/component_manager.go` 只负责管理组件，不处理配置变更
4. **配置加载职责**: `pkg/config` 只负责从文件、命令行、环境变量加载配置，不处理配置中心

## 迁移指南

### 从旧 API 迁移

**旧代码**:
```go
hotReloadManager := config.GetHotReloadManager()
hotReloadManager.InitializeConfigCenter(...)
```

**新代码**:
```go
hotReloadManager := hotreload.NewManager()
client, _ := configcenter.NewFromConfig(cfg)
client.ConnectHotReload(hotReloadManager)
client.WatchChanges()
```
