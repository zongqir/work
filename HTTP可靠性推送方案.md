# 上游通过 HTTP 调用 notification 的可靠性推送方案



- 上游业务系统
- 通过 `HTTP` 调用 `notification` 服务
- `notification` 再负责下游微信、邮件等渠道发送、失败重试和渠道适配


## 1. 先统一几个边界

这个方案里，上游真正要解决的是：

- 业务成功后，通知请求不能因为进程异常、网络抖动、超时等原因被悄悄丢掉
- 同一条通知允许重复提交，但不能因为重复提交导致下游重复执行多次
- 出问题时，能知道卡在哪一段，并且可以补偿

这个方案里，上游不负责：

- 微信、邮件发送细节
- 多渠道路由
- notification 内部消费重试
- 渠道送达 ACK

所以上游的可靠性目标应该收敛为：

- 保证“应该发给 notification 的请求”被可靠记录
- 保证“最终至少一次成功提交给 notification”
- 保证重复提交可幂等

## 2. 可靠性的正确落点

如果上游只是这样写：

1. 业务事务提交成功
2. 直接调用 `notification HTTP`

那么仍然存在典型风险：

- 业务已成功，但 HTTP 请求根本没发出去
- HTTP 请求发出后超时，上游不知道对方有没有收到
- 进程在业务提交后、HTTP 调用前崩溃

所以只靠同步 HTTP 直调，不足以支撑高价值通知。

如果通知的重要性较高，推荐方案是：

1. 业务事务内写业务数据
2. 同事务写一条 `notification_outbox`
3. 事务提交成功
4. 后台 dispatcher 扫描 outbox
5. dispatcher 调用 `notification HTTP`
6. notification 返回成功受理
7. outbox 记录更新为 `sent`
8. 调用失败则重试，超过阈值进入 `dead`

这就是 `Outbox + HTTP relay`。


## 4. 推荐总体方案

推荐采用两层结构：

### 4.1 业务侧调用层

对业务只暴露两个动作：

- `Send(ctx, cmd)`
- `EnqueueInTx(ctx, tx, cmd)`

其中：

- 普通通知可以直接 `Send`
- 高价值通知必须 `EnqueueInTx`

业务不要直接操作 outbox 表，不要自己处理重试，不要自己拼 HTTP 请求。

### 4.2 后台投递层

由独立 dispatcher 做以下事情：

1. 扫描 `pending` 且到期的 outbox 记录
2. 抢占成 `sending`
3. 调用 `notification HTTP`
4. 成功则标记 `sent`
5. 失败则更新 `retry_count` 和 `next_retry_at`
6. 超过阈值则标记 `dead`

## 5. HTTP 模式下的关键约束

### 5.1 notification HTTP 必须支持幂等

上游必须给每条通知带一个稳定的 `idempotentKey`，例如：

```text
trade:order_paid:202604080001
```

notification 侧必须基于这个键做幂等。

因为 HTTP 场景最常见的问题不是“没发”，而是“超时后不确定对方是否已处理”。
如果没有幂等，上游重试就可能造成重复通知。

### 5.2 HTTP 成功的语义必须明确

推荐约定：

- `2xx` 表示 notification 已成功受理
- 返回体里最好带 `requestId`
- 上游只以“已受理”作为成功，不以“已发送到渠道”作为成功

也就是说：

- 上游成功 = notification 接单成功
- 渠道成功 = notification 内部职责

### 5.3 HTTP 超时不能直接当失败丢掉

HTTP 超时通常有两种情况：

1. notification 根本没收到
2. notification 已收到并处理，但响应没回来

所以 HTTP 超时必须走幂等重试，而不是简单记日志后结束。

## 6. 推荐时序

### 6.1 普通通知

适合低价值、允许偶发丢失或人工补发的通知。

```text
业务代码 -> SDK Send -> notification HTTP -> 返回 2xx/失败
```

特点：

- 简单
- 实时
- 但不解决“业务提交成功后进程崩溃导致未发”问题

### 6.2 高价值通知

适合支付成功、退款成功、关键告警、重要状态变更等。

```text
业务事务:
  更新业务表
  插入 notification_outbox
  commit

后台 dispatcher:
  claim pending
  调 notification HTTP
  success -> sent
  fail -> retry/dead
```

这是推荐主方案。

## 7. outbox 表设计建议

建议字段如下：

- `message_id`
- `biz_type`
- `event_code`
- `template_code`
- `receiver_json`
- `payload_json`
- `idempotent_key`
- `trace_id`
- `priority`
- `headers_json`
- `meta_json`
- `status`
- `retry_count`
- `next_retry_at`
- `last_error`
- `created_at`
- `updated_at`

