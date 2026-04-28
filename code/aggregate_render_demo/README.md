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
