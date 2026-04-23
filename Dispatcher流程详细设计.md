# Dispatcher 流程详细设计

本文只聚焦一个问题：

- 在多租户 HTTP 定时调度方案里
- dispatcher 到底应该怎么工作

这里不再展开 scheduler、配置中心、业务模型，只讨论 dispatcher 如何从任务表中取任务、保护下游、执行 HTTP，并正确回写状态。

## 1. dispatcher 的职责边界

dispatcher 只负责五件事：

1. 从任务表中找到当前可执行任务
2. 抢占任务，避免重复执行
3. 控制全局和租户级流量
4. 调用下游 `notification HTTP`
5. 根据结果回写任务状态

dispatcher 不负责：

- 生成调度任务
- 决定某个租户多久执行一次
- 渲染业务内容
- 处理 notification 内部重试

一句话：

dispatcher 是任务执行器，不是调度决策器。

## 2. dispatcher 的基本模型

dispatcher 推荐做成一个长期运行的 worker loop。

逻辑模型如下：

```text
轮询任务表
  -> 抢占任务
  -> 放入本地候选池
  -> 限流检查
  -> HTTP 调用
  -> 回写结果
  -> 继续下一轮
```

这不是数据库监听，也不是触发器，而是：

- 主动轮询
- 显式抢占
- 状态驱动执行

## 3. 为什么第一版推荐扫表

原因很现实：

- 当前没有 MQ
- 任务状态要可见
- 需要支持重试和死信
- 需要控制多租户公平性
- 需要支持运行时调整

任务表有这些天然优势：

- 状态清晰
- 故障可恢复
- 易做重试
- 易做审计
- 易做限流前置判断

所以第一版最稳的路径就是：

- scheduler 写表
- dispatcher 扫表

## 4. 任务状态机

建议任务状态如下：

- `ready`
- `running`
- `success`
- `retry`
- `dead`
- `cancelled`

状态流转建议：

```text
ready -> running -> success
ready -> running -> retry
retry -> running -> success
retry -> running -> retry
retry -> running -> dead
ready/retry -> cancelled
```

其中：

- `ready` 表示待执行
- `running` 表示已被某个 worker 抢占
- `retry` 表示失败但可继续重试
- `dead` 表示终态失败

## 5. dispatcher 主流程

dispatcher 一轮处理建议分成 6 步：

1. 查询候选任务
2. 抢占任务
3. 组装公平执行队列
4. 做准入检查
5. 调 HTTP
6. 回写状态

### 5.1 查询候选任务

只查询当前可执行任务：

- `status in ('ready', 'retry')`
- `next_retry_at <= now`

SQL 示意：

```sql
SELECT id, tenant_id, job_type, priority, retry_count, next_retry_at
FROM http_schedule_task
WHERE status IN ('ready', 'retry')
  AND next_retry_at <= NOW(6)
ORDER BY priority DESC, next_retry_at ASC, id ASC
LIMIT 200;
```

注意：

- 第一版可以先简单排序
- 真正公平调度在查询后再做，不建议完全靠 SQL

### 5.2 抢占任务

查询出来不代表你能执行。

必须做状态抢占：

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

只有影响行数为 `1`，该任务才属于当前 worker。

这一步的目的很直接：

- 多实例并发时防止重复执行

### 5.3 组装公平执行队列

假设一轮抢到 100 条任务，不能直接按顺序全打出去。

推荐先按 `tenant_id` 分桶：

```text
tenantA: [task1, task9, task20]
tenantB: [task2, task5]
tenantC: [task3, task7, task8, task10]
```

然后 round robin 取任务：

```text
tenantA.task1
tenantB.task2
tenantC.task3
tenantA.task9
tenantB.task5
tenantC.task7
...
```

这样做的收益是：

- 高频租户不会立刻吃满本轮执行机会
- 小租户不会长期饥饿

第一版不一定要做 weighted round robin，普通 round robin 就够有价值。

### 5.4 准入检查

真正发 HTTP 前，必须同时通过四个门：

1. 全局 QPS 令牌
2. 租户 QPS 令牌
3. 全局并发余量
4. 租户并发余量

任何一个不满足，都不要直接发 HTTP。

处理建议：

- 将任务改回 `retry`
- `next_retry_at = now + short_backoff`
- `last_error = 'rate limited locally'` 或类似说明

也就是说：

dispatcher 不是“抢到任务就一定发”，而是“抢到任务后还要做本地准入控制”。

### 5.5 调 HTTP

准入成功后，执行下游 HTTP 调用。

建议请求里至少带这些字段：

- `tenantId`
- `jobType`
- `scheduleTime`
- `requestId`
- `idempotentKey`
- `traceId`

其中最关键的是：

- `idempotentKey`

推荐格式：

```text
tenant:{tenantId}:job:{jobType}:schedule:{scheduleTime}
```

这样 HTTP timeout 之后可以安全重试。

### 5.6 回写状态

HTTP 调用之后，必须明确分类：

- 成功
- 可重试失败
- 不可重试失败

#### 成功

```sql
UPDATE http_schedule_task
SET status = 'success',
    request_id = ?,
    http_status_code = ?,
    finished_at = NOW(6),
    updated_at = NOW(6),
    last_error = ''
WHERE id = ?;
```

#### 可重试失败

```sql
UPDATE http_schedule_task
SET status = 'retry',
    retry_count = ?,
    next_retry_at = ?,
    http_status_code = ?,
    last_error = ?,
    updated_at = NOW(6)
WHERE id = ?;
```

#### 终态失败

```sql
UPDATE http_schedule_task
SET status = 'dead',
    retry_count = ?,
    http_status_code = ?,
    last_error = ?,
    finished_at = NOW(6),
    updated_at = NOW(6)
WHERE id = ?;
```

