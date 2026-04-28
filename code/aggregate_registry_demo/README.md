# Aggregate Registry Demo

这个版本演示的是 `message_type + payload + registry + effective policy`：

- 业务聚合结果只返回 `message_type + payload`
- 平台通过 `registry` 找到该消息对应的强类型结构体
- 渠道不由业务结果定义，而是由生效策略决定
- 邮件、webhook、短信分别产出各自需要的结果格式

这里刻意把输入和输出分开：

- 输入：`sample_request.json`
- 聚合结果：`sample_result.json`
- 生效策略：`sample_policy.json`

输入侧允许更灵活：

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

输出侧要求强约束，必须符合平台定义的返回结构：

```json
{
  "message_type": "xdr_risk_digest",
  "payload": {
    "total_count": 23,
    "category_count": 3,
    "examples": [
      {
        "object_name": "host-a",
        "risk_type": "暴力破解",
        "event_count": 6
      }
    ]
  }
}
```

生效策略示例：

```json
{
  "tenant_id": "t_1001",
  "message_type": "xdr_risk_digest",
  "channels": [
    {
      "channel": "email",
      "template_code": "xdr_risk_digest_default"
    },
    {
      "channel": "webhook",
      "template_code": "xdr_risk_digest_default"
    },
    {
      "channel": "sms",
      "template_code": "SMS_001"
    }
  ]
}
```

注册表定义在 [registry.go](/C:/Users/Administrator/code/notes/code/aggregate_registry_demo/messages/registry.go)：

```go
var bizAggregateResultRegistry = map[string]BizAggregateResultMeta{
    "xdr_risk_digest": {
        NewPayload: func() any {
            return &XdrRiskDigest{}
        },
        BuildSMSParams: func(windowLabel string, payload any) (map[string]string, error) {
            ...
        },
    },
}
```

当前职责拆分：

- `message_type + payload` 由业务聚合侧负责
- `tenant_id + message_type -> channels` 由通知配置侧负责
- `email` 和 `webhook` 走本地模板资产
- `sms` 走供应商 `templateCode + kv`

新增一种消息时，需要：

1. 新增 payload 结构体
2. 在 `registry` 注册解码逻辑
3. 如果支持短信，补充短信参数映射
4. 按需新增邮件和 webhook 模板资产
5. 在配置侧新增渠道策略

不需要改统一顶层结构。

当前目录结构：

```text
code/aggregate_registry_demo/
  main.go
  render.go
  sample_request.json
  sample_result.json
  sample_policy.json
  messages/
    envelope.go
    registry.go
    xdr_risk_digest.go
  templates/
    email/
      xdr_risk_digest_default.subject.tmpl
      xdr_risk_digest_default.body.tmpl
    webhook/
      xdr_risk_digest_default.tmpl
```

运行方式：

```powershell
cd .\code\aggregate_registry_demo
go run .
```
