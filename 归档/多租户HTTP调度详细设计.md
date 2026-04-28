# 多租户 HTTP 定时调度详细设计

本文讨论的场景是：

- 上游系统需要大量通过定时任务触发通知
- 下游统一是 `notification HTTP` 接口
- 调用粒度是“按租户触发一次 HTTP”
- 不考虑 MQ

目标是设计一套可长期演进的多租户调度系统，满足：

1. 每个租户频率不同
2. 不能打崩下游
3. 配置允许动态调整

## 1. 问题定义

如果简单做成：

- 每个租户一个 cron
- 到点后直接调一次 `notification HTTP`

很快会出现这些问题：

- 租户越多，定时任务越多，运维和排查成本线性上升
- 高频租户容易压满线程池和下游 HTTP
- 配置调整需要改代码或发版
- 租户调度延迟时，容易补出一堆历史任务，导致突发流量
- 某个租户异常重试时，会影响其他租户

所以这个问题的本质不是“写几个 cron”，而是：

- 多租户调度
- 多级限流
- 公平执行
- 动态配置
- 可观测与可补偿

## 2. 设计目标

### 2.1 功能目标

- 支持每个租户独立配置调度频率
- 支持启停单个租户
- 支持租户级限流、并发控制
- 支持全局限流、并发控制
- 支持 HTTP 失败重试
- 支持在线修改配置并快速生效

### 2.2 非功能目标

- 不因为单租户异常拖垮全局
- 不因为调度延迟形成补偿风暴
- 在租户规模扩大后仍然可控
- 任务状态可追踪，问题可审计

### 2.3 非目标

- 不解决 notification 内部发送逻辑
- 不解决微信、邮件渠道策略
- 不追求秒级精确实时调度
- 不实现复杂工作流编排

## 3. 总体设计

推荐拆成四层：

1. 配置层
2. 调度层
3. 执行层
4. 保护层

整体链路如下：

```text
tenant_schedule_config
        |
        v
    scheduler
        |
        v
http_schedule_task
        |
        v
 fair dispatcher
        |
        v
global limiter + tenant limiter + concurrency guard
        |
        v
notification HTTP
```

### 3.1 配置层

负责存储和下发租户维度调度配置。

### 3.2 调度层

负责判断“哪些租户现在应该执行”，并生成待执行任务。

### 3.3 执行层

负责按公平策略挑选任务，并执行 HTTP 调用。

### 3.4 保护层

负责对下游做保护：

- 全局限流
- 租户级限流
- 并发限制
- 熔断与退避

## 4. 核心原则

### 4.1 不为每个租户单独起 cron

调度器统一扫描配置表，按 `next_run_at` 生成任务。

### 4.2 调度与执行解耦

调度器不直接发 HTTP，只负责创建任务。  
真正调用下游 HTTP 的是 dispatcher。

### 4.3 单租户最多保留一个待执行任务

这是本设计最关键的原则之一。

原因是：

- 你调用的是“按租户一次 HTTP”
- 这种任务通常代表“触发该租户一次通知处理”
- 如果调度器滞后 30 分钟，没有必要为 1 分钟频率的租户补出 30 个历史任务

正确策略应该是：

- 同一租户同一任务类型，在 `ready/running/retry` 状态下最多保留 1 条活跃任务
- 新的调度时刻到来时，如果已有活跃任务，则不再新增任务，只更新下一次调度时间

这叫任务压缩或 backlog coalescing。

### 4.4 不能让大租户吃满全局资源

必须同时具备：

- 全局限流
- 租户级限流
- 公平调度

### 4.5 所有关键配置都必须动态化

不要把这些参数写死在代码里：

- 频率
- QPS
- 并发
- 超时
- 重试次数
- 熔断阈值

## 5. 数据模型

建议至少两张表：

1. 租户调度配置表
2. 调度任务表

如果后续需要更强治理，再加第三张执行日志表。

### 5.1 租户调度配置表

表名建议：`tenant_schedule_config`

字段建议：

- `id`
- `tenant_id`
- `job_type`
- `enabled`
- `interval_seconds`
- `jitter_seconds`
- `max_qps`
- `max_concurrency`
- `burst`
- `timeout_ms`
- `max_retry`
- `retry_policy_json`
- `priority`
- `circuit_breaker_enabled`
- `circuit_breaker_threshold`
- `circuit_breaker_cooldown_seconds`
- `next_run_at`
- `last_run_at`
- `config_version`
- `updated_at`
- `created_at`

字段说明：

