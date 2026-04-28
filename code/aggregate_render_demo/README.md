# Aggregate Render Demo

这个 demo 只演示一件事：

- 输入是请求 JSON
- 顶层统一返回结构里带 `message_type`
- 每种消息占一个独立属性
- 平台根据 `message_type` 选择模板目录
- `window_label` 由渲染层根据窗口自己生成
- 渲染层用 Go `text/template` 按渠道生成最终 `message`

这里刻意把输入和输出分开：

- 输入：`sample_request.json`
- 输出：`sample_result.json`

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

当前示例结构是：

```json
{
  "message_type": "xdr_risk_digest",
  "xdr_risk_digest": {
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

模板直接吃强类型字段，例如：

```gotemplate
{{.WindowLabel}}内发现{{.Payload.TotalCount}}条高危风险。
```

如果结果里带案例列表，模板直接 `range .Payload.Examples`：

```gotemplate
{{range .Payload.Examples}}
- {{.ObjectName}}：{{.RiskType}} {{.EventCount}}条
{{end}}
```

这样平台只统一两件事：

- 模板定位规则：`message_type + channel`
- 结果协议：每种消息一个独立子结构

当前 demo 还额外做了一条运行时约束：

- 顶层消息体里只能有一个非空
- 且它必须和 `message_type` 一致

当前 `buildRenderView` 采用通用反射逻辑：

- 根据顶层哪个消息字段非空，自动识别当前 payload
- 校验它和 `message_type` 是否一致
- 自动把该 payload 作为模板的 `.Payload`

因此，新增一种消息时，demo 里只需要：

- 在顶层结果结构里增加一个新字段
- 增加对应模板目录

不需要再为 `buildRenderView` 新增一个 `switch case`。

运行方式：

```powershell
go run .\code\aggregate_render_demo\main.go
```

当前示例使用：

- [sample_config.json](/C:/Users/Administrator/code/notes/code/aggregate_render_demo/sample_config.json)
- [sample_result.json](/C:/Users/Administrator/code/notes/code/aggregate_render_demo/sample_result.json)
- [email.tmpl](/C:/Users/Administrator/code/notes/code/aggregate_render_demo/templates/xdr_risk_digest/email.tmpl)
- [wecom.tmpl](/C:/Users/Administrator/code/notes/code/aggregate_render_demo/templates/xdr_risk_digest/wecom.tmpl)
- [sms.tmpl](/C:/Users/Administrator/code/notes/code/aggregate_render_demo/templates/xdr_risk_digest/sms.tmpl)

输出会分别展示 `email`、`wecom`、`sms` 三个渠道的最终消息。

当前目录结构：

```text
code/aggregate_render_demo/
  main.go
  render.go
  messages/
    result.go
    xdr_risk_digest.go
  sample_request.json
  sample_result.json
  templates/
    xdr_risk_digest/
      email.tmpl
      wecom.tmpl
      sms.tmpl
```
