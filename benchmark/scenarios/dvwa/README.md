# 场景: dvwa (Damn Vulnerable Web Application)

## 靶场

- 镜像: `vulnerables/web-dvwa:latest` (PHP + Apache + MySQL)
- 端口: `http://localhost:8080`
- 起靶: `docker compose -f benchmark/docker-compose.yml up -d` (或仅该服务: `docker compose up -d dvwa`)

## 初始化(必做, run_benchmark.sh 已自动处理)

DVWA 首次访问需两步初始化, 否则页面停留在 setup 向导:

1. **建库**: POST `setup.php` 提交 `create_db=Create / Reset Database`
2. **登录**: POST `login.php` 使用默认凭证 `admin / password`(需保持 cookie 会话)

手动初始化:
```bash
curl -c /tmp/dvwa.txt "http://localhost:8080/setup.php" -o /dev/null
curl -b /tmp/dvwa.txt -c /tmp/dvwa.txt \
  --data-urlencode "create_db=Create / Reset Database" \
  "http://localhost:8080/setup.php" -o /dev/null
curl -b /tmp/dvwa.txt -c /tmp/dvwa.txt \
  --data-urlencode "username=admin" --data-urlencode "password=password" \
  --data-urlencode "Login=Login" \
  "http://localhost:8080/login.php" -o /dev/null
```

## 预期发现 (ground_truth.json)

| id | 说明 | 命中关键词 | 稳定复现? |
|----|------|-----------|----------|
| `tech-fingerprint-apache` | http_probe(curl -sI) 提取 `Server: Apache` 指纹 | `apache` | ✅ 稳定 |

DVWA 的漏洞页面(SQLi/XSS 等)需要登录后会话与具体功能参数, 当前无 LLM 决策器
在不提供凭证的情况下不深入登录后功能, 故 ground truth 只收录登录页可达指纹。
这是工具链覆盖边界(侦察到服务即可复现), 非评估器缺陷。

## 评估解读

- 理想结果: `recall=1.0`, `precision=1.0`, `evidence_violations=0`
- 若 nuclei 报出 Log4j/Jenkins 等无关漏洞 -> `decoy_hits` 计入(真实误报信号)。
- 攻击链 `attack_chain_success` 预期 `false`(无登录后利用链)。

## 单独运行

```bash
go run ./benchmark/evaluator -mode run -scenario dvwa \
  -target http://localhost:8080 \
  -ground-truth benchmark/scenarios/dvwa/ground_truth.json \
  -out benchmark/results/dvwa/result.json
```
