# Aggregate Registry Demo

这个版本演示的是 `message_type + template_vars + effective policy`：

- 业务聚合结果直接返回 `message_type + template_vars`
- 平台不再解码业务 payload，也不再注册消息结构体
- 渠道不由业务结果定义，而是由生效策略决定
- 邮件、webhook、短信围绕同一份 `template_vars` 工作

这里刻意把输入和输出分开：

- 输入：`sample_request.json`
- 聚合结果：`sample_result.json`
- 生效策略：`sample_policy.json`

输入侧只保留平台自己需要的上下文，例如租户标识和查询参数：

```json
{
  "tenant_id": "t_1001",
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
  "template_vars": {
    "window_label": "过去1小时",
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

模板只引用 `template_vars`，例如：

```tmpl
{{.window_label}}风险摘要：{{.total_count}}条高危事件
```

当前职责拆分：

- `message_type + template_vars` 由业务聚合侧负责
- `tenant_id + message_type -> channels` 由通知配置侧负责
- `email` 和 `webhook` 走本地模板资产
- `sms` 直接使用 `templateCode + kv`

新增一种消息时，通常只需要：

1. 约定新的 `message_type`
2. 定义该 `message_type` 允许出现的 `template_vars`
3. 按需新增邮件和 webhook 模板资产
4. 在配置侧新增渠道策略

不需要新增 payload 结构体，也不需要注册解码逻辑。

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
