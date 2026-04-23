# AES 双通道批次表字段设计

## 1. 设计结论

对于双通道执行器，我不建议：

- 两张表完全共用一套字段且没有语义差异
- 两张表完全设计成两套不同模型

更合理的方式是：

- 两张表分开
- 核心字段统一
- 按通道补少量差异字段

一句话总结：

- 80% 同构
- 20% 差异化

## 2. 为什么这样设计

这样设计的好处有三个：

### 2.1 执行器逻辑更容易复用

如果两张表核心字段一致，执行器可以复用：

- 扫描逻辑
- 抢占逻辑
- 状态推进逻辑
- 发送逻辑
- 回写逻辑

### 2.2 响应式与后台的语义仍能区分

虽然核心字段一致，但两类任务的触发方式不同：

- 响应式偏事件驱动
- 后台偏窗口驱动

因此保留少量差异字段是必要的。

### 2.3 监控和排障更清晰

字段风格一致后：

- 监控指标更容易统一
- SQL 查询方式更容易统一
- 观察任务状态更容易统一

## 3. 两张表建议

建议保留两张表：

- `aes_notify_realtime_batch`
- `aes_notify_schedule_batch`

## 4. 核心统一字段

这部分字段建议两张表都保留。

### 主键与路由字段

- `id`
- `tenant_id`
- `biz_type`
- `message_type`

说明：

- `tenant_id` 用于标识租户
- `biz_type` 用于标识来源服务或业务域
- `message_type` 用于标识通知类型

### 发送相关字段

- `channel_set`
- `template_code`
- `payload_snapshot`
- `idempotent_key`

说明：

- `channel_set` 表示本批次要走哪些渠道
- `template_code` 供执行器选择模板
- `payload_snapshot` 是业务层产出的结构化数据
- `idempotent_key` 用于发送幂等控制

### 状态相关字段

- `status`
- `retry_count`
- `last_error`
- `next_retry_at`

说明：

- `status` 建议统一使用 `pending/sending/success/failed`
- `retry_count` 用于有限次补发
- `last_error` 记录最后一次失败原因
- `next_retry_at` 控制重试时间

### 时间字段

- `created_at`
- `updated_at`

## 5. 响应式表特有字段

响应式表建议增加事件维度字段。

表：

- `aes_notify_realtime_batch`

建议特有字段：

- `event_time`
- `priority`
- `source_id`

字段说明：

- `event_time`：业务事件实际发生时间
- `priority`：响应式场景下可用于标识优先级
- `source_id`：原始业务对象 ID，例如事件 ID、告警 ID、任务 ID

这类字段更适合事件驱动型通知。

## 6. 后台表特有字段

后台表建议增加时间窗口维度字段。

表：

- `aes_notify_schedule_batch`

建议特有字段：

- `window_start`
- `window_end`
- `schedule_time`

字段说明：

- `window_start`：本次汇总窗口开始时间
- `window_end`：本次汇总窗口结束时间
- `schedule_time`：本次计划调度时间

这类字段更适合周期汇总型通知。

## 7. 表结构差异总结

可以简单理解成：

### 响应式表

- 更强调“事件”
- 更强调“低延迟”
- 更适合保留事件发生时间和来源对象 ID

### 后台表

- 更强调“窗口”
- 更强调“汇总”
- 更适合保留时间窗口和计划调度时间

## 8. 推荐字段对照

### 8.1 `aes_notify_realtime_batch`

建议字段：

- `id`
- `tenant_id`
- `biz_type`
- `message_type`
- `source_id`
- `event_time`
- `priority`
- `channel_set`
- `template_code`
- `payload_snapshot`
- `idempotent_key`
- `status`
- `retry_count`
- `last_error`
- `next_retry_at`
- `created_at`
- `updated_at`

### 8.2 `aes_notify_schedule_batch`

建议字段：

- `id`
- `tenant_id`
- `biz_type`
- `message_type`
- `window_start`
- `window_end`
- `schedule_time`
- `channel_set`
- `template_code`
- `payload_snapshot`
- `idempotent_key`
- `status`
- `retry_count`
- `last_error`
- `next_retry_at`
- `created_at`
- `updated_at`

## 9. 状态模型建议统一

两张表虽然分开，但状态模型建议统一：

- `pending`
- `sending`
- `success`
- `failed`

这样执行器可以最大化复用处理代码。

## 10. 第一版不建议加太多字段

虽然可以继续扩字段，但第一版不建议加太多，例如：

- 渠道详细回执字段
- 多阶段审核字段
- 复杂工作流字段
- 多级死信字段

原因是：

- 现在重点是把链路跑通
- 太多字段会让执行器复杂度迅速膨胀

## 11. 当前推荐结论

双通道批次表更适合这样设计：

- 两张表
- 核心字段统一
- 响应式表保留事件字段
- 后台表保留窗口字段

一句话总结：

响应式和后台任务真正需要隔离的是执行通道，但在表结构上没有必要彻底分家；保持核心字段统一、按场景补少量差异字段，是当前最平衡的设计。 
