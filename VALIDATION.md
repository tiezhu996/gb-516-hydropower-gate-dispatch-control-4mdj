# 验收记录

验收日期：2026-08-22（Asia/Shanghai）

## 静态质量

- `go test ./...`：通过。
- `go test -race ./...`：通过，无数据竞争。
- `go vet ./...`：通过。
- `go build ./...`：通过。
- `npm run typecheck`：通过。
- `npm run build`：通过。
- `docker compose config --quiet`：通过。
- 规模：38 个非测试 `.go` 文件、3167 行非测试 Go 代码，符合 30-42 文件和 3000-4200 行要求。

## 空卷部署与 API

使用 `KEEP_RUNNING=1 ./scripts/validate.sh` 删除旧卷后重新构建。PostgreSQL、Redis、backend、frontend 均进入 healthy，`/healthz` 返回数据库和 Redis ready。

- `viewer` 可读取四类实体；写入和审计 API 均返回 403。
- `reviewer` 可读取审计 API。
- `operator` 创建指令后完成 `draft → pending`，审批证据保存 `val-submit-*` 请求 ID。
- `operator` 尝试批准待审指令返回 403。
- `reviewer` 完成 `pending → approved`，提交人与批准人不同，审批证据保存 `val-approve-*` 请求 ID。
- 批准后的指令尝试编辑返回 409；随后由操作员完成 `approved → executing → completed`。
- 执行回执完成 `pending → confirmed`，保存 `confirmedBy` 与 `confirmedAt`。
- 关闭闸门直接迁移为开启返回 422，必须先经过 `moving`。
- 运行配置响应带 `X-Request-ID`，审计列表可检索到提交、审批和执行事件。

## 内置 Browser

全程使用 Codex 内置 Browser，未使用外部 Chrome。

- `/reservoirs`：页面与数据正常；通过新增表单创建记录并在列表看到结果。
- `/gates`：`GateStateBadge` 正常；实际完成 `GU-002 closed → moving`。
- `/directives`：双人确认轨迹、闸门快照与角色按钮正常；复核员实际完成 `OD-002 pending → approved`。
- `/confirmations`：共享 `DirectiveTimeline` 展示关联指令、确认人和时间。
- `/audit`：统计、请求 ID 列表和 `GateUnit` 筛选正常。
- 切换 `viewer` 后，新增/迁移按钮和审计导航消失；直接访问 `/audit` 被路由守卫重定向到 `/reservoirs`。
- 390 x 844 视口下 `innerWidth` 与页面宽度均为 390，无页面级横向溢出；桌面与移动端截图无文字遮挡。
- 浏览器控制台 error/warning：0。

## 清理

执行 `docker compose down -v --remove-orphans` 后，本项目容器、网络和两个命名卷均已删除；`docker compose ps -a` 为空。
