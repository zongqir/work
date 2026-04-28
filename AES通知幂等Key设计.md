# AES 通知幂等 Key 设计

## 1. 设计目标

幂等 key 的目标不是“让请求长得唯一”，而是：

- 让同一条通知语义在重跑、重试、重复下发时仍然映射到同一个稳定标识

也就是说，幂等 key 必须表达：

- 这条通知到底是哪一条业务通知

而不能表达：

- 这次请求是什么时候发的

## 2. 总原则

建议直接按通知类型拆成两类设计：

- 定时/汇总型通知：按窗口做幂等
- 响应式通知：按事件做幂等

一句话总结：

- 定时通知按窗口去重
- 响应式通知按事件去重

## 3. 定时/汇总型通知

### 3.1 核心原则

定时型通知的幂等 key，不应该基于“当前执行时间”，而应该基于：

- 这条通知所属的固定时间窗口

也就是说，哪怕这轮任务失败后重跑，或者延迟发送，只要它仍然是在处理同一个窗口，就应该使用同一个幂等 key。

### 3.2 10 分钟频率

如果频率是每 `10m`，那么幂等 key 应该绑定到：

- `window_start`

建议格式：

```text
sched:{messageType}:{tenantId}:10m:{windowStart}
```

示例：

```text
sched:security_alert_summary:tenant-1:10m:2026-04-14T06:10:00+08:00
```

如果当前时间是 `06:17`，它属于 `06:10 ~ 06:20` 窗口，那么使用的仍然是：

- `06:10` 这个窗口起点

到 `06:20` 之后，新窗口的 key 才会变成：

```text
sched:security_alert_summary:tenant-1:10m:2026-04-14T06:20:00+08:00
```

### 3.3 每小时频率

建议格式：

```text
sched:{messageType}:{tenantId}:1h:{hourStart}
```

示例：

```text
sched:audit_summary:tenant-1:1h:2026-04-14T06:00:00+08:00
```

### 3.4 每天固定时间

如果是每天固定时间，例如每天 `05:00` 执行，幂等 key 应该绑定到：

- 具体的调度点

建议格式：

```text
sched:{messageType}:{tenantId}:1d:{schedulePoint}
```

示例：

```text
sched:license_alert:tenant-1:1d:2026-04-14T05:00:00+08:00
```

这里不要只写 `day`，否则语义不够精确。

## 4. 响应式通知

### 4.1 核心原则

响应式通知的幂等 key，不应该绑定时间窗口，而应该绑定到：

- 具体业务事件

也就是说，重复触发同一个事件时，应该能映射到同一个幂等 key。

### 4.2 最优先使用事件 ID

如果业务天然有稳定事件 ID，那是最好的方案。

建议格式：

```text
rt:{messageType}:{tenantId}:{eventId}
```

示例：

```text
rt:new_alert:tenant-1:event-98765
```

### 4.3 如果没有事件 ID

如果没有天然事件 ID，可以退一步使用：

- `bizId + 变化语义`

建议格式：

```text
rt:{messageType}:{tenantId}:{bizId}:{eventSemantic}
```

示例：

```text
rt:task_status_change:tenant-1:task-123:pending->running
```

这里建议不要只写：

- `status=running`

因为语义不够强，最好写：

- `from->to`

或者某个明确版本号。

## 5. 不建议的设计

### 5.1 不要用当前执行时间

例如：

```text
tenantId + messageType + now()
```

这个设计的问题是：

- 每次重跑 `now()` 都变
- 同一条通知无法稳定幂等

### 5.2 不要用模糊时间粒度

例如：

```text
daily:tenant-1:messageType:2026-04-14
```

如果一天内存在多个调度点，这种设计就不够准确。

更合理的是：

- 带上具体调度点或窗口起点

### 5.3 不要只用 `status`

例如：

```text
tenantId + bizId + status
```

这个设计的问题是：

- 业务语义太弱
- 难以区分不同次变化

更合理的是：

- `eventId`
- 或 `from->to`
- 或版本号

## 6. 推荐的统一模板

### 6.1 定时/汇总型

```text
sched:{messageType}:{tenantId}:{freqType}:{windowStart}
```

### 6.2 响应式

```text
rt:{messageType}:{tenantId}:{bizIdOrEventId}:{eventSemantic}
```

如果有稳定 `eventId`，建议优先用 `eventId`，这样可以省掉 `eventSemantic` 的复杂度。

## 7. 与写放大的关系

合理的幂等 key 设计会直接影响系统是否需要很多中间状态写。

如果幂等 key 足够稳定，那么系统可以接受：

- 重跑同一窗口
- 重复下发同一事件

这样就不需要为了“绝不重复”去维护很多细粒度中间状态。

也就是说，幂等 key 设计得越稳定，系统越可以：

- 弱化中间状态
- 接受 at-least-once
- 把写放大收缩到真正关键的位置

## 8. 当前结论

AES 通知幂等 key 最合理的设计原则是：

- 定时型通知按窗口生成 key
- 响应式通知按业务事件生成 key

一句话总结：

不要让幂等 key 依赖“这次什么时候发的”，而要让它依赖“这条通知本来是哪一条”。
