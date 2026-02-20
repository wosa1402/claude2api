# 版本回退记录

## 安全回退点

| 日期 | Commit | 说明 |
|---|---|---|
| 2026-02-20 | `eb54d00` | 修复 finish_reason 后的稳定版本（tool_calls 实现前） |

## 回退命令

```bash
git reset --hard eb54d00
```
