# Web 控制台（简化版）设计文档

目标：提供一个极简 Web 页面，用于**查看账号可用状态**与**调用统计**（成功/失败/成功率/最近调用结果条），方便个人自用排障与观察。

## 1. 范围与原则

- 只做“看得见、能排障”的最小功能，不做多租户/权限系统/复杂图表。
- 不在页面/接口中输出明文 `sessionKey`、`apiKey`；所有敏感信息必须脱敏。
- 统计以“请求级”计数（每一次 `/v1/chat/completions` 视为一次调用）。
- 数据默认保存在内存（进程重启即清空）；如需持久化再扩展。

## 2. 页面结构（单页）

路径：`GET /admin`（返回静态 HTML）

页面布局建议（从上到下）：

1) 顶部概览（全局）
- 最近 24h（或最近 N 次）成功率：`success / (success+fail)`
- 累计成功/失败次数
- 当前启用账号数 / 总账号数
- 最近一次调用时间 & 最近一次错误原因（如果有）

2) 账号卡片列表（每个 session 一张卡）

每张卡显示：
- 账号标识：`sk-ant-****...****`（仅前 4 后 4）
- 状态徽标（颜色）：
  - `正常`：最近一次探测/调用成功，且近 N 次成功率 ≥ 阈值（如 50%）
  - `异常`：连续失败 ≥ K（如 3）或最近错误为 403/401
  - `冷却`（可选）：手动禁用/暂停
- 计数：成功次数 / 失败次数
- 成功率：百分比
- “最近调用记录条”（类似截图的小方块条）
  - 建议展示最近 30 次调用结果：成功=绿色，失败=红色，未发生=灰色
- 控件（尽量少）：
  - 开关：启用/禁用（可选；如你只想看状态，可不做）
  - “立即探测”按钮（可选）：对该账号发起一次最轻量的连通测试

> 说明：如果你只需要“显示状态 + 统计条”，可以只保留开关和探测按钮中的一个，甚至都不做。

## 3. 后端数据模型（内存）

建议新增一个简单的统计结构体（伪结构）：

- `AccountStat`
  - `maskedKey string`
  - `enabled bool`
  - `okCount int`
  - `failCount int`
  - `recent []bool`（固定长度环形数组，保存最近 30 次是否成功）
  - `lastError string`（脱敏后的错误类别，如 `403`, `timeout`, `orgid_failed`）
  - `lastSeenAt time.Time`

- `GlobalStat`
  - `totalOk int`
  - `totalFail int`
  - `lastRequestAt time.Time`
  - `lastError string`

数据更新点：
- 在请求处理链路中（`/v1/chat/completions`）：
  - 成功写入：账号 ok +1，recent push(true)
  - 失败写入：账号 fail +1，recent push(false)，记录错误类型

错误类型建议粗粒度分类（便于排障）：
- `403`（常见：云端 IP/风控）
- `401`（常见：session 失效/登录过期）
- `429`（限流）
- `timeout`
- `network`
- `unknown`

## 4. 接口设计（最小集）

1) `GET /admin/status`
- 返回 JSON，包含全局统计 + 账号列表
- 仅输出脱敏后的 `maskedKey`

示例（结构示意）：
```json
{
  "global": { "ok": 103, "fail": 8, "successRate": 0.927, "lastAt": "..." },
  "accounts": [
    {
      "maskedKey": "sk-a****mgAA",
      "enabled": true,
      "ok": 103,
      "fail": 8,
      "successRate": 0.60,
      "recent": [true,false,true],
      "lastError": "403",
      "lastAt": "..."
    }
  ]
}
```

2) （可选）`POST /admin/accounts/{id}/toggle`
- 仅用于启用/禁用某个账号
- `id` 可以是账号索引，避免暴露任何可逆标识

3) （可选）`POST /admin/accounts/{id}/probe`
- 主动探测：只调用 `GetOrgID` 或一次轻量请求，更新状态

## 5. 访问控制（强烈建议）

因为它涉及账号状态与调用行为，建议至少做以下之一：
- 仅监听内网：通过反代（Nginx）做 BasicAuth 或 IP 白名单
- 或新增 `ADMIN_KEY`：`/admin*` 需要 `Authorization: Bearer <ADMIN_KEY>`

## 6. MVP 里程碑（建议实现顺序）

1) 只读统计：内存统计 + `GET /admin/status`
2) 静态页面：`GET /admin` 渲染卡片与“最近条形块”
3) 可选交互：开关启用/禁用、立即探测

## 7. 你需要确认的 3 个问题（决定实现细节）

1) 你希望统计窗口是“最近 30 次调用”还是“最近 24 小时”？
2) 你需要“启用/禁用账号”的开关吗，还是只看状态即可？
3) `/admin` 是否需要单独的 `ADMIN_KEY`（推荐：需要）？

