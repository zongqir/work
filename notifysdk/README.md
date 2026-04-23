# notifysdk

这是一个按你当前场景收敛过的 Go 二方库骨架：

- 上游业务只表达“我要发什么通知”
- SDK 负责统一模型、校验和投递适配
- MQ 发送能力通过 `MQPublisher` 复用你们现有发送库
- 高价值通知可以切到 `OutboxTransport`
- 每个业务 `payload` 必须实现 `Validate() error`
- 监控通过 `Metrics` 抽象接入，默认 `noop`，可选 Prometheus

## 包内结构

- `Command` / `Envelope`
  - `Command` 面向业务调用方
  - `Envelope` 面向传输层
- `Client`
  - 统一入口，固定为 `Send(ctx, cmd)` 和 `EnqueueInTx(ctx, tx, cmd)`
- `HTTPTransport`
  - 直连下游 `notification` HTTP
- `MQTransport`
  - 复用你们自有 MQ producer
- `OutboxTxStore`
  - 事务内写本地消息表
- `Dispatcher`
  - 后台 relay/poller，把 outbox 投递出去
- `OceanBaseOutboxStore`
  - 面向 OceanBase 的本地消息表实现
- `Metrics`
  - 默认空实现
  - `PrometheusMetrics` 提供现成指标

## 推荐用法

普通通知：

```go
type OrderPaidPayload struct {
    OrderID string `json:"orderId"`
    UserID  string `json:"userId"`
}

func (p OrderPaidPayload) Validate() error {
    if p.OrderID == "" {
        return errors.New("orderId is required")
    }
    if p.UserID == "" {
        return errors.New("userId is required")
    }
    return nil
}

publisher := YourMQPublisher{}

client := notifysdk.New(&notifysdk.MQTransport{
    Topic:     "notification.events",
    Publisher: publisher,
}, notifysdk.WithMetrics(metrics))
```

高价值通知：

```go
store := notifysdk.NewOceanBaseOutboxStore(db)

client := notifysdk.New(
    &notifysdk.MQTransport{Topic: "notification.events", Publisher: publisher},
    notifysdk.WithOutboxTxStore(store),
)

err := txManager.WithTx(ctx, func(tx *sql.Tx) error {
    if err := orderRepo.Pay(ctx, tx, orderID); err != nil {
        return err
    }

    _, err := client.EnqueueInTx(ctx, tx, cmd)
    return err
})
```

表结构见 [oceanbase_notification_outbox.sql](/C:/Users/Administrator/code/notes/notifysdk/oceanbase_notification_outbox.sql)。

## 设计边界

这个库应该做：

- 统一通知请求模型
- 统一校验
- 通过 `Payload.Validate()` 让业务参数校验前置
- 统一幂等键
- 统一 HTTP/MQ/Outbox 入口
- 暴露稳定的监控埋点
- 对业务暴露 `EnqueueInTx`，不暴露 outbox 细节
- 给 OceanBase 提供现成的 outbox store 落地

这个库不做：

- 模板渲染
- 渠道路由
- 微信/邮件发送细节
- 下游 notification 的消费重试体系

## Prometheus 指标

- `notify_sdk_requests_total`
  - 标签：`mode`, `biz_type`, `event_code`, `result`
- `notify_sdk_request_duration_seconds`
  - 标签：`mode`, `biz_type`, `event_code`
- `notify_sdk_errors_total`
  - 标签：`mode`, `error_type`
- `notify_sdk_dispatch_total`
  - 标签：`result`
- `notify_sdk_outbox_size`
  - 标签：`status`