- `tenant_id`
  - 租户标识
- `job_type`
  - 任务类型，便于后续一个租户支持多类定时任务
- `interval_seconds`
  - 调度间隔
- `jitter_seconds`
  - 随机抖动，避免大量租户同一秒触发
- `max_qps`
  - 该租户的最大发送速率
- `max_concurrency`
  - 该租户允许的并发数
- `burst`
  - 突发令牌容量
- `next_run_at`
  - 下次应执行时间
- `config_version`
  - 配置版本，便于追踪动态调整

索引建议：

- 唯一索引：`uk_tenant_job (tenant_id, job_type)`
- 普通索引：`idx_enabled_next_run (enabled, next_run_at)`

### 5.2 调度任务表

表名建议：`http_schedule_task`

字段建议：

- `id`
- `tenant_id`
- `job_type`
- `schedule_time`
- `status`
- `priority`
- `retry_count`
- `next_retry_at`
- `last_error`
- `http_status_code`
- `request_id`
- `config_version`
- `claimed_by`
- `claimed_at`
- `started_at`
- `finished_at`
- `updated_at`
- `created_at`

状态建议：

- `ready`
- `running`
- `success`
- `retry`
- `dead`
- `cancelled`

约束建议：

- 活跃任务唯一约束不能直接靠普通唯一索引表达，因为状态是变化的
- 推荐在调度生成时显式检查：
  - 同租户、同任务类型是否存在 `ready/running/retry`
  - 存在则不新增

索引建议：

- `idx_status_next_retry (status, next_retry_at, priority, id)`
- `idx_tenant_status (tenant_id, job_type, status)`
- `idx_claimed (claimed_by, claimed_at)`

### 5.3 可选执行日志表

表名建议：`http_schedule_task_log`

用于审计：

- 每次 HTTP 请求耗时
- 请求体摘要
- 错误码
- 返回体摘要

第一版不是必须，可以先用日志系统承接。

## 6. 调度器设计

### 6.1 调度器职责

调度器只负责：

1. 找到到期租户
2. 决定是否生成任务
3. 推进 `next_run_at`

调度器不负责：

- 真正发 HTTP
- 业务重试
- 复杂限流

### 6.2 调度周期

推荐每 `1s` 到 `5s` 扫描一次。

不建议每分钟扫一次，原因：

- 调度粒度太粗
- 高频租户会抖动很大
- 延迟感知不及时

### 6.3 调度 SQL

示意：

```sql
SELECT *
FROM tenant_schedule_config
WHERE enabled = 1
  AND next_run_at <= NOW(6)
ORDER BY next_run_at ASC
LIMIT 500;
```

### 6.4 生成任务的规则

对每条到期租户配置，执行：

1. 检查该租户该任务类型是否已有活跃任务
2. 如果没有，则插入一条 `ready` 任务
3. 如果已经有，则跳过本次任务创建
4. 无论是否创建任务，都推进 `next_run_at`

这里要强调：

**推进 `next_run_at` 不等于补历史。**

推荐推进规则：

- `next_run_at = now + interval + jitter`

不要用：

- `next_run_at = old_next_run_at + interval` 然后循环补齐

原因是后一种会在系统抖动后补出大量历史任务。

### 6.5 jitter 策略

为避免整点雪崩，建议每个租户配置一个抖动范围，例如：

- `jitter_seconds = 10`

生成下次时间时：

- `next_run_at = now + interval + random(0, jitter_seconds)`

如果希望调度更稳定，也可以对同一租户做固定抖动：

- `hash(tenant_id) % jitter_seconds`

这样每次分布固定，不会来回漂移。

## 7. 执行器设计

### 7.1 执行器职责

执行器负责：

1. 取出可执行任务
2. 抢占任务
3. 做限流检查
4. 发 HTTP
5. 更新任务状态

### 7.2 为什么不能简单并发扫表执行

如果多个 worker 同时扫 `ready` 任务，容易出现：

- 重复执行
- 同租户并发失控
- 全局吞吐突刺

所以需要“抢占 + 调度 + 限流”三件套。

### 7.3 任务抢占

先查任务：

```sql
SELECT *
FROM http_schedule_task
WHERE status IN ('ready', 'retry')
  AND next_retry_at <= NOW(6)
ORDER BY priority DESC, next_retry_at ASC, id ASC
LIMIT 200;
```

然后逐条抢占：

