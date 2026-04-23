# AES 通知执行配置快照模型设计

## 1. 设计目标

这一版解决的是：

- 默认策略在 `ConfigMap`
- 租户覆盖在 `OceanBase`
- 执行层如何低复杂度拿到“当前生效配置”

当前更推荐的思路不是：

- 每次发送时按需查 DB
- 每次发送时重新 parse `ConfigMap`
- 再额外引入一层缓存系统

而是：

- 周期性构建统一内存快照

## 2. 当前结论

每个参与通知执行的服务，周期性执行下面 3 步：

1. reload `ConfigMap`
2. reload DB 中的租户配置
3. 构建一份新的内存快照

快照统一按两层组织：

- `common`
- `tenants`

一句话总结：

- 默认值在 `common`
- 租户特殊值在 `tenants`
- 执行时只读快照

## 3. 快照结构

推荐结构如下：

```go
type NotifyConfigSnapshot struct {
    LoadedAt time.Time
    Common   []NotifyStrategyBody
    Tenants  []TenantStrategySet
}

type NotifyStrategyBody struct {
    MessageType     string
    Enabled         *bool
    Channels        []string
    Receivers       []string
    QuotaRule       *QuotaRule
    TemplateBinding *TemplateBinding
}

type TenantStrategySet struct {
    TenantID string
    Items    []NotifyStrategyBody
}
```

这套表达的关键点是：

- `common` 和 `tenants` 外层显式分开
- 公共和租户共用同一个策略 body
- 不靠 `TenantID=nil` 这种方式表达语义

## 4. 为什么这版更合适

### 4.1 执行路径固定

执行线程不需要再动态查：

- `ConfigMap`
- DB

而是只读当前内存快照。

### 4.2 逻辑更清楚

快照天然分成：

- 公共默认
- 租户特殊

后续合并逻辑非常直观。

### 4.3 适合执行层

执行层只关心：

- 当前某租户某消息类型到底该怎么发

它不需要知道业务什么时候触发，只需要知道发送时的最终配置。

## 5. reload 周期

当前建议第一版先定为：

- 每 `60s` reload 一次

这个周期的好处是：

- 足够简单
- 无需复杂事件机制
- 对当前通知执行场景够用

## 6. reload 流程

每次 reload 建议按固定顺序执行：

1. 读取共享 `ConfigMap`
2. 解析默认配置
3. 查询 DB 中全部租户特殊配置
4. 构建新的稳定快照
5. 与旧快照比较
6. 不同则原子替换

这里的关键点是：

- 不是边读边改当前快照
- 而是先完整构建新快照，再一次性切换

## 7. 为什么不单独维护 version

当前不推荐专门维护：

- `version`

原因很简单：

- 快照本身已经是稳定结构
- 直接比较新旧内容就够了

## 8. 稳定比较要求

为了让快照内容可直接比较，构建快照时要固定排序：

- `Common` 按 `MessageType` 排序
- `Tenants` 按 `TenantID` 排序
- `Tenant.Items` 按 `MessageType` 排序
- `Channels`、`Receivers` 按字典序排序

这样新旧快照在内容一致时，结构也一致。

## 9. 原子替换原则

快照替换必须原子完成。

原因是：

- 执行线程不能读到半更新状态

所以推荐：

1. 构建新快照
2. 构建运行时索引
3. 最后原子替换当前引用

## 10. 执行时怎么读

执行层真正发送时，不再查配置源。

而是：

1. 获取当前 snapshot
2. 根据 `tenantID + messageType` 计算最终生效配置
3. 按最终配置决定渠道、模板和接收对象

## 11. 最终生效配置怎么合并

读取逻辑保持简单：

1. 先取 `common` 里的对应 `messageType`
2. 再看该租户在 `tenants` 中是否存在同类策略
3. 对存在的字段做覆盖
4. 得到最终生效配置

## 12. 为什么当前不建议额外引缓存

这版虽然也有内存数据，但它不是传统意义上的业务缓存。

这里做的是：

- 周期性构建完整快照

而不是：

- 每次按需查询后做 TTL 缓存

所以当前不建议再叠加：

- 本地 TTL 缓存
- 多级缓存
- 按租户懒加载缓存

## 13. 配置变化后如何生效

默认值变化和租户覆盖变化，都会在下一次 reload 时进入新快照。

在纯通知执行层定位下，这种变化只意味着：

- 后续新的发送动作按新配置执行

它不再意味着：

- 重算业务调度
- 重算业务窗口
- 重算业务 checkpoint

## 14. 当前适用边界

这版更适合当前阶段，因为：

- 当前不想引入额外缓存复杂度
- 当前更看重实现简单和行为一致
- 当前重点是通知执行，不是业务调度

## 15. 当前结论

当前 AES 通知执行配置最合理的实现方式是：

- 共享 `ConfigMap` 提供默认配置
- DB 提供租户特殊配置
- 每 `60s` reload 一次
- 构建 `common + tenants` 稳定内存快照
- 执行时只读当前快照

一句话总结：

相比按需查表、复杂缓存、额外版本管理，`60s reload + 稳定快照 + 原子替换` 是当前阶段最干净、最容易解释、也最容易落地的执行配置实现。 
