# Aggregate Registry Demo

这个版本演示的是 `message_type + biz_vars + system_vars + effective policy`：

- 业务聚合结果返回 `message_type + biz_vars`
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

这种做法下：

- 上游协议保持简单，不需要传 `biz.xxx`
- 模板边界清晰，不会和系统变量混在一起
- 后续系统变量增加时，只需要往 `.sys` 里补

当前目录结构：

```text
code/aggregate_registry_demo/
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

模板预览工具：

```powershell
cd .\code\aggregate_registry_demo
go run . -request sample_request.json -result sample_result.json -policy sample_policy.json -templates templates
```

如果要先看最终注入模板的 `.biz / .sys` 上下文，可以加：

```powershell
go run . -show-context
```
