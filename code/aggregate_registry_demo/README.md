# Aggregate Contract Demo

这个版本演示的是生产级聚合契约：

- AES 定义聚合请求和返回协议
- 业务方实现 `Handler` 接口
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

- `Handler`：业务方必须实现的最小接口
- `MessageType()`：业务方自己定义消息标识
- `MustRegister()`：业务方自己显式注册
- `Aggregate(...)`：业务方自己完成聚合并返回结果
- `Evaluate(...)`：业务方自己完成实时筛选并返回结果
- `BizAggregateRequest`：聚合请求
- `RealtimeRequest`：实时请求
- `RealtimeDecision`：实时判断结果
- `DispatchMessage`：实时命中后要分发的消息载体
- `ErrInvalidRequest`：请求非法
- `ErrUnsupportedConfig`：配置不支持
- `ErrTemporaryFailure`：临时失败，可由调用方决定是否重试
- `ErrAggregatorNotFound`：运行时没有找到对应 `message_type` 的实现

统一分发入口见 [dispatcher.go](/C:/Users/Administrator/code/notes/code/aggregate_registry_demo/dispatcher.go:1)。

分发口径：

- AES 提供 `Dispatcher`
- `Dispatcher.Dispatch(...)` 是统一发布能力
- 默认发布口径可以是 MQ，不绑定数据库
- 聚合和实时最后都归一到 `DispatchMessage`

实时场景口径：

- AES 调用 `Dispatcher.HandleRealtime(...)`
- 业务方实现 `Handler` 的 `Evaluate(...)`
- AES 把 `filter_query json + event_body json` 交给业务方
- 业务方自己筛选
- 命中时返回 `Matched=true + biz_vars`
- `Dispatcher` 统一负责把结果发布出去

正式业务实现统一放在 `handlers/`：

- 每个 `message_type` 一个子目录
- 每个子目录只有一个 `handler.go`
- 一个 `Handler` 同时实现：
  `MessageType()`、`MustRegister()`、`Aggregate(...)`、`Evaluate(...)`
- `handlers/registry.go` 统一按 `message_type` 注册和查找

共享契约统一放在 `contract.go` 根包里，不放进 `handlers/`。原因很简单：

- 它描述的是 AES 和业务方共享的契约
- 不是某个具体实现目录的私有代码
- `handlers/` 只放实现和注册，职责更清楚

这种方式的作用是：

- 避免新增实现时忘记手动改注册表
- `message_type` 由实现自己声明，不需要在外部重复写一遍
- 聚合和实时统一收口到一个 handler 文件里

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
  handlers/
    registry.go
    registry_test.go
    xdr_risk_digest/
      handler.go
      handler_test.go
  contract.go
  dispatcher.go
  dispatcher_test.go
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
