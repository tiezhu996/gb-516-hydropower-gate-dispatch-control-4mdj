
# 水电站闸门调度许可

水电站库区、闸门、操作指令与执行确认安全控制平台。项目采用前后端分离和明确的领域分层，重点保证状态迁移、RBAC、审计日志、请求追踪与限流在各层保持一致。

## Docker Compose 快速启动

```bash
cp .env.example .env
docker compose up -d --build
docker compose ps
```

- Web 工作台：http://127.0.0.1:18516
- 后端健康检查：http://127.0.0.1:19516/healthz
- 后端 API：http://127.0.0.1:19516/api
开发模式的四个验收账号共用本地密码 `Admin123!`：

| 账号 | 角色 | 主要权限 |
|---|---|---|
| `viewer` | 值班观察员 | 查看库区、闸门、指令和回执 |
| `operator` | 现场操作员 | 新建、提交、执行和填写执行回执 |
| `reviewer` | 安全复核员 | 独立批准/中止待审指令、查看审计 |
| `admin` | 系统管理员 | 系统治理及删除草稿记录 |

开发环境额外创建 `viewer`。生产空库会初始化 `admin`、`operator`、`reviewer` 三个职责分离账号，并强制要求三组互不相同的 12 位以上启动密码和至少 32 位的非占位 `JWT_SECRET`；已停用或降权账号的旧 Token 会在下一次请求立即失效。

停止并清理本项目容器与数据卷：

```bash
docker compose down -v --remove-orphans
```

## 主要功能

| 业务模块 | 后端实体 | API 前缀 | 状态流 |
|---|---|---|---|
| 库区 | `Reservoir` | `/api/reservoirs` | normal, warning, critical, restricted |
| 闸门 | `GateUnit` | `/api/gates` | open, closed, moving, locked |
| 操作指令 | `OperationDirective` | `/api/directives` | draft, pending, approved, executing, completed, aborted |
| 执行确认 | `ExecutionConfirmation` | `/api/confirmations` | pending, confirmed, failed, cancelled |

- JWT 登录和 viewer/operator/reviewer/admin 四级 RBAC；写接口在 Gin 路由层再次校验角色。
- 指令严格按 `draft → pending → approved → executing` 推进，不允许跳过审批；`completed` 只能由有效执行回执原子触发，任意活动阶段可按权限中止。
- 提交人与批准人必须是两个不同账号。`DirectiveApproval` 逐条保存角色、意见、请求 ID 和时间，前端显示完整轨迹。
- 所有状态变化使用乐观锁，并将业务状态、审批证据和不可覆盖审计日志放在同一个数据库事务中。
- 库区、闸门、指令和执行回执逐级校验权威关联；不存在、跨区域或闭锁的对象不能进入下游流程。
- 指令开始执行时闸门原子进入 `moving`；成功回执同时完成指令并落定目标闸位，失败回执同时中止指令并闭锁闸门。
- 闸门执行权锁：同一闸门同一时刻只允许一条已批准指令进入 `executing`。设备被占用时返回 409 并携带占用方指令编号，原指令保持 `approved` 不变；两个开工请求并发时由数据库部分唯一索引（`status = 'executing'` 按闸门去重）保证只有一个拿到执行权。指令完成、中止或回执失败都会自动释放执行权，指令页实时显示各闸门的当前占用指令与执行状态。
- 请求 ID、结构化日志、全局错误映射和 Redis 分布式限流。
- 提供脱敏运行配置、当前会话、审计汇总和单实体审计历史接口。
- 业务工作台支持查询、新建、状态推进、风险标识及操作审计查看。

## 技术栈

| 层次 | 技术 |
|---|---|
| 前端 | Vue 3 + TypeScript + Vite + Element Plus |
| 后端 | Go 1.22 + Gin + GORM |
| 数据 | PostgreSQL + Redis |
| 部署 | Docker Compose + Nginx |

## 本地开发

