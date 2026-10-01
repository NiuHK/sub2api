# 分支开发跟踪：`feat/quota-percentage-allocation`

更新时间：2026-10-01  
当前状态：功能实现阶段暂告一段落，等待具备 Go 工具链的环境完成后端验证，再安排 api2 发布。

## 已完成

- 按分组内用户的 `ActualCost` 占比，结合 OpenAI OAuth 账号当前 5 小时和 7 天窗口使用比例，计算用户的估算使用比例。
- 在 Chat Completions、Images、Embeddings、Alpha Search、Live 等 OpenAI 请求入口补齐配额检查；Responses、Messages、WebSocket 等已有入口继续复用统一检查逻辑。
- 账号返回 `limit_reached` 时直接阻止分组继续放行；分组金额为零时按零占比处理。
- 修正上游窗口边界：优先使用 `ResetAt`，缺少时使用 `FetchedAt + ResetAfterSeconds`，过期边界按完整窗口推进，避免跨窗口混入旧消耗。
- 将分组速率/配额配置同步改为事务处理，并使用 PostgreSQL 事务级 advisory lock，避免并发更新产生半成品配置。
- 保留只有百分比配额、没有速率倍率的用户配置，管理端可以读取、编辑和展示该类配置。
- 增加数据库迁移 `243_user_group_openai_quota_percentage.sql` 和 `244_user_group_openai_quota_windows.sql`。
- 增加后端契约/仓储/配额窗口测试，以及管理端组件测试；前端定向测试和 `vue-tsc --noEmit` 已通过。

## 尚未完成

- 当前工作区没有 Go 工具链，后端 Go 测试和格式化尚未执行；发布前必须在 api2 构建环境或 CI 中补跑。
- 暂不改变现有单 OAuth 账号分组约束。多账号、故障转移、跨账号配额迁移仍不在本阶段范围内。
- 配额查询会调用上游 `/wham/usage`，单次查询超时上限为 20 秒；查询失败按现有策略放行，但高并发配置用户可能增加延迟和上游请求压力。

## api2 发布检查

1. 先备份生产数据库，再让新镜像启动自动执行 243、244 迁移。
2. 必须提交并构建包含迁移文件、wire 生成文件和前后端改动的新镜像；旧镜像不会回滚数据库迁移。
3. 发布期间避免新旧镜像混跑。旧代码保存分组配置时可能删除新增加的“仅配额百分比”用户行。
4. 先对一个目标分组灰度，观察 `/wham/usage` 延迟、配额拒绝日志和数据库迁移结果，再扩大范围。
5. 确认目标分组满足现有前置条件：只有一个 OpenAI OAuth 账号，且启用了 OAuth-only 配置；否则配额门禁会按既有规则跳过。

## 当前运行环境

- 已停止：`sub2api-api3-test.service`、`sub2api-api3-redis`、`sub2api-api3-postgres`。
- 保持运行：api2 的 `sub2api`、`sub2api-postgres`、`sub2api-redis` 及其 Caddy 依赖。
