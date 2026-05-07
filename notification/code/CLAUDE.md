# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

```bash
go build ./...
go test ./...
go test ./handlers/sample_both/   # single package
```

Module: `notes`, Go 1.24.

## Architecture

这是一个通知推送平台的聚合契约实现。核心流程：配置驱动的定时调度 → 业务 handler 聚合/评估 → 投递消息到 MQ → 消费端发送 + 重试。

### 包职责

```
contract/    类型、Handler 接口（4方法）、注册表（MustRegister/Resolve）、错误变量
config/      MessageConfig（租户配置结构）、Cache（带异步刷新的配置缓存）
dispatch/    Dispatcher.SendAggregate / SendRealtime —— 编排层：查配置、解析 filter、调 handler、构建 DispatchMessage 并发到 Publisher
scheduler/   AggregateScheduler —— 定时轮询配置，按 watermark 推进聚合窗口，调用 Dispatcher
delivery/    Processor —— 消费 DispatchMessage：加载策略 → 渲染渠道消息 → 发送 → 失败重试 → 记录结果
consumer/    PulsarConsumer —— 订阅 Pulsar，反序列化 DispatchMessage，调用 Processor.Process(...)
handlers/    业务 handler 实现。每个 handler 通过 init() 自注册到 contract.MustRegister()
render/      模板渲染（email/webhook/SMS），根据 EffectivePolicy 产出最终通知内容
preview/     离线预览：从 JSON 文件加载请求/结果/策略，渲染输出
publisher/   PulsarPublisher —— 将 DispatchMessage 序列化后发到 Pulsar
bootstrap/   启动装配：创建 Pulsar client、publisher、dispatcher
```

### Handler 接口

```go
type Handler interface {
    MessageType() string
    NewFilter() any
    Aggregate(ctx, *BizAggregateRequest) (*BizAggregateResult, error)
    Evaluate(ctx, *RealtimeRequest) (*RealtimeResult, error)
}
```

- `NewFilter()` 返回空壳供平台反序列化配置中的 filter JSON
- `Aggregate` 和 `Evaluate` 的 `Filter` 字段由平台解析好后传入（类型已是 handler 自己的 Filter 类型）
- `RealtimeRequest.Event` 是 `any`，调用方先解析成自己的事件结构，再传给 handler

### 幂等 Key 设计

- 聚合：`aggregate:{tenant}:{messageType}:{windowStart}:{windowEnd}`
- 实时：`realtime:{tenant}:{messageType}:{bizKey}`（bizKey 由 handler 在 RealtimeResult.IdempotencyKey 中返回）
