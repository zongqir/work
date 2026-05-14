# AI 可执行改造说明：通知系统多规则支持

## 背景

当前通知系统的核心模型是：

- 一个 `tenant + message_type` 对应一套配置
- 这套配置只包含一个 `filter`
- 这套配置只包含一个 `channel`
- 聚合调度水位只按 `tenant + message_type` 维护

本次要改成：

- 一个 `tenant + message_type` 可以对应多条规则
- 每条规则独立决定：
  - `realtime_enabled`
  - `aggregate_enabled`
  - `aggregate_period_minutes`
  - `filter`
  - `channel`

外部复杂配置不在本仓库内建模。本仓库只接收一组已经“打平”的规则并执行。

## 本次目标

本次改造目标只有 4 个：

1. 让运行时支持 `tenant + message_type -> []rule`
2. 让 `Dispatcher` 按规则逐条执行实时/聚合
3. 让 `DispatchMessage` 带上 `rule_id`
4. 让 `Processor` 和 `AggregateScheduler` 都按 `rule_id` 工作

## 非目标

本次明确不做这些事情：

- 不做复杂规则引擎
- 不做规则优先级
- 不做“首条命中后停止”
- 不做规则互斥
- 不做规则冲突检测
- 不做管理台改造
- 不做本地默认配置体系升级
- 不做 capability/schema 的完整重构

换句话说，本次只做“运行时多规则执行”，不做“配置治理平台”。

## 行为定义

### 1. 实时行为

- 一个事件到来后，加载这个 `tenant + message_type` 对应的规则集
- 遍历所有 `realtime_enabled == true` 的规则
- 对每条规则单独解析 `filter`
- 对每条规则单独调用业务 handler 的 `Evaluate(...)`
- 命中的规则各自产生一条 `DispatchMessage`
- 多条规则可同时命中，同时发送

### 2. 聚合行为

- 聚合调度器加载所有规则集
- 只处理 `aggregate_enabled == true && aggregate_period_minutes > 0` 的规则
- 每条规则独立计算自己的聚合窗口
- 每条规则独立调用业务 handler 的 `Aggregate(...)`
- 每条规则独自产生一条 `DispatchMessage`
- 每条规则独立维护自己的 watermark

### 3. 投递行为

- `Processor` 收到消息后，必须根据 `tenant_id + message_type + rule_id` 找到唯一规则
- 发送时使用这条规则自己的 `channel`
- 不再允许只按 `tenant + message_type` 找唯一配置

### 4. 配置变更行为

第一版不实现强版本一致性。

但消息体中要预留：

- `rule_id`
- `config_version`

其中：

- `rule_id` 必须用于运行时定位规则
- `config_version` 先作为透传字段和排障字段使用

## 总体实现原则

1. 保持业务 handler 契约不变
2. 不把多规则逻辑塞到 handler wrapper
3. 多规则主逻辑放在 `dispatcher/scheduler/processor`
4. 运行时统一消费“打平后的规则”

## 数据结构变更

## 1. 新增运行时规则结构

新增文件：

- `notification/code/config/rules.go`

新增结构：

```go
package config

import (
    "encoding/json"
    "work/notification/code/internal/render"
)

type ExecutableRule struct {
    RuleID                 string
    SourceRuleID           string
    TenantID               string
    MessageType            string
    ConfigVersion          string
    RealtimeEnabled        bool
    AggregateEnabled       bool
    AggregatePeriodMinutes int
    Filter                 json.RawMessage
    Channel                render.ChannelPolicy
}

type MessageRuleSet struct {
    TenantID      string
    MessageType   string
    ConfigVersion string
    Rules         []ExecutableRule
}
```

要求：

- `RuleID` 不能为空
- `MessageType` 不能为空
- `TenantID` 在运行时必须有值
- `Rules` 可为空，但不能为 `nil` 时造成 panic

## 2. 修改 DispatchMessage

修改文件：

- [notification/code/pkg/notification/contract/contract.go](/C:/Users/Administrator/code/work/notification/code/pkg/notification/contract/contract.go:37)

将 `DispatchMessage` 改为：

```go
type DispatchMessage struct {
    IdempotencyKey string       `json:"idempotency_key"`
    TenantID       string       `json:"tenant_id"`
    MessageType    string       `json:"message_type"`
    RuleID         string       `json:"rule_id"`
    ConfigVersion  string       `json:"config_version,omitempty"`
    Source         string       `json:"source"`
    RetryCount     int          `json:"retry_count"`
    CreatedAt      time.Time    `json:"created_at"`
    ExpectedSendAt time.Time    `json:"expected_send_at"`
    ExpireAt       time.Time    `json:"expire_at"`
    BizVars        TemplateVars `json:"biz_vars"`
}
```