```sql
UPDATE http_schedule_task
SET status = 'running',
    claimed_by = ?,
    claimed_at = NOW(6),
    started_at = NOW(6),
    updated_at = NOW(6)
WHERE id = ?
  AND status IN ('ready', 'retry');
```

影响行数为 `1` 才表示抢占成功。

### 7.4 公平调度

如果只是简单按 `priority` 或 `id` 顺序处理，会出现：

- 高频大租户占满执行机会
- 低频小租户长期等待

推荐策略：

- 先从任务表里批量取出候选任务
- 按 `tenant_id` 分桶
- 执行器按 `round robin` 或 `weighted round robin` 从每个租户桶里取任务

推荐第一版做：

- 同一轮中，每个租户最多取 1 条任务
- 全局循环多轮

这样可以明显降低单租户霸占问题。

### 7.5 租户并发控制

执行任务前，需要判断该租户当前运行数是否超过阈值。

实现方式有两种：

1. 进程内计数器
2. 数据库状态统计

推荐：

- 进程内做快速控制
- 数据库状态做最终兜底

如果多实例部署，建议在任务抢占后再做一次租户并发检查：

- 超限则释放任务回 `retry`

## 8. 下游保护设计

这是本方案最重要的部分之一。

### 8.1 全局限流

必须有一个全局令牌桶，保护下游总体流量。

配置建议：

- `global_max_qps`
- `global_max_concurrency`
- `global_burst`

作用：

- 即使租户很多，也不会因为总量过大把 notification 打崩

### 8.2 租户级限流

每个租户单独一个令牌桶。

配置建议：

- `tenant.max_qps`
- `tenant.burst`

作用：

- 限制单租户峰值
- 防止“一个租户过于活跃”拖垮全局

### 8.3 双重准入

真正发 HTTP 前必须同时满足：

1. 拿到全局令牌
2. 拿到租户令牌
3. 未超过全局并发
4. 未超过租户并发

任一不满足，都不能直接发送。

不满足时建议：

- 将任务回写为 `retry`
- `next_retry_at = now + short_backoff`

### 8.4 熔断

如果某租户连续出现这些情况：

- HTTP 超时
- 429
- 5xx

达到阈值后，可以打开租户级熔断：

- 在 `cooldown` 时间内，不再真正发请求
- 任务直接延后

这样可以避免异常租户反复冲击下游。

### 8.5 自适应降速

如果全局层面观察到：

- 429 比例上升
- timeout 比例上升
- p99 显著增高

可以临时下调：

- 全局 QPS
- 全局并发

这属于运行时保护策略，建议支持动态配置。

## 9. HTTP 请求设计

### 9.1 请求约束

每个租户每次调度，调用一次统一 HTTP：

- URL 固定
- Header 固定格式
- Body 包含租户标识、任务类型、幂等键、追踪信息

建议公共字段：

- `tenantId`
- `jobType`
- `scheduleTime`
- `requestId`
- `idempotentKey`
- `traceId`

### 9.2 幂等

HTTP 调用必须带 `idempotentKey`。

推荐规则：

```text
tenant:{tenantId}:job:{jobType}:schedule:{scheduleTime}
```

例如：

```text
tenant:t1001:job:marketing_digest:schedule:20260410T160000
```

这样 HTTP 超时后可以安全重试。

### 9.3 成功语义

推荐定义：

- `2xx` = notification 已受理
- `4xx` = 请求有问题，通常不应无限重试
- `429` = 下游限流，应退避重试
- `5xx` = 下游异常，应重试
- `timeout` = 未知状态，按幂等重试

## 10. 重试策略

### 10.1 错误分类

建议分成四类：

1. 参数类错误
2. 下游限流类错误
3. 下游临时异常
4. 系统内部异常

### 10.2 处理建议

#### 参数类错误

例如：

- 请求体构造失败
- 必填字段为空
- 下游返回明确 `400`

处理：

- 直接 `dead`
- 不自动重试

#### 下游限流类错误

例如：

- `429`

处理：

- 重试
- 退避时间适当更长
- 可触发全局或租户降速

#### 下游临时异常

例如：

- `5xx`
- timeout
- 连接失败

处理：

- 指数退避或固定阶梯退避

#### 系统内部异常

例如：

- 抢占任务失败
- DB 短暂抖动

处理：

- 快速短退避重试

### 10.3 推荐退避

第一版可采用固定阶梯：

1. 10 秒
2. 30 秒
3. 1 分钟
4. 5 分钟
5. 15 分钟
6. 1 小时
7. 超过后 `dead`

## 11. 动态调整设计

