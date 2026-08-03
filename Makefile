# Vero —— 自主红队渗透智能体
# Windows 产物为 vero.exe; Linux/macOS 可自行改 BIN=vero。
BIN ?= vero.exe

.PHONY: build web-build test selfcheck dev-server dev-web fmt vet clean bench install-tools

## build: 构建单一二进制(前端 embed 进 Go)
build: web-build
	go build -o $(BIN) ./cmd/vero

## web-build: 仅构建前端到 internal/webui/dist
web-build:
	cd web && npm install && npm run build

## test: 全量测试(内核 + 规划 + 场景 + 审计 + eval + store + 端到端集成)
test:
	go test ./internal/...

## bench: 起 benchmark 靶场(需 Docker)并给出评估命令
bench:
	@docker info >/dev/null 2>&1 || { echo "✗ 需要 Docker 才能起 benchmark 靶场 (docker 未运行或未安装)"; exit 1; }
	cd benchmark/scenarios/CVE-2021-44228-log4shell && docker compose up -d
	@echo "✓ 靶场已启动。运行战役后评估:"
	@echo "  python3 benchmark/evaluator/evaluate.py --ground-truth benchmark/scenarios/CVE-2021-44228-log4shell/ground_truth.json --result <agent_result.json> --output benchmark/results/eval.json"

## install-tools: 一键安装缺失工具(经本地服务 API; 先启动 ./vero)
install-tools:
	@echo "启动 ./vero 后, 用 API 批量安装缺失工具:"
	@echo "  curl -X POST http://localhost:8000/api/tools/install-all -d '{}'"
	@echo "单工具安装: curl -X POST http://localhost:8000/api/tools/install -d '{\"name\":\"nuclei\",\"type\":\"binary\"}'"
	@echo "或直接在 Web 作战室「工具管理」页点「全部自动安装」。"
