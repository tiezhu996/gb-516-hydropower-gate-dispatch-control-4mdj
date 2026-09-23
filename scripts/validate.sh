#!/usr/bin/env sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"
set -a
if [ -f .env ]; then . ./.env; else . ./.env.example; fi
set +a

command -v jq >/dev/null 2>&1 || { echo "jq is required for API validation" >&2; exit 1; }
api="http://127.0.0.1:${BACKEND_PORT:-19516}/api"
web="http://127.0.0.1:${FRONTEND_PORT:-18516}"
tmp_dir=$(mktemp -d)

compose_cleanup() {
  docker compose down -v --remove-orphans
  rm -rf "$tmp_dir"
}
if [ "${KEEP_RUNNING:-0}" = "1" ]; then
  trap compose_cleanup INT TERM
else
  trap compose_cleanup EXIT INT TERM
fi

expect_status() {
  expected=$1
  shift
  actual=$(curl -sS -o "$tmp_dir/response.json" -w '%{http_code}' "$@")
  if [ "$actual" != "$expected" ]; then
    echo "expected HTTP $expected, got $actual" >&2
    cat "$tmp_dir/response.json" >&2
    exit 1
  fi
}

login_token() {
  username=$1
  curl -fsS -X POST "$api/auth/login" -H 'Content-Type: application/json' \
    -d "{\"username\":\"$username\",\"password\":\"Admin123!\"}" | jq -er '.data.token'
}

echo "[1/5] Static quality checks"
(cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./...)
(cd frontend && npm ci --no-audit --no-fund && npm run typecheck && npm run build)
docker compose config --quiet

echo "[2/5] Empty-volume Compose startup"
docker compose down -v --remove-orphans
docker compose up -d --build
i=0
until curl -fsS "http://127.0.0.1:${BACKEND_PORT:-19516}/healthz" | jq -e '.data.status == "ok" and .data.database == "ready" and .data.redis == "ready"' >/dev/null; do
  i=$((i + 1))
  [ "$i" -lt 60 ] || { docker compose logs; exit 1; }
  sleep 2
done
i=0
until curl -fsS "$web/" >/dev/null 2>&1; do
	i=$((i + 1))
	[ "$i" -lt 30 ] || { docker compose logs frontend; exit 1; }
	sleep 1
done

echo "[3/5] Authentication and route-level RBAC"
admin_token=$(login_token admin)
operator_token=$(login_token operator)
reviewer_token=$(login_token reviewer)
viewer_token=$(login_token viewer)
for resource in reservoirs gates directives confirmations; do
  curl -fsS "$api/$resource?page=1&pageSize=20" -H "Authorization: Bearer $viewer_token" | jq -e '.data | type == "array"' >/dev/null
done
curl -fsS "$api/session" -H "Authorization: Bearer $viewer_token" | jq -e '.data.role == "viewer" and (.data.requestId | length > 0)' >/dev/null
expect_status 403 "$api/audits?page=1&pageSize=10" -H "Authorization: Bearer $viewer_token"
expect_status 200 "$api/audits?page=1&pageSize=10" -H "Authorization: Bearer $reviewer_token"
now=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
viewer_payload=$(jq -n --arg at "$now" '{code:"VIEWER-DENIED",name:"Viewer denied",facility:"Main dam",owner:"viewer",category:"rbac",riskLevel:"low",metricValue:1,metricUnit:"m",effectiveAt:$at,evidence:"must not persist",relatedCode:""}')
expect_status 403 -X POST "$api/reservoirs" -H "Authorization: Bearer $viewer_token" -H 'Content-Type: application/json' -d "$viewer_payload"

echo "[4/5] Two-person directive and execution flow"
suffix=$(date +%s)
code="OD-VAL-$suffix"
directive_payload=$(jq -n --arg code "$code" --arg at "$now" '{code:$code,name:"右岸泄洪闸调度许可",description:"空卷运行验证",facility:"水电站闸门调度许可区域2",owner:"运行一组",category:"泄洪调度",riskLevel:"high",metricValue:35,metricUnit:"%",effectiveAt:$at,evidence:"水位窗口、设备闭锁和通信链路已核对",relatedCode:"GU-002",gateState:"open"}')
created=$(curl -fsS -X POST "$api/directives" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H "X-Request-ID: val-create-$suffix" -d "$directive_payload")
id=$(printf '%s' "$created" | jq -er '.data.id')
version=$(printf '%s' "$created" | jq -er '.data.version')
printf '%s' "$created" | jq -e '.data.status == "draft" and .data.gateState == "open"' >/dev/null