要求：

- `RuleID` 在多规则路径下必须写入
- `validate` 时要求 `RuleID` 非空

## 3. 修改聚合水位结构

修改文件：

- [notification/code/internal/dao/aggregate_watermark.go](/C:/Users/Administrator/code/work/notification/code/internal/dao/aggregate_watermark.go:1)

改为：

```go
type AggregateWatermark struct {
    TenantID      string
    MessageType   string
    RuleID        string
    LastWindowEnd time.Time
    UpdatedAt     time.Time
}

type AggregateWatermarkStore interface {
    LastWindowEnd(ctx context.Context, tenantID, messageType, ruleID string) (time.Time, error)
    SaveWindowEnd(ctx context.Context, tenantID, messageType, ruleID string, windowEnd time.Time) error
}
```

## 加载接口变更

## 1. 新增 RuleSetLoader

新增文件：

- `notification/code/config/rule_loader.go`

新增接口：

```go
package config

import "context"

type RuleSetLoader interface {
    LoadRuleSet(ctx context.Context, tenantID, messageType string) (*MessageRuleSet, error)
    LoadAllRuleSets(ctx context.Context) (map[string]map[string]MessageRuleSet, error)
}
```

说明：

- 这里不要求你实现真实外部接口调用
- 只要求为运行时建立明确边界
- 调用方不应该再依赖单个 `MessageConfig`

## 2. 兼容策略

本次改造允许保留旧的 `config.MessageConfig`、`LoadMessageConfig` 等结构和函数，但：

- 新的 `Dispatcher`
- 新的 `Processor`
- 新的 `AggregateScheduler`

不得再依赖旧单配置模型。

如果为了过渡需要保留旧文件，请在新逻辑中只依赖 `MessageRuleSet`。

## Dispatcher 修改要求

修改文件：

- [notification/code/internal/dispatch/dispatcher.go](/C:/Users/Administrator/code/work/notification/code/internal/dispatch/dispatcher.go:1)

## 1. 字段调整

将 `Dispatcher` 的加载依赖由：

```go
LoadAll func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
```

改为：

```go
RuleSets config.RuleSetLoader
```

允许保留原字段用于兼容，但新逻辑不得再使用它。

## 2. 新增私有辅助函数

新增类似函数：

```go
func (d *Dispatcher) loadRuleSet(ctx context.Context, tenantID, messageType string) (*config.MessageRuleSet, error)
func (d *Dispatcher) sendRealtimeForRule(ctx context.Context, spec contract.MessageTypeSpec, realtime contract.RealtimeEvaluator, rule config.ExecutableRule, event any) error
func (d *Dispatcher) sendAggregateForRule(ctx context.Context, spec contract.MessageTypeSpec, aggregate contract.AggregateProvider, rule config.ExecutableRule, windowStart, windowEnd time.Time) (bool, error)
```

函数名可以不同，但职责必须存在。

## 3. SendRealtime 行为要求

`SendRealtime(...)` 必须改成：

1. 解析 handler
2. 加载 `MessageRuleSet`
3. 遍历所有 `RealtimeEnabled` 规则
4. 每条规则独立解析 `filter`
5. 每条规则独立调用 `Evaluate(...)`
6. 每条规则命中后独立发布消息

注意：

- 某条规则未命中，不影响其他规则
- 某条规则返回业务错误，整次调用返回错误
- 规则为空时直接返回 `nil`

## 4. SendAggregate 行为要求

`SendAggregate(...)` 必须改成：

1. 解析 handler
2. 加载 `MessageRuleSet`
3. 遍历所有 `AggregateEnabled` 规则
4. 每条规则独立解析 `filter`
5. 每条规则独立调用 `Aggregate(...)`
6. 每条规则有结果时独立发布消息

返回值要求：

- 只要有任意一条规则被处理过，即使没有消息发布，也算 handled
- 如果所有规则都不满足聚合条件，则返回 `false, nil`

更具体地说：

- 至少一条规则启用聚合并成功执行到 `Aggregate(...)`，返回 `true`
- 没有任何聚合规则，返回 `false`

## 5. 幂等键规则

实时幂等键改为：

```text
realtime:{tenant_id}:{message_type}:{rule_id}:{biz_key}
```

聚合幂等键改为：

