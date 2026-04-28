# Aggregate Registry Demo

这个版本演示的是 `message_type + payload + registry`：

- 顶层结果固定成 `message_type` + `payload`
- 平台通过 `registry` 找到该消息对应的结构体和模板目录
- 模板仍然按强类型字段渲染

当前示例结构：

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

注册表定义在 [main.go](/C:/Users/Administrator/code/notes/code/aggregate_registry_demo/main.go)：

```go
var registry = map[string]MessageSpec{
    "xdr_risk_digest": {
        TemplateCode: "xdr_risk_digest",
        NewPayload: func() any {
            return &XdrRiskDigest{}
        },
    },
}
```

新增一种消息时，需要：

1. 新增 payload 结构体
2. 在 `registry` 注册一次
3. 新增模板目录

不需要改统一顶层结构。

运行方式：

```powershell
go run .\code\aggregate_registry_demo\main.go
```
