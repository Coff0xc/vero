# REDCELL —— 自主红队渗透智能体
# Windows 产物为 redcell.exe; Linux/macOS 可自行改 BIN=redcell。
BIN ?= redcell.exe

.PHONY: build web-build test selfcheck dev-server dev-web fmt vet clean

## build: 构建单一二进制(前端 embed 进 Go)
build: web-build
	go build -o $(BIN) ./cmd/redcell

## web-build: 仅构建前端到 internal/webui/dist
web-build:
	cd web && npm install && npm run build

## test: 全量测试(内核 + 规划 + 场景 + 审计 + eval + store + 端到端集成)
test:
	go test ./internal/...

## selfcheck: 离线跑通内核闭环(无需前端/API key)
selfcheck:
	go run ./cmd/redcell -selfcheck

## dev-server: 开发模式后端(:8000)
dev-server:
	go run ./cmd/redcell

## dev-web: 开发模式前端(:5173, 反代 API 到 :8000)
dev-web:
	cd web && npm run dev

fmt:
	go fmt ./cmd/... ./internal/...

vet:
	go vet ./cmd/... ./internal/...

## clean: 清理二进制与运行时数据
clean:
	rm -f $(BIN) *.db *.db-journal audit.jsonl rollback.jsonl *.log
