#!/usr/bin/env bash
# =============================================================================
# Vero Benchmark 一键跑评估
#
# 流程: 起靶场(docker compose) -> 等待健康 -> DVWA 初始化 -> 逐场景跑真实战役
#       -> 评估器算指标 -> 汇总 results/benchmark-result.json
#
# 前置要求:
#   - docker (或 docker compose v2)
#   - go 1.26+ (评估器为 Go 程序, 与产品内核同源)
#   - curl
#   - (可选) nuclei / curl 真实工具已在 PATH 或经 vero -tooltest 安装
#   - (可选) DEEPSEEK_API_KEY / ANTHROPIC_API_KEY + VERO_ENGINE=llm 用真实 LLM 决策;
#     缺省 VERO_ENGINE=script 用真实工具固定脚本, 无需 key
#
# 用法:
#   bash benchmark/run_benchmark.sh                 # 默认场景: juice-shop, dvwa
#   bash benchmark/run_benchmark.sh log4shell       # 追加 log4shell 场景(见场景 README 的覆盖缺口)
#   VERO_ENGINE=llm bash benchmark/run_benchmark.sh # 用真实 LLM 自主决策
#
# Windows 用户: 本脚本在 Git Bash / WSL 下直接运行。
# PowerShell 等价命令(不带健康等待/初始化, 需手动起靶与初始化 DVWA):
#   docker compose -f benchmark/docker-compose.yml up -d
#   go run ./benchmark/evaluator -mode run -scenario juice-shop -target http://localhost:3000 ^
#       -ground-truth benchmark/scenarios/juice-shop/ground_truth.json -out benchmark/results/juice-shop/result.json
# =============================================================================
set -euo pipefail

# 本机靶场请求一律不走环境代理(修: 用户设 http_proxy 且无 NO_PROXY 时,
# 战役内 curl 打 localhost 会打到代理 -> 502 被误判为“利用成功”/靶场未就绪)。
# curl 尊重 NO_PROXY; 脚本内导出生效, 不污染用户 shell。
export NO_PROXY="localhost,127.0.0.1,::1"
export no_proxy="$NO_PROXY"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BENCH="$ROOT/benchmark"
EVAL="go run ./benchmark/evaluator"
SCENARIOS_DEFAULT="juice-shop dvwa"

log()  { printf '\033[1;34m[bench]\033[0m %s\n' "$*"; }
fail() { printf '\033[1;31m[bench][ERROR]\033[0m %s\n' "$*" >&2; exit 1; }

# ---------- 前置检查 ----------
command -v docker >/dev/null 2>&1 || fail "未找到 docker, 请先安装 Docker Desktop / docker engine"
command -v go     >/dev/null 2>&1 || fail "未找到 go, 请安装 Go 1.26+"
command -v curl   >/dev/null 2>&1 || fail "未找到 curl"
# Windows Store 的 python3 是 stub(退出码 49): 探测 python3/python 且验证能 import json
PY=""
if command -v python3 >/dev/null 2>&1 && python3 -c "import json" >/dev/null 2>&1; then
  PY=python3
elif command -v python >/dev/null 2>&1 && python -c "import json" >/dev/null 2>&1; then
  PY=python
fi
if [ -z "$PY" ]; then log "未找到可用的 python, 将跳过汇总 JSON(每场景 result.json 仍会生成)"; fi

# ---------- 参数 ----------
SCENARIOS="${*:-$SCENARIOS_DEFAULT}"

# ---------- 起靶场 ----------
log "启动靶场: juice-shop(:3000) + dvwa(:8080) ..."
docker compose -f "$BENCH/docker-compose.yml" up -d

# ---------- 等待健康 ----------
wait_http() {
  local url="$1" name="$2" tries="${3:-60}"
  for _ in $(seq 1 "$tries"); do
    # --noproxy "*": 本机靶场请求不走环境代理(修: 用户设 http_proxy 且无 NO_PROXY 时误判靶场未就绪)
    if curl -sf --noproxy "*" -o /dev/null --max-time 3 "$url"; then
      log "靶场 $name 就绪: $url"
      return 0
    fi
    sleep 2
  done
  fail "靶场 $name 未就绪(超时): $url"
}

