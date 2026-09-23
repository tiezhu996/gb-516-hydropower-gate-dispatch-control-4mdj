请生成 `hydropower-gate-dispatch-control`「水电站闸门调度许可」Go 全栈项目，面向水电站值班团队管理闸门、库区水位窗口、操作指令和执行确认。项目重点是安全控制和状态迁移，不做账务、订单、库存或预约。

## 项目主要需求

复杂度下限：核心实体不少于 3 个、核心页面不少于 4 个、横切关注点不少于 2 个、共享前端组件不少于 3 个、自定义 hooks/utils 不少于 2 个、后端中间件不少于 2 个。

### 核心实体

`Reservoir`（库区与水位阈值）、`GateUnit`（闸门和当前状态）、`OperationDirective`（操作指令）、`ExecutionConfirmation`（执行回执）必须贯穿数据库、Go model/service/handler 和前端 API/store/page。

### 核心页面

`/reservoirs` 库区；`/gates` 闸门；`/directives` 指令编排；`/confirmations` 执行确认；`/audit` 审计。`GateStateBadge` 在闸门和指令页共用，`DirectiveTimeline` 在指令和确认页共用。

### 横切关注点

RBAC、双人确认、请求 ID 和审计日志必须跨数据库、Go middleware、路由守卫和前端按钮；全局错误处理和限流独立实现。

### 共享枚举/组件

同步 `GateState`（open/closed/moving/locked）与 `DirectiveState`（draft/pending/approved/executing/completed/aborted）。共享 `StatusBadge`、`DirectiveTimeline`、`ConfirmDialog`，hooks 为 `useAuth`、`usePolling`。

### 技术与规模要求

前端 Vue 3 + TypeScript + Vite + Element Plus；后端 Go 1.22 + Gin + GORM；PostgreSQL + Redis。目标 3000–4200 行、30–42 个 `.go` 文件。

### 文件结构强制清单

前端 `api/stores/types/components/common/hooks/pages/router/utils`；后端 `model/dto/repository/service/handler/router/middleware/constants/util`，README 列出枚举出现位置。

### 结构红线

严禁合并职责到单一文件；指令和确认必须沿数据库、后端、前端分层实现。

### 部署与交付

根目录必须提供 `docker-compose.yml`（顶层 `name: hydropower-gate-dispatch-control`，且不写 `version:`）、`.env` 和 `.env.example`（均含 `COMPOSE_PROJECT_NAME=hydropower-gate-dispatch-control`）、`README.md`、`frontend/Dockerfile`、`backend/Dockerfile` 和 `frontend/nginx.conf`。前端端口 `18516`、后端端口 `19516`；Nginx `/api` 代理、数据库 healthcheck、命名卷和 `condition: service_healthy` 齐全，提供真实 `/healthz`、Git 初始化。