后端可使用 SQLite 开发模式，不需要先启动数据库：

```bash
cd backend
go mod download
DATABASE_DRIVER=sqlite DATABASE_DSN=local.db REDIS_ADDR='' \
JWT_SECRET=local-development-secret PORT=8080 go run ./cmd/server
```

前端开发服务器：

```bash
cd frontend
npm install
npm run dev
```

质量检查：

```bash
cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./...
cd ../frontend && npm run typecheck && npm run build
cd .. && docker compose config --quiet
```

也可以从项目根目录执行 `./scripts/validate.sh`。脚本会从空数据卷构建并启动全部服务，验证四级 RBAC、双人审批、状态红线、执行回执、请求 ID 和审计证据，并在结束时关闭容器。设置 `KEEP_RUNNING=1` 可在脚本验证后保留服务，供内置 Browser 验收。

## 目录结构

```text
.
├── backend/
│   ├── cmd/server/                 # 服务入口与优雅退出
│   └── internal/
│       ├── config/                 # 环境配置
│       ├── constants/              # 状态枚举与迁移图
│       ├── database/               # 连接、迁移与演示数据
│       ├── dto/                    # 输入契约
│       ├── handler/                # HTTP 接口
│       ├── middleware/             # JWT、追踪、限流
│       ├── model/                  # GORM 实体
│       ├── repository/             # 持久化边界
│       ├── router/                 # 路由装配
│       ├── service/                # 业务规则与审计
│       └── util/                   # 统一 HTTP 响应
├── frontend/src/
│   ├── api/                        # 按实体拆分的 API
│   ├── components/common/          # 状态徽标、闸门徽标、指令轨迹和确认框
│   ├── hooks/                      # RBAC 会话、分页和可见页轮询 hooks
│   ├── pages/                      # 登录与五个业务路由页面
│   ├── router/                     # 路由配置
│   ├── stores/                     # 按实体拆分的状态仓库
│   ├── types/                      # 共享类型与枚举
│   └── utils/                      # 格式化与状态工具
├── docker-compose.yml
└── runtime_smoke.json
```

## 共享枚举位置

| 枚举 | 值 | 前后端出现位置 |
|---|---|---|
| `GateState` | `open, closed, moving, locked` | `backend/internal/constants/status.go`、`frontend/src/types/status.ts`、`frontend/src/components/common/GateStateBadge.vue` |
| `DirectiveState` | `draft, pending, approved, executing, completed, aborted` | `backend/internal/constants/status.go`、`frontend/src/types/status.ts`、`frontend/src/components/common/DirectiveTimeline.vue` |

每个实体自己的完整迁移图同样位于 `backend/internal/constants/status.go`；页面使用的状态列表位于 `frontend/src/types/status.ts`。修改状态时必须同步两处并更新对应服务测试。

## 环境变量

| 变量 | 说明 |
|---|---|
| `COMPOSE_PROJECT_NAME` | 固定英文 Compose 项目名，支持中文父目录 |
| `DB_NAME/DB_USER/DB_PASSWORD` | 数据库名称与业务账号 |
| `JWT_SECRET` | JWT 签名密钥，生产环境必须使用至少 32 位且非占位值 |
| `LOGIN_REQUEST_LIMIT` | 单 IP 每分钟登录尝试上限，默认 8 |
| `BOOTSTRAP_*_PASSWORD` | 空库首次启动的管理员、操作员、复核员密码；生产环境至少 12 位、禁止默认值且三者必须不同 |
| `FRONTEND_PORT/BACKEND_PORT/DB_PORT` | 宿主机端口映射 |
| `REDIS_PORT` | Redis 宿主机端口 |

## API 使用示例

```bash
token=$(curl -sS -X POST http://127.0.0.1:19516/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"Admin123!"}' | jq -r '.data.token')

curl -sS http://127.0.0.1:19516/api/overview \
  -H "Authorization: Bearer $token"
```

## License

MIT