## 6. HTTP 结果分类建议

dispatcher 的判断不要模糊。

建议明确成三类：

### 6.1 成功

- HTTP `2xx`

含义：

- notification 已受理

结果：

- `success`

### 6.2 可重试失败

包括：

- `429`
- `5xx`
- timeout
- connect reset
- temporary DNS/network error

结果：

- `retry`

### 6.3 不可重试失败

包括：

- 明确参数错误
- 下游返回不可恢复 `4xx`
- 本地构造请求失败

结果：

- `dead`

## 7. retry 设计

第一版建议固定阶梯退避，不要一开始就做太复杂：

1. 10 秒
2. 30 秒
3. 1 分钟
4. 5 分钟
5. 15 分钟
6. 1 小时
7. 超过阈值进入 `dead`

注意：

- 对 `429` 可以单独用更长退避
- 对本地限流导致的 defer，可以用更短退避，例如 3 到 5 秒

## 8. 为什么抢占后还会回退成 retry

这是很多人第一次设计 dispatcher 时容易别扭的地方。

他们会觉得：

- 既然任务已经 `running`
- 为什么还能改回 `retry`

原因是：

- `running` 只表示“当前 worker 拿到了执行权”
- 不代表“这次一定实际发起了 HTTP”

例如这些情况都可能在 `running` 后回退：

- 本地全局限流没拿到令牌
- 租户并发已满
- 请求构造发现依赖数据缺失
- worker 在真正发出请求前发现租户已被暂停

所以 `running` 不是成功执行，只是已被占有。

## 9. running 卡死怎么处理

这块必须有兜底。

比如：

- worker 抢占成功后进程崩溃
- HTTP 长时间阻塞
- goroutine 泄漏

需要一个 recovery job 定期扫描：

```sql
SELECT id
FROM http_schedule_task
WHERE status = 'running'
  AND claimed_at <= DATE_SUB(NOW(6), INTERVAL 5 MINUTE);
```

然后按策略回收：

- 改回 `retry`
- `next_retry_at = now + short_backoff`
- `last_error = 'worker timeout recovery'`

这一步不做，系统会逐渐积累永久卡死任务。

## 10. dispatcher 的 worker loop 建议

可以抽象成下面这个循环：

```text
for {
  1. 扫候选任务
  2. 抢占一批任务
  3. 按 tenant 分桶
  4. round robin 组装待执行列表
  5. 启动受控并发执行
  6. 每条任务：
     - 过全局限流
     - 过租户限流
     - 过并发控制
     - 发 HTTP
     - 回写状态
  7. 没任务则 sleep 短时间
}
```

推荐参数：

- dispatcher 主轮询间隔：`500ms` 到 `2s`
- 每轮候选任务上限：`100` 到 `500`
- worker 并发数：由全局并发配置决定

## 11. 一个更贴近实现的伪代码

```go
func (d *Dispatcher) Run(ctx context.Context) {
    for {
        tasks := d.repo.ListRunnable(ctx, 200)
        if len(tasks) == 0 {
            sleep(1 * time.Second)
            continue
        }

        claimed := make([]Task, 0, len(tasks))
        for _, task := range tasks {
            ok := d.repo.TryClaim(ctx, task.ID, d.workerID)
            if ok {
                claimed = append(claimed, task)
            }
        }

        buckets := bucketByTenant(claimed)
        queue := roundRobin(buckets)

        for _, task := range queue {
            if !d.globalLimiter.Allow() {
                d.repo.MarkRetry(ctx, task.ID, shortBackoff(), "global rate limited")
                continue
            }

            if !d.tenantLimiter(task.TenantID).Allow() {
                d.repo.MarkRetry(ctx, task.ID, shortBackoff(), "tenant rate limited")
                continue
            }

            if !d.globalConcurrency.TryAcquire() {
                d.repo.MarkRetry(ctx, task.ID, shortBackoff(), "global concurrency limited")
                continue
            }

            if !d.tenantConcurrency(task.TenantID).TryAcquire() {
                d.repo.MarkRetry(ctx, task.ID, shortBackoff(), "tenant concurrency limited")
                d.globalConcurrency.Release()
                continue
            }

            go d.execute(ctx, task)
        }
    }
}
```

`execute()` 内部再做：

- 构造 HTTP 请求
- 调用 notification
- 按结果 MarkSuccess/MarkRetry/MarkDead
- 释放并发令牌

## 12. 监控重点

dispatcher 这层建议重点看这些指标：

- 每轮查询任务数
- 每轮抢占成功数
- 每轮回退 retry 数
- HTTP success/retry/dead 数
- 本地全局限流命中数
- 本地租户限流命中数
- 本地全局并发拒绝数
- 本地租户并发拒绝数
- running 超时回收数
- 单租户连续失败数

这些指标能直接反映 dispatcher 是不是设计对了。

## 13. 推荐结论

你现在的直觉“先写表，然后 dispatcher 持续处理表里的数据”是对的。

但准确说法应该是：

- scheduler 负责产任务
- dispatcher 负责轮询任务表、抢占任务、做限流、公平执行 HTTP、回写状态

dispatcher 的真正技术重点不是扫表本身，而是这五件事：

1. 抢占，避免重复执行
2. 公平，避免大租户压死小租户
3. 限流，避免打崩下游
4. 重试，处理 timeout/429/5xx
5. 回收，处理 running 卡死

一句话总结：

dispatcher 可以先做成“扫表执行器”，这是对的。  
但它绝不是一个简单的 while 循环调 HTTP，而是一套带状态机、抢占、限流、公平性和重试能力的执行系统。
