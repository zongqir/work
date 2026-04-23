# AES 统一策略表设计

## 1. 设计结论

在当前方案下，我建议策略侧统一成一张表。

但这里要特别强调：

- 统一的是通知编排策略
- 不是把所有业务规则都塞进一个表

一句话总结：

- 策略统一一张表
- 业务规则继续留在各业务服务里

## 2. 为什么建议统一成一张表

当前已经明确的配置模型是：

- 默认配置在 `configmap`
- 单租户特殊配置在数据库
- 读取时先查租户覆盖，没覆盖再降级到默认值

在这个模型下，如果数据库侧还拆成很多策略表，会带来几个问题：

- 页面查询和修改接口更复杂
- 编排层合并逻辑更复杂
- 业务层读取策略更复杂
- 同类策略字段会散在不同表里

所以第一版更合适的方式是：

- 数据库里只保留一张统一策略覆盖表

## 3. 统一表里应该放什么

这张表里建议只放编排层公共策略字段。

例如：

- 是否启用
- 发送频率
- 渠道选择
- 接收对象
- 配额策略
- 模板绑定
- 少量扩展字段

这些字段表达的是：

- 某个租户对某类通知如何发送

它们适合统一建模。

## 4. 统一表里不应该放什么

这张表里不建议放：

- 业务查询条件
- 聚合 SQL
- 业务私有规则表达式
- 某个业务模块特有的数据生成逻辑

这些逻辑应该继续留在业务服务里。

原因是：

- 不同业务的数据模型差异太大
- 如果把生产规则也抽进一张表，最终会变成一个难维护的大杂烩

## 5. 推荐表名

建议表名：

- `aes_notify_strategy_override`

这个命名直接表达了它的含义：

- `notify`
- `strategy`
- `override`

也就是说，它不是默认表，也不是业务表，而是：

- 租户通知策略覆盖表

## 6. 推荐字段

建议字段如下：

- `id`
- `tenant_id`
- `message_type`
- `enabled_override`
- `frequency_override`
- `channels_override_json`
- `receivers_override_json`
- `quota_rule_override_json`
- `template_binding_override_json`
- `ext_json`
- `created_at`
- `updated_at`

## 7. 字段说明

### 7.1 基本标识

- `tenant_id`
- `message_type`

说明：

- `tenant_id` 标识租户
- `message_type` 标识通知类型

建议唯一键：

- `(tenant_id, message_type)`

### 7.2 覆盖字段

- `enabled_override`
- `frequency_override`
- `channels_override_json`
- `receivers_override_json`
- `quota_rule_override_json`
- `template_binding_override_json`

说明：

- 这些字段都表示“租户覆盖值”
- 如果字段为 `NULL`，表示该项没有覆盖，应回退到 `configmap`

### 7.3 扩展字段

- `ext_json`

说明：

- 用于容纳少量扩展策略
- 第一版尽量少用
- 不要把复杂业务规则全塞进去

## 8. OceanBase 建表示例

```sql
CREATE TABLE aes_notify_strategy_override (
    id BIGINT NOT NULL AUTO_INCREMENT,
    tenant_id BIGINT NOT NULL,
    message_type VARCHAR(64) NOT NULL,
    enabled_override TINYINT NULL,
    frequency_override VARCHAR(32) NULL,
    channels_override_json JSON NULL,
    receivers_override_json JSON NULL,
    quota_rule_override_json JSON NULL,
    template_binding_override_json JSON NULL,
    ext_json JSON NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_tenant_msg_type (tenant_id, message_type)
);
```

## 9. 读取逻辑

读取最终生效配置时，建议统一这样处理：

1. 先从 `configmap` 读取 `message_type` 默认配置
2. 再查 `aes_notify_strategy_override`
3. 如果该租户该类型存在记录，则按字段覆盖
4. 没有覆盖值的字段继续使用默认值
5. 返回最终生效配置

也就是说：

- 策略表只存差异
- 不是存一整份完整配置

## 10. 接口语义

结合前面已经确认的接口模型：

- `GET`
- `PUT`

这张表可以直接承接 `PUT` 的写入动作。

### GET

- 输入：`tenant_id`
- 输出：最终生效配置

处理逻辑：

- 读 `configmap`
- 读 `aes_notify_strategy_override`
- 合并后返回

### PUT

- 输入：`tenant_id + message_type + override`
- 行为：新增或更新对应覆盖配置

这里建议保持边界：

- `PUT` 只写 override
- 不改默认配置

## 11. 为什么批次表可以分两张，策略表却建议一张

这是两个不同层次的问题。

### 批次表

批次表承载的是：

- 执行通道
- 调度节奏
- 低延迟 vs 批量吞吐

所以分两张表是为了执行隔离。

### 策略表

策略表承载的是：

- 租户对某类通知的编排规则

这部分本质上是一类数据，所以更适合统一。

一句话理解：

- 批次分表，是因为执行模型不同
- 策略统一，是因为配置模型相同

## 12. 当前推荐结论

当前更合理的结构是：

- 默认策略：`configmap`
- 租户覆盖策略：`aes_notify_strategy_override`
- 响应式批次表：`aes_notify_realtime_batch`
- 后台批次表：`aes_notify_schedule_batch`

一句话总结：

在当前阶段，策略更适合统一成一张覆盖表，而批次更适合按执行通道拆成两张表；这两种“统一”和“拆分”并不矛盾，反而正好对应了不同层次的问题。 