“允许调整”在这个系统里必须是一级能力，而不是附属能力。

### 11.1 可动态调整项

- 租户启停
- 调度周期
- 租户 QPS
- 租户并发
- 全局 QPS
- 全局并发
- 超时
- 最大重试次数
- 熔断阈值
- 优先级

### 11.2 生效方式

推荐两种方式：

1. 配置表轮询刷新
2. 配置中心推送

第一版建议用：

- DB 配置表 + 本地缓存 + 每 5 秒刷新

这样简单稳妥。

### 11.3 修改原则

配置变更只影响：

- 新创建的任务
- 尚未执行的任务

已经在执行中的任务不强行中断。

### 11.4 紧急开关

必须支持：

- 全局暂停
- 单租户暂停
- 单任务类型暂停

这样在下游故障时可以快速止血。

## 12. 状态机设计

### 12.1 任务状态流转

```text
ready -> running -> success
ready -> running -> retry
retry -> running -> success
retry -> running -> retry
retry -> running -> dead
ready/retry -> cancelled
```

### 12.2 非法状态保护

必须避免：

- `success` 再次被执行
- `dead` 被正常 worker 再次捞起
- `running` 长时间卡死无人处理

### 12.3 running 超时回收

需要一个清理任务处理僵尸任务：

- 找到 `running` 且 `claimed_at` 超过阈值的任务
- 按策略恢复为 `retry`

例如：

- worker 崩溃
- HTTP 调用卡死
- 进程 OOM

## 13. 推荐 SQL 草图

### 13.1 取到期租户配置

```sql
SELECT tenant_id, job_type, interval_seconds, jitter_seconds, priority, config_version
FROM tenant_schedule_config
WHERE enabled = 1
  AND next_run_at <= NOW(6)
ORDER BY next_run_at ASC
LIMIT 500;
```

### 13.2 检查是否已有活跃任务

```sql
SELECT COUNT(1)
FROM http_schedule_task
WHERE tenant_id = ?
  AND job_type = ?
  AND status IN ('ready', 'running', 'retry');
```

### 13.3 插入 ready 任务

```sql
INSERT INTO http_schedule_task (
  tenant_id,
  job_type,
  schedule_time,
  status,
  priority,
  retry_count,
  next_retry_at,
  config_version,
  created_at,
  updated_at
) VALUES (?, ?, ?, 'ready', ?, 0, NOW(6), ?, NOW(6), NOW(6));
```

### 13.4 推进下次调度时间

```sql
UPDATE tenant_schedule_config
SET last_run_at = NOW(6),
    next_run_at = ?,
    updated_at = NOW(6)
WHERE tenant_id = ?
  AND job_type = ?;
```

### 13.5 成功回写

```sql
UPDATE http_schedule_task
SET status = 'success',
    request_id = ?,
    http_status_code = 200,
    finished_at = NOW(6),
    updated_at = NOW(6)
WHERE id = ?;
```

## 14. 监控设计

至少需要这些指标：

- 调度器每轮扫描数量
- 调度器每轮命中数量
- 新建任务数量
- 因已有活跃任务而被压缩的数量
- ready 任务堆积量
- running 任务数量
- retry 任务数量
- dead 任务数量
- HTTP 成功率
- HTTP p95/p99
- 429 数量
- 5xx 数量
- timeout 数量
- 租户级 QPS
- 租户级失败率

关键告警：

- `dead` 数量 > 0
- `ready/retry` 长时间堆积
- 某租户连续失败
- 全局 429 比例异常
- running 超时任务增加

## 15. 灰度与上线建议

不要全量直接切。

建议分阶段：

1. 先接配置表和调度表，只生成任务，不真正发 HTTP
2. 选少量租户灰度执行 HTTP
3. 打开监控，观察堆积、限流和重试情况
4. 逐步提升租户规模和全局限额
5. 最后再考虑熔断和自适应降速等高级能力

## 16. 推荐结论

这块不应该设计成“每个租户一个定时任务直接打 HTTP”。

推荐方案是：

- 配置表驱动的统一 scheduler
- 任务表承接 ready/running/retry/dead
- dispatcher 按公平策略执行
- 全局和租户双层限流
- 配置动态调整
- 活跃任务压缩，避免历史补偿风暴

一句话总结：

这不是一个 cron 问题，而是一个多租户调度系统问题。  
真正的关键不是“怎么定时”，而是“怎么在多租户、高频率、可调整的前提下，稳定地把 HTTP 流量送到下游而不把它打崩”。
