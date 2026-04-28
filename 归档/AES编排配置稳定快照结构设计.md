# AES 通知执行配置稳定快照结构设计

## 1. 设计目标

这一版专门解决一个很细但很关键的问题：

- 通知执行配置快照到底怎么建模，才能稳定、清晰、方便比较

这里有两个约束必须同时满足：

- 公共配置和租户配置要共用同一套 body/schema
- 不能靠 `TenantID=nil` 这种隐式语义区分“默认”和“覆盖”

一句话总结：

- body 统一
- 外层显式分组
- 快照本体稳定可比较

## 2. 核心原则

当前更推荐的原则是：

- 快照主结构不用大量 `map`
- 快照主结构不用隐藏语义
- 公共和租户共用一套策略 body
- `common` 和 `tenants` 由外层结构显式表达

## 3. 推荐快照结构

建议快照结构收敛成：

```go
type NotifyConfigSnapshot struct {
    LoadedAt time.Time
    Common   []NotifyStrategyBody
    Tenants  []TenantStrategySet
}
```

其中：

- `Common` 表示公共默认配置集合
- `Tenants` 表示租户级配置集合

## 4. 统一 body 结构

公共配置和租户配置共用同一个 body：

```go
type NotifyStrategyBody struct {
    MessageType     string
    Enabled         *bool
    Channels        []string
    Receivers       []string
    QuotaRule       *QuotaRule
    TemplateBinding *TemplateBinding
}
```

这个 body 的含义是：

- 它只描述“某类消息的发送配置内容”
- 它不负责表达“自己属于公共还是租户”

## 5. 租户分组结构

租户部分建议再显式按租户分组：

```go
type TenantStrategySet struct {
    TenantID string
    Items    []NotifyStrategyBody
}
```

这样结构会非常清楚：

- 快照里有哪些公共默认
- 哪些租户有特殊覆盖
- 每个租户覆盖了哪些消息类型

## 6. 为什么不建议 `TenantID=nil`

之前那种设计的问题不是“不能工作”，而是：

- 语义藏得太深
- debug 时不直观
- review 时容易误读

当前更好的做法就是：

- body 只管内容
- 外层结构直接说清楚它属于 `common` 还是 `tenants`

## 7. 稳定性要求

既然快照是为了 reload 和比较，那结构顺序必须稳定。

建议固定排序规则：

- `Common` 按 `MessageType` 排序
- `Tenants` 按 `TenantID` 排序
- `Tenant.Items` 按 `MessageType` 排序
- `Channels` 按字典序排序
- `Receivers` 按字典序排序
- `TemplateBinding` 内部如果是键值结构，也要转成稳定顺序后再比较

## 8. 快照怎么比较

如果快照已经被构造成稳定结构，那比较方式就很简单：

- 直接比较结构体内容

第一版完全可以使用：

- `reflect.DeepEqual`

或者：

- 序列化成稳定字节后比较

## 9. 查找性能怎么处理

快照主结构稳定，不代表运行时只能线性扫。

推荐做法是：

- 快照主结构保持稳定切片
- 加一层派生索引，仅用于运行时查找

例如：

```go
type SnapshotRuntimeIndex struct {
    CommonByMessageType map[string]int
    TenantByID          map[string]int
    TenantRuleByType    map[string]map[string]int
}
```

这里要强调：

- `index` 是派生数据
- 不是快照的主表达

## 10. 最终生效配置怎么取

读取某租户某消息类型的最终配置时，逻辑还是两步：

1. 从 `Common` 找该 `messageType`
2. 再看 `Tenants` 里该租户是否有同类型配置
3. 按字段覆盖得到最终生效值

## 11. 当前推荐结论

当前 AES 通知执行配置快照，推荐设计为：

- `NotifyConfigSnapshot{Common, Tenants}`
- 公共和租户共用同一个 `NotifyStrategyBody`
- 不用 `TenantID=nil` 这类隐式语义
- 快照主结构使用稳定切片表达
- 运行时查询通过派生索引补齐性能

一句话总结：

快照建模最重要的不是“少写几个结构体”，而是“语义显式、结构稳定、比较自然”；因此当前最合理的方式就是公共和租户共用一套 body，但由 `common + tenants` 外层显式分组。 
