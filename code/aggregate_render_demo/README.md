# Aggregate Render Demo

这个 demo 只演示一件事：

- 配置里给出 `template_code`
- 业务返回聚合结果数据
- 平台按 `template_code/channel` 维护模板文件
- `window_label` 由渲染层根据窗口自己生成
- 渲染层用 Go `text/template` 按渠道生成最终 `message`

模板直接使用原始字段名，不做额外驼峰转换，例如：

```gotemplate
{{index . "window_label"}}内发现{{index . "total"}}条高危风险
```

如果结果里带列表，例如 `examples`，模板可以直接用 `range`：

```gotemplate
典型案例：
{{range $e := index . "examples"}}
- {{index $e "name"}}：{{index $e "risk"}} {{index $e "count"}}条
{{end}}
```

这样平台只统一两件事：

- 模板定位规则：`template_code + channel`
- 渲染协议：普通字段用 `index`，列表字段用 `range`

至于案例要不要返回、返回多少条、每条案例有哪些字段，交给聚合结果自己决定。

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
  sample_config.json
  sample_result.json
  templates/
    xdr_risk_digest/
      email.tmpl
      wecom.tmpl
      sms.tmpl
```
