# Aggregate Contract Demo

这个版本演示的是生产级聚合契约：

- AES 定义聚合请求和返回协议
- 业务方实现 `Aggregator` 接口
- 业务结果返回 `message_type + biz_vars`
- 平台根据请求上下文补 `system_vars`，例如 `window_label`
- 渲染模板时统一包装成 `.biz` 和 `.sys`
- 渠道不由业务结果定义，而是由生效策略决定

这里刻意把输入和输出分开：

- 输入：`sample_request.json`
- 聚合结果：`sample_result.json`
- 生效策略：`sample_policy.json`

输入侧保留平台自己需要的上下文，例如租户和时间窗口：

```json
{
  "tenant_id": "t_1001",
  "window_start": "2026-04-28T10:00:00Z",
  "window_end": "2026-04-28T11:00:00Z",
  "config_body": {
    "severity": ["high", "critical"],
    "sample_limit": 3
  }
}
```

业务聚合结果示例：

```json
{
  "message_type": "xdr_risk_digest",
  "biz_vars": {
    "total_count": "23",
    "category_count": "3",
    "examples": [
      {
        "object_name": "host-a",
        "risk_type": "暴力破解",
        "event_count": "6"
      }
    ]
  }
}
```

平台在渲染前会构造模板上下文：

```json
{
  "biz": {
    "total_count": "23"
  },
  "sys": {
    "window_label": "过去1小时"
  }
}
```

模板引用方式例如：

```tmpl
{{.sys.window_label}}风险摘要：{{.biz.total_count}}条高危事件
```

当前职责拆分：

- `message_type + biz_vars` 由业务聚合侧负责
- `system_vars` 由通知平台负责
- `tenant_id + message_type -> channels` 由通知配置侧负责
- `email` 和 `webhook` 走本地模板资产
- `sms` 直接使用 `templateCode + kv`

生产级契约入口见 [contract.go](/C:/Users/Administrator/code/notes/code/aggregate_registry_demo/contract.go:1)：

- `Aggregator`：业务方实现的聚合接口
- `ErrInvalidRequest`：请求非法
- `ErrUnsupportedConfig`：配置不支持
- `ErrTemporaryFailure`：临时失败，可由调用方决定是否重试
- `ErrAggregatorNotFound`：运行时没有找到对应 `message_type` 的实现

正式业务实现目录放在 `aggregators/`：

- 每个 `message_type` 一个子目录
- 子目录里放正式实现代码
- `registry.go` 只做两件事：`MustRegister` 和 `Resolve`
- 现在先给了一个样例：[aggregators/xdr_risk_digest](/C:/Users/Administrator/code/notes/code/aggregate_registry_demo/aggregators/xdr_risk_digest)
- 这个样例故意压到最小，只保留接口骨架
- 业务侧真正需要做的事，就是在 `Aggregate(...)` 里填自己的 `biz_vars`
- 每个实现同时满足 `Aggregator + MessageType()`，并在 `init()` 里自注册

`contract.go` 继续留在根包，不放进 `aggregators/`。原因很简单：

- 它描述的是 AES 和业务方共享的契约
- 不是某个具体实现目录的私有代码
- `aggregators/` 只放实现和注册，职责更清楚

这种方式的作用是：

- 避免新增实现时忘记手动改注册表
- `message_type` 由实现自己声明，不需要在外部重复写一遍
- 运行时只保留最小分发能力，不引入一套复杂 registry API

需要注意：

- `init()` 只能解决“忘记注册”
- 如果某个实现包根本没有被引入，它的 `init()` 也不会执行

这种做法下：

- 上游协议保持简单，不需要传 `biz.xxx`
- 模板边界清晰，不会和系统变量混在一起
- 后续系统变量增加时，只需要往 `.sys` 里补

当前目录结构：

```text
code/aggregate_registry_demo/
  aggregators/
    registry.go
    registry_test.go
    xdr_risk_digest/
      aggregator.go
      aggregator_test.go
  contract.go
  render.go
  sample_request.json
  sample_result.json
  sample_policy.json
  messages/
    envelope.go
  preview/
    preview.go
    preview_test.go
  templates/
    email/
      xdr_risk_digest_default.subject.tmpl
      xdr_risk_digest_default.body.tmpl
    webhook/
      xdr_risk_digest_default.tmpl
```

最轻的预览方式是复用 `preview/` 子目录里的辅助函数，或者直接跑那里的测试：

```powershell
cd .\code\aggregate_registry_demo
go test .\preview
```

如果业务方自己要写一个很薄的调用，只需要调用：

```go
preview, err := preview.FromFiles(
    "sample_request.json",
    "sample_result.json",
    "sample_policy.json",
    "templates",
    true,
)
```

如果想把结果打印成 JSON：

```go
data, err := preview.Marshal(preview)
```