```text
aggregate:{tenant_id}:{message_type}:{rule_id}:{window_start}:{window_end}
```

## 6. 发布消息要求

所有新发布的 `DispatchMessage` 必须包含：

- `TenantID`
- `MessageType`
- `RuleID`
- `ConfigVersion`
- `Source`
- `BizVars`

## Processor 修改要求

修改文件：

- [notification/code/internal/delivery/processor.go](/C:/Users/Administrator/code/work/notification/code/internal/delivery/processor.go:1)

## 1. 字段调整

将：

```go
LoadConfig func(ctx context.Context, tenantID, messageType string) (*config.MessageConfig, error)
```

改为：

```go
LoadRuleSet func(ctx context.Context, tenantID, messageType string) (*config.MessageRuleSet, error)
```

允许保留旧字段做兼容，但新逻辑不得再依赖旧字段。

## 2. 新增规则查找逻辑

新增辅助函数，例如：

```go
func findRule(ruleSet *config.MessageRuleSet, ruleID string) (*config.ExecutableRule, bool)
```

行为要求：

- 按 `RuleID` 精确查找
- 找不到时按 `ErrUnsupportedConfig` 处理并记录失败

## 3. Process 行为要求

`Process(...)` 改成：

1. 校验 `DispatchMessage`
2. 校验 `RuleID` 非空
3. 加载 `MessageRuleSet`
4. 根据 `RuleID` 找到唯一规则
5. 用这条规则的 `Channel` 构造 `render.EffectivePolicy`
6. 渲染
7. 发送

注意：

- 不允许继续调用 `EffectiveChannel()` 这类“单配置唯一 channel”语义
- `RuleID` 找不到时，应保存失败记录而不是 panic

## 4. validate 行为要求

在 `validate(msg *DispatchMessage)` 中新增：

- `msg.RuleID == ""` 时返回 `ErrInvalidRequest`

## Scheduler 修改要求

修改文件：

- [notification/code/internal/scheduler/aggregate_scheduler.go](/C:/Users/Administrator/code/work/notification/code/internal/scheduler/aggregate_scheduler.go:1)

## 1. 字段调整

将：

```go
LoadAll func(ctx context.Context) (map[string]map[string]json.RawMessage, error)
```

改为：

```go
RuleSets config.RuleSetLoader
```

允许保留原字段，但新逻辑不得再依赖原字段。

## 2. Tick 行为要求

`Tick(...)` 改成：

1. `LoadAllRuleSets(...)`
2. 遍历所有 tenant
3. 遍历所有 message type
4. 遍历所有 rule
5. 对启用聚合且周期大于 0 的规则执行调度判断

## 3. tickOne 维度调整

原先 `tickOne(...)` 的配置输入是一个单配置 raw config。

改造后应以单条 `ExecutableRule` 为输入，例如：

```go
func (s *AggregateScheduler) tickOne(ctx context.Context, current time.Time, rule config.ExecutableRule) error
```

函数名可以不同，但语义必须是“按单条规则调度”。

## 4. watermark 使用要求

调用 watermark store 时必须传：

- `tenantID`
- `messageType`
- `ruleID`

## 5. 聚合 sender 接口

现有 `AggregateSender` 可以保留：

```go
SendAggregate(ctx, tenantID, messageType, windowStart, windowEnd)
```

但由于 scheduler 已经按 rule 调度，建议给 `Dispatcher` 增加内部的 rule 级执行函数。

如果你愿意直接改接口，也可以把 sender 改成：

```go
SendAggregateForRule(ctx context.Context, rule config.ExecutableRule, windowStart, windowEnd time.Time) (bool, error)
```

但这不是强制要求。

## 旧模型处理要求

以下旧模型允许暂时保留，但新路径不再依赖其“单配置”语义：

- `config.MessageConfig`
- `config.LoadMessageConfig`
- `config.MessageConfigLoader`
- `model.MessageConfig`

如果修改这些文件太大，第一版可以保留它们不动。

但：

- `Dispatcher`
- `Processor`
- `Scheduler`

不得继续围绕单配置工作。

## 测试修改要求

必须修改并补充以下测试。

## 1. Dispatcher 测试

修改文件：

- [notification/code/internal/dispatch/dispatcher_test.go](/C:/Users/Administrator/code/work/notification/code/internal/dispatch/dispatcher_test.go:1)

新增测试场景：