推荐状态：

- `pending`
- `sending`
- `sent`
- `dead`

说明：

- `message_id` 用于本地记录主键
- `idempotent_key` 用于对 notification 幂等
- `status` 描述当前投递进度
- `retry_count` 和 `next_retry_at` 用于补偿调度

## 8. dispatcher 的正确实现方式

### 8.1 不要扫出来就直接发

如果多个 worker 并发跑，直接查询 `pending` 后发送，会出现重复处理。

推荐做法：

1. 先查出一批候选记录
2. 对每条记录执行抢占更新
3. 只有抢占成功的 worker 才能发送

示意 SQL：

```sql
SELECT *
FROM notification_outbox
WHERE status = 'pending'
  AND next_retry_at <= NOW(6)
ORDER BY next_retry_at, created_at, message_id
LIMIT 100;
```

逐条抢占：

```sql
UPDATE notification_outbox
SET status = 'sending', updated_at = NOW(6)
WHERE message_id = ?
  AND status = 'pending';
```

如果影响行数为 `1`，说明抢占成功。

### 8.2 成功和失败怎么落状态

调用 notification 成功：

```sql
UPDATE notification_outbox
SET status = 'sent', last_error = '', updated_at = NOW(6)
WHERE message_id = ?;
```

调用失败但还可重试：

```sql
UPDATE notification_outbox
SET status = 'pending',
    retry_count = ?,
    next_retry_at = ?,
    last_error = ?,
    updated_at = ?
WHERE message_id = ?;
```

超过阈值：

```sql
UPDATE notification_outbox
SET status = 'dead',
    retry_count = ?,
    last_error = ?,
    updated_at = ?
WHERE message_id = ?;
```

## 9. 重试策略建议

推荐固定阶梯退避：

1. 第 1 次失败：1 分钟后
2. 第 2 次失败：5 分钟后
3. 第 3 次失败：15 分钟后
4. 第 4 次失败：1 小时后
5. 第 5 次失败：6 小时后
6. 再失败：进入 `dead`

这样比“无限频繁重试”更稳，也更利于告警和人工处理。

## 10. HTTP 请求模型建议

上游给 notification 的请求建议分成两层：

### 10.1 固定信封

- `bizType`
- `eventCode`
- `templateCode`
- `receivers`
- `idempotentKey`
- `traceId`
- `priority`

### 10.2 业务 payload

复杂参数统一放进 `payload`，由具体业务结构体承载。

不要设计成：

- 一个包含上百个平铺字段的大 DTO
- 或一堆位置参数方法

应该设计成：

```json
{
  "bizType": "trade",
  "eventCode": "order_paid",
  "templateCode": "tpl_order_paid",
  "receivers": [
    { "type": "wechat", "value": "openid-123" }
  ],
  "idempotentKey": "trade:order_paid:202604080001",
  "traceId": "trace-001",
  "priority": 10,
  "payload": {
    "orderId": "202604080001",
    "userId": "u-1",
    "amountFen": 19900
  }
}
```

## 11. 上游 SDK 应该做什么

推荐上游二方库只做这些事：

- 统一 `Command` 模型
- 统一参数校验
- 统一 `idempotentKey`
- 统一 HTTP 调用封装
- 统一 `EnqueueInTx`
- 统一日志、trace、metrics
- 统一错误分类

不应该做这些事：

- 模板管理
- 渠道路由
- 微信、邮件发送实现
- notification 内部重试编排

## 12. 监控和告警建议

至少要有以下指标：

- HTTP 调用总量
- HTTP 调用失败量
- HTTP 调用耗时
- outbox `pending` 数量
- outbox `dead` 数量
- dispatcher `sent/retry/dead` 数量

至少要有以下告警：

- outbox `dead` 大于 0
- outbox `pending` 持续堆积
- HTTP 调用失败率异常升高
- HTTP 调用 p95/p99 明显升高

## 13. 推荐结论

如果只考虑上游通过 HTTP 调 notification，那么推荐方案不是“直接同步 HTTP 重试几次”。

推荐分层是：

1. 普通通知：`Send(ctx, cmd)`
2. 高价值通知：`EnqueueInTx(ctx, tx, cmd)` + `outbox` + dispatcher + HTTP 幂等

一句话总结：

对上游来说，真正可靠的不是“HTTP 本身”，而是：

- 本地事务里先把待发送通知可靠记下来
- 然后通过可重试、可幂等的 HTTP 投递给 notification

这才是适合当前架构的 HTTP 可靠性推送方案。
