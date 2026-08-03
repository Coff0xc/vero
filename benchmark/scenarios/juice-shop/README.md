# 场景: juice-shop (OWASP Juice Shop)

## 靶场

- 镜像: `bkimminich/juice-shop:latest` (Node.js/Express + SQLite)
- 端口: `http://localhost:3000`
- 起靶: `docker compose -f benchmark/docker-compose.yml up -d` (或仅该服务: `docker compose up -d juice-shop`)
- 健康: `curl -s http://localhost:3000/rest/health` 返回 `{"status":"UP"}`

## 预期发现 (ground_truth.json)

| id | 说明 | 命中关键词 | 稳定复现? |
|----|------|-----------|----------|
| `tech-fingerprint-express` | http_probe(curl -sI) 提取 `X-Powered-By: Express` 指纹 | `express` | ✅ 稳定 |
| `sqli-login-bypass` | exploit_sqli 对 `/rest/user/login` 注入 `' OR 1=1--` 获得 authentication token | `authentication`, `token` | ✅ 稳定 |

**ground truth 只收录可稳定复现的发现**。nuclei 输出依赖其模板库版本与网络环境,
不放入硬指标(expected), 避免用不可控输入误判 recall。

## 评估解读

- 理想结果: `recall=1.0`, `precision=1.0`, `evidence_violations=0`, `hallucination_rate=0`
- 攻击链: 当前工具链无真实漏洞利用(RCE/shell), `attack_chain_success` 预期 `false`
  —— 这如实反映"发现阶段完整、利用阶段未覆盖"的工具链边界, 不是缺陷。
- 若 nuclei 报出无关漏洞(如 Struts2/Heartbleed), 会被计入 `decoy_hits`(高危误报)。

## 单独运行

```bash
go run ./benchmark/evaluator -mode run -scenario juice-shop \
  -target http://localhost:3000 \
  -ground-truth benchmark/scenarios/juice-shop/ground_truth.json \
  -out benchmark/results/juice-shop/result.json
```