wait_http "http://localhost:3000" "juice-shop"
wait_http "http://localhost:8080/login.php" "dvwa"

# ---------- DVWA 初始化(建库 + 登录, 幂等) ----------
init_dvwa() {
  local jar="$BENCH/.dvwa-cookies.txt"
  rm -f "$jar"
  # 1) 提交建库表单
  curl -sf -b "$jar" -c "$jar" --max-time 10 \
    --data-urlencode "create_db=Create / Reset Database" \
    "http://localhost:8080/setup.php" >/dev/null 2>&1 || true
  # 2) 登录 admin/password(web-dvwa 默认凭证)
  curl -sf -b "$jar" -c "$jar" --max-time 10 \
    --data-urlencode "username=admin" --data-urlencode "password=password" \
    --data-urlencode "Login=Login" \
    "http://localhost:8080/login.php" >/dev/null 2>&1 || true
  # 3) 验证登录成功(跳转 index.php 且带 PHPSESSID)
  if curl -sf -b "$jar" -c "$jar" --max-time 10 -o /dev/null \
    -w "%{http_code}" "http://localhost:8080/index.php" | grep -q "200"; then
    log "DVWA 初始化完成(建库+登录)"
  else
    log "警告: DVWA 初始化未完全成功, 场景可能只覆盖登录页指纹(不阻塞)"
  fi
}
init_dvwa

# ---------- 逐场景跑评估 ----------
mkdir -p "$BENCH/results"
SUMMARY="$BENCH/results/benchmark-result.json"
: > "$SUMMARY.parts"   # 每场景 result.json 路径清单(供汇总)

for sc in $SCENARIOS; do
  GT="$BENCH/scenarios/$sc/ground_truth.json"
  [ -f "$GT" ] || { log "跳过未知场景 $sc (无 $GT)"; continue; }

  # target 从 ground truth 读取(JSON 字段 target)
  TARGET="$(grep -o '"target"[[:space:]]*:[[:space:]]*"[^"]*"' "$GT" | head -1 | sed -E 's/.*"target"[[:space:]]*:[[:space:]]*"([^"]*)"/\1/' || true)" # || true: 防 pipefail 下 grep 无匹配中止脚本
  [ -n "$TARGET" ] || { log "跳过 $sc (ground truth 缺 target)"; continue; }

  OUT="$BENCH/results/$sc/result.json"
  mkdir -p "$BENCH/results/$sc"
  log "场景 $sc: 跑真实战役 -> $TARGET"
  # 场景失败不阻断整体: benchmark 是测量不是门禁(修: 原无 || 兜底, set -e 下单场景失败直接退出)
  if (cd "$ROOT" && $EVAL -mode run -scenario "$sc" -target "$TARGET" \
    -ground-truth "$GT" -out "$OUT"); then
    echo "$OUT" >> "$SUMMARY.parts"
  else
    log "场景 $sc 战役失败, 跳过(见上方错误)"
  fi
done

# ---------- 汇总 ----------
if [ -n "$PY" ]; then
  log "生成汇总: $SUMMARY"
  "$PY" - "$SUMMARY" "$SUMMARY.parts" <<'PY'
import json, sys, os
summary_path, parts_path = sys.argv[1], sys.argv[2]
entries = []
for p in open(parts_path, encoding="utf-8"):
    p = p.strip()
    if not p or not os.path.exists(p):
        continue
    with open(p, encoding="utf-8") as f:
        r = json.load(f)
    entries.append({
        "scenario": r.get("scenario"),
        "target": r.get("target"),
        "engine": r.get("engine"),
        "verdict": r.get("verdict"),
        "metrics": r.get("metrics"),
    })
summary = {
    "generated_at": __import__("datetime").datetime.now().utcnow().isoformat() + "Z",
    "note": "Vero Benchmark 实测汇总; 指标定义见 benchmark/README.md",
    "scenarios": entries,
}
with open(summary_path, "w", encoding="utf-8") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
print("已写入", summary_path, "共", len(entries), "个场景")
PY
fi

log "完成。逐场景结果: $(cat "$SUMMARY.parts" | tr '\n' ' ')"
