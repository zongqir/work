# AES 通知执行配置设计

## 1. 设计目标

这一版配置设计遵循一个更轻也更准确的原则：

- 默认配置放在 `ConfigMap`
- 单租户特殊配置放在 `OceanBase`
- 页面和执行层读取的都是最终生效配置

同时要明确：

- 这里只配置通知执行域
- 不配置业务调度域

也就是说，当前不做“业务几点跑、多久汇总一次”这类配置，而是先收敛成：

- `default config`
- `tenant override`

最终生效规则为：

- 先查租户 `override`
- 有覆盖就按覆盖生效
- 没覆盖就降级使用 `ConfigMap` 默认值

## 2. 为什么这样设计

这套设计更适合当前阶段，原因有几个：

- 大多数配置其实是全局默认值，没必要全部入库
- 真正需要特殊处理的只是少数租户
- 页面仍然需要查看和修改租户级配置
- 执行层需要读取最终生效配置后再发送
- 整体复杂度明显低于“默认 + 租户组 + 租户例外 + 业务调度”这种更重的模型

一句话说：

- 默认值走 `ConfigMap`
- 特殊值走数据库
- 生效值由配置层统一计算

## 3. 配置范围

第一版建议只把真正需要运行时覆盖的通知执行配置纳入这层：

- 是否启用某类通知
- 默认渠道
- 接收对象
- 配额策略
- 模板绑定关系

其中：

- 全局默认值写在 `ConfigMap`
- 单租户例外值写在数据库

这里要明确排除：

- 业务巡检时间
- 业务汇总频率
- 业务补跑策略

这些不属于 AES 通知执行层。

## 4. 默认配置放在 ConfigMap

`ConfigMap` 里建议维护每类通知的默认规则。

示意结构可以是：

```yaml
notify:
  defaults:
    security_alert:
      enabled: true
      channels:
        - wechat_work
        - email
      receivers:
        - admin
      quota_rule:
        wechat_daily_limit: 5
      template_binding:
        wechat_work: tpl_security_alert_wecom
        email: tpl_security_alert_email
    audit_event:
      enabled: true
      channels:
        - email
      receivers:
        - admin
      template_binding:
        email: tpl_audit_event_email
```

这里的原则是：

- `ConfigMap` 只存默认值
- 它面向所有租户
- 它不存单租户特殊配置

## 5. 租户覆盖配置放在 OceanBase

数据库里只存“单租户 override”。

表名建议：

- `aes_notify_tenant_config_override`

建议字段：

- `id`
- `tenant_id`
- `message_type`
- `enabled_override`
- `channels_override_json`
- `receivers_override_json`
- `quota_rule_override_json`
- `template_binding_override_json`
- `created_at`
- `updated_at`

字段说明：

- 只存覆盖字段
- 没有覆盖的字段可以为 `NULL`
- 查询时按字段回退到 `ConfigMap`

唯一键建议：

- `uk_tenant_msg_type (tenant_id, message_type)`

## 6. OceanBase 建表示例

```sql
CREATE TABLE aes_notify_tenant_config_override (
    id BIGINT NOT NULL AUTO_INCREMENT,
    tenant_id BIGINT NOT NULL,
    message_type VARCHAR(64) NOT NULL,
    enabled_override TINYINT NULL,
    channels_override_json JSON NULL,
    receivers_override_json JSON NULL,
    quota_rule_override_json JSON NULL,
    template_binding_override_json JSON NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_tenant_msg_type (tenant_id, message_type)
);
```

## 7. 生效配置合并规则

配置层提供统一合并逻辑：

1. 先从 `ConfigMap` 读取该 `message_type` 的默认配置
2. 再查询该租户该 `message_type` 是否存在 `override`
3. 对存在覆盖的字段使用 `override`
4. 对不存在覆盖的字段继续使用默认值
5. 返回最终生效配置

例如：

- 默认渠道是 `["wechat_work", "email"]`
- 某租户覆盖成 `["email"]`

最终生效：

- `["email"]`

再例如：

- 默认接收对象是 `["admin"]`
- 某租户没有覆盖接收对象

最终生效：

- `["admin"]`

## 8. 接口设计

这一版建议只保留两个接口：

- `GET`
- `PUT`

### 8.1 GET 接口

接口语义：

- 输入：`tenant_id`
- 输出：该租户所有消息类型的最终生效配置

处理逻辑：

1. 先取 `ConfigMap` 默认值
2. 再查该租户的 `override`
3. 合并后返回

建议接口形态：

```text
GET /aes/notify/config?tenantId=xxx
```

返回示意：

```json
{
  "tenantId": 1001,
  "configs": [
    {
      "messageType": "security_alert",
      "enabled": true,
      "channels": ["wechat_work", "email"],
      "receivers": ["admin"],
      "quotaRule": {
        "wechatDailyLimit": 5
      },
      "templateBinding": {
        "wechat_work": "tpl_security_alert_wecom",
        "email": "tpl_security_alert_email"
      }
    }
  ]
}
```

### 8.2 PUT 接口

接口语义：

- 输入：`tenant_id + message_type + override fields`
- 行为：写入或更新该租户该类型的覆盖配置

建议接口形态：

```text
PUT /aes/notify/config
```

请求示意：

```json
{
  "tenantId": 1001,
  "messageType": "security_alert",
  "override": {
    "enabled": true,
    "channels": ["email"]
  }
}
```

这里的边界建议明确：

- `PUT` 只改租户 `override`
- 不改 `ConfigMap` 默认值
- 默认值仍通过配置发布流程维护

## 9. 如何恢复默认

既然第一版只保留 `GET/PUT`，那“恢复默认”也通过 `PUT` 表达。

建议方式是：

- `PUT` 传空的 `override`
- 或对某个字段传 `null`

由服务端解释为：

- 清除该字段的租户覆盖
- 后续读取时回退到 `ConfigMap`

例如：

```json
{
  "tenantId": 1001,
  "messageType": "security_alert",
  "override": {
    "channels": null
  }
}
```

处理后表示：

- `channels` 不再走租户覆盖
- 回退到默认配置

如果某个租户该类型下所有字段都被清空，可以顺手删掉整条 `override` 记录。

## 10. 执行层如何使用

执行层不需要感知：

- 默认值来自 `ConfigMap`
- 覆盖值来自数据库

执行层只需要拿最终生效配置即可。

例如：

- `GetEffectiveNotifyConfig(tenantId, messageType)`

返回内容应直接可用：

- `enabled`
- `channels`
- `receivers`
- `quota_rule`
- `template_binding`

然后执行层负责：

- 判断是否允许发送
- 渲染模板
- 调用 `notification`

## 11. 为什么不建议把默认值也全部入库

如果默认值也全部放数据库，第一版会多出很多额外复杂度：

- 还要维护默认值编辑入口
- 还要考虑默认值变更审批或发布
- 还要区分“默认值不存在”和“默认值关闭”
- 还要做更多管理界面

而你当前真正需要的其实是：

- 默认值稳定存在
- 个别租户支持覆盖

所以更轻的方案就是：

- 默认值放 `ConfigMap`
- 运行时只管租户覆盖

## 12. 当前推荐结论

当前阶段，AES 通知执行配置最合适的做法是：

- 默认配置写在 `ConfigMap`
- 租户例外配置写在 `OceanBase`
- `GET` 返回最终生效配置
- `PUT` 写租户覆盖配置

一句话总结：

配置层不用做重，先把“默认值走 `ConfigMap`，特殊值走 `override`，读取时自动降级”这条链路打通，就已经足够支撑当前 AES 的通知执行需求。 