- 一个 `message_type` 下两条实时规则，都命中，发布两条消息
- 一个 `message_type` 下两条实时规则，一条命中一条不命中，只发布一条
- 实时发布消息时，`IdempotencyKey` 包含 `rule_id`
- 实时发布消息时，`DispatchMessage.RuleID` 已写入
- 一个 `message_type` 下两条聚合规则，都启用，分别执行
- 聚合发布消息时，`IdempotencyKey` 包含 `rule_id`
- 聚合无结果时，该规则算 handled 但不发消息

## 2. Processor 测试

修改文件：

- [notification/code/internal/delivery/processor_test.go](/C:/Users/Administrator/code/work/notification/code/internal/delivery/processor_test.go:1)

新增测试场景：

- 根据 `RuleID` 正确找到目标规则并发送
- 同一个 `message_type` 下两条规则，能命中正确接收人
- `RuleID` 缺失时返回 `ErrInvalidRequest`
- `RuleID` 找不到时记录失败

## 3. Scheduler 测试

修改文件：

- [notification/code/internal/scheduler/aggregate_scheduler_test.go](/C:/Users/Administrator/code/work/notification/code/internal/scheduler/aggregate_scheduler_test.go:1)

新增测试场景：

- 同一 `tenant + message_type` 下两条聚合规则，各自独立调度
- watermark 维度包含 `rule_id`
- 一个规则 5 分钟周期、一个规则 60 分钟周期，不互相干扰

## 4. 其他测试

如果 `DispatchMessage` 增加 `RuleID` 导致其他测试构造消息失败，需要一并修正所有构造函数。

## 代码风格要求

- 保持标准 Go 风格
- 不新增重型抽象
- 尽量新增小型辅助函数，而不是引入复杂接口层级
- 保持现有错误语义：
  - `ErrInvalidRequest`
  - `ErrUnsupportedConfig`
  - `ErrTemporaryFailure`

## 实现建议

推荐按以下顺序改：

1. 先加 `ExecutableRule`、`MessageRuleSet`
2. 再改 `DispatchMessage`、watermark
3. 再改 `Dispatcher`
4. 再改 `Processor`
5. 最后改 `Scheduler`
6. 最后统一补测试

## 必改文件清单

- [notification/code/config/rules.go](/C:/Users/Administrator/code/work/notification/code/config/rules.go)
- [notification/code/config/rule_loader.go](/C:/Users/Administrator/code/work/notification/code/config/rule_loader.go)
- [notification/code/pkg/notification/contract/contract.go](/C:/Users/Administrator/code/work/notification/code/pkg/notification/contract/contract.go:37)
- [notification/code/internal/dispatch/dispatcher.go](/C:/Users/Administrator/code/work/notification/code/internal/dispatch/dispatcher.go:1)
- [notification/code/internal/delivery/processor.go](/C:/Users/Administrator/code/work/notification/code/internal/delivery/processor.go:1)
- [notification/code/internal/scheduler/aggregate_scheduler.go](/C:/Users/Administrator/code/work/notification/code/internal/scheduler/aggregate_scheduler.go:1)
- [notification/code/internal/dao/aggregate_watermark.go](/C:/Users/Administrator/code/work/notification/code/internal/dao/aggregate_watermark.go:1)

## 必改测试清单

- [notification/code/internal/dispatch/dispatcher_test.go](/C:/Users/Administrator/code/work/notification/code/internal/dispatch/dispatcher_test.go:1)
- [notification/code/internal/delivery/processor_test.go](/C:/Users/Administrator/code/work/notification/code/internal/delivery/processor_test.go:1)
- [notification/code/internal/scheduler/aggregate_scheduler_test.go](/C:/Users/Administrator/code/work/notification/code/internal/scheduler/aggregate_scheduler_test.go:1)

## 验收标准

代码完成后，至少满足以下标准：

1. `Dispatcher` 可以在一个 `message_type` 下处理多条实时规则
2. `Dispatcher` 可以在一个 `message_type` 下处理多条聚合规则
3. `Processor` 按 `RuleID` 定位渠道和接收人
4. `AggregateScheduler` 按 `RuleID` 维护 watermark
5. 所有新增测试通过

## 验收命令

在 `notification` 目录执行：

```powershell
go test ./...
go build ./...
```

至少要保证这两个命令通过。

## 给 AI 的最后要求

实现时不要做这些自行发挥：

- 不要引入规则优先级系统
- 不要把 handler 契约改复杂
- 不要试图顺手把 capability/schema/默认配置体系全部重构
- 不要再回到“单配置 + wrapper 兜底”的思路

正确方向只有一个：

- 运行时引入 `rule set`
- 分发、调度、投递全部转到 `rule_id` 维度