submit=$(jq -n --argjson version "$version" '{status:"pending",expectedVersion:$version,reason:"操作员提交水位窗口和目标开度复核"}')
submitted=$(curl -fsS -X POST "$api/directives/$id/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H "X-Request-ID: val-submit-$suffix" -d "$submit")
version=$(printf '%s' "$submitted" | jq -er '.data.version')
printf '%s' "$submitted" | jq -e --arg request "val-submit-$suffix" '.data.status == "pending" and .data.submittedBy == "operator" and (.data.approvals | length) == 1 and .data.approvals[0].requestId == $request' >/dev/null

self_approval=$(jq -n --argjson version "$version" '{status:"approved",expectedVersion:$version,reason:"操作员不得越权自行批准"}')
expect_status 403 -X POST "$api/directives/$id/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$self_approval"
approve=$(jq -n --argjson version "$version" '{status:"approved",expectedVersion:$version,reason:"复核员确认水位窗口、闸门目标和现场证据一致"}')
approved=$(curl -fsS -X POST "$api/directives/$id/transition" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -H "X-Request-ID: val-approve-$suffix" -d "$approve")
version=$(printf '%s' "$approved" | jq -er '.data.version')
printf '%s' "$approved" | jq -e --arg request "val-approve-$suffix" '.data.status == "approved" and .data.submittedBy == "operator" and .data.approvedBy == "reviewer" and .data.submittedBy != .data.approvedBy and (.data.approvals | length) == 2 and .data.approvals[1].requestId == $request' >/dev/null

expect_status 409 -X PUT "$api/directives/$id" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$(printf '%s' "$directive_payload" | jq --argjson version "$version" '. + {expectedVersion:$version}')"
execute=$(jq -n --argjson version "$version" '{status:"executing",expectedVersion:$version,reason:"双人许可完成，现场开始执行"}')
executing=$(curl -fsS -X POST "$api/directives/$id/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$execute")
version=$(printf '%s' "$executing" | jq -er '.data.version')
complete=$(jq -n --argjson version "$version" '{status:"completed",expectedVersion:$version,reason:"闸门动作与目标开度核对完成"}')
expect_status 422 -X POST "$api/directives/$id/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$complete"

confirmation_code="EC-VAL-$suffix"
confirmation_payload=$(jq -n --arg code "$confirmation_code" --arg related "$code" --arg at "$now" '{code:$code,name:"泄洪闸执行回执",description:"现场执行验证",facility:"水电站闸门调度许可区域2",owner:"运行一组",category:"执行回执",riskLevel:"high",metricValue:35,metricUnit:"%",effectiveAt:$at,evidence:"闸位反馈、视频和对讲记录已核对",relatedCode:$related}')
confirmation=$(curl -fsS -X POST "$api/confirmations" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$confirmation_payload")
confirmation_id=$(printf '%s' "$confirmation" | jq -er '.data.id')
confirmation_version=$(printf '%s' "$confirmation" | jq -er '.data.version')
confirm_body=$(jq -n --argjson version "$confirmation_version" '{status:"confirmed",expectedVersion:$version,reason:"现场闸位反馈与批准指令一致"}')
curl -fsS -X POST "$api/confirmations/$confirmation_id/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -H "X-Request-ID: val-confirm-$suffix" -d "$confirm_body" | jq -e '.data.status == "confirmed" and .data.confirmedBy == "operator" and .data.confirmedAt' >/dev/null

curl -fsS "$api/gates/2" -H "Authorization: Bearer $operator_token" | jq -e '.data.status == "open"' >/dev/null
gate_version=$(curl -fsS "$api/gates/2" -H "Authorization: Bearer $operator_token" | jq -er '.data.version')
direct_close=$(jq -n --argjson version "$gate_version" '{status:"closed",expectedVersion:$version,reason:"不得绕过 moving 中间态"}')
expect_status 422 -X POST "$api/gates/2/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$direct_close"

echo "[5/5] Request tracing, audit evidence and service status"
curl -fsS "$api/directives/$id" -H "Authorization: Bearer $reviewer_token" | jq -e '.data.status == "completed" and (.data.approvals | length) == 2' >/dev/null
curl -fsS "$api/audits?page=1&pageSize=100&search=OperationDirective" -H "Authorization: Bearer $reviewer_token" | jq -e '.meta.total >= 5 and ([.data[].requestId] | index("val-submit-'"$suffix"'")) != null and ([.data[].requestId] | index("val-approve-'"$suffix"'")) != null' >/dev/null
curl -fsS -D "$tmp_dir/headers" "$api/runtime" -o "$tmp_dir/runtime.json" -H "Authorization: Bearer $admin_token"
jq -e '.data.appName == "hydropower-gate-dispatch-control" and .data.databaseDriver == "postgres" and .data.redisEnabled == true' "$tmp_dir/runtime.json" >/dev/null
grep -iq '^x-request-id:' "$tmp_dir/headers"
docker compose exec -T db psql -U "${DB_USER}" -d "${DB_NAME}" -c "UPDATE users SET active = false WHERE username = 'viewer'" >/dev/null
expect_status 401 "$api/session" -H "Authorization: Bearer $viewer_token"
docker compose exec -T db psql -U "${DB_USER}" -d "${DB_NAME}" -c "UPDATE users SET active = true WHERE username = 'viewer'" >/dev/null
docker compose ps

if [ "${KEEP_RUNNING:-0}" = "1" ]; then
  rm -rf "$tmp_dir"
  trap compose_cleanup INT TERM
  echo "KEEP_RUNNING=1: containers left running for built-in Browser validation"
fi
