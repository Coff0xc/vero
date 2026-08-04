# Vero 实战验证指南

本指南用于验证第 1+2 阶段的 10 个项目能力是否真正可用。

---

## 🎯 验证目标

1. **LLM 是否按规格填参** - 14 个新工具的 ArgSpec 是否生效
2. **工具是否正常执行** - 依赖检测、实际调用、错误处理
3. **Parser 是否正确提取** - 观察节点、攻击图构建
4. **反射学习是否生效** - 失败自动分类、retry、教训持久化

---

## 🛠️ 靶场环境准备

### 1. 代码审计靶场 (验证 CodeAuditPack)

```bash
# 创建测试项目
mkdir -p /tmp/vero-test-code
cd /tmp/vero-test-code

# Python 靶场 (测试 bandit_scan)
cat > vulnerable.py <<'EOF'
import os

# B105: 硬编码密码
password = "admin123"

# B602: shell=True 命令注入
user_input = input("Enter command: ")
os.system(f"ls {user_input}")

# B301: pickle 不安全反序列化
import pickle
data = pickle.loads(user_data)
EOF

# JS 靶场 (测试 semgrep_scan)
cat > app.js <<'EOF'
const express = require('express');
const app = express();

// SQL 注入
app.get('/user', (req, res) => {
  const query = "SELECT * FROM users WHERE id = " + req.query.id;
  db.query(query);
});

// XSS
app.get('/search', (req, res) => {
  res.send("<h1>Results: " + req.query.q + "</h1>");
});
EOF

# package.json (测试 dependency_check)
cat > package.json <<'EOF'
{
  "name": "vulnerable-app",
  "dependencies": {
    "express": "4.16.0",
    "lodash": "4.17.15",
    "axios": "0.18.0"
  }
}
EOF

echo "✓ 代码审计靶场已创建: /tmp/vero-test-code"
```

### 2. 云渗透靶场 (验证 CloudPackEnhanced)

```bash
# 配置 AWS 本地模拟 (LocalStack)
docker run -d --name vero-localstack \
  -p 4566:4566 \
  -e SERVICES=s3,iam,ec2 \
  localstack/localstack

# 配置 AWS CLI 指向 LocalStack
export AWS_ENDPOINT_URL=http://localhost:4566
aws configure set aws_access_key_id test
aws configure set aws_secret_access_key test
aws configure set region us-east-1

# 创建测试 S3 桶 (公开配置)
aws s3 mb s3://public-data --endpoint-url=http://localhost:4566
aws s3api put-bucket-acl --bucket public-data --acl public-read --endpoint-url=http://localhost:4566

# 创建 IAM 用户 (危险权限)
aws iam create-user --user-name test-user --endpoint-url=http://localhost:4566
aws iam attach-user-policy --user-name test-user \
  --policy-arn arn:aws:iam::aws:policy/AdministratorAccess \
  --endpoint-url=http://localhost:4566

echo "✓ 云渗透靶场已创建 (LocalStack)"
```

### 3. K8s 靶场 (验证 K8sPackEnhanced)

```bash
# 使用 kind 创建本地 K8s 集群
kind create cluster --name vero-test

# 部署特权 Pod
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: privileged-pod
spec:
  containers:
  - name: nginx
    image: nginx:latest
    securityContext:
      privileged: true
  volumes:
  - name: docker-sock
    hostPath:
      path: /var/run/docker.sock
EOF

# 部署 cluster-admin ServiceAccount
kubectl create sa admin-sa
kubectl create clusterrolebinding admin-binding \
  --clusterrole=cluster-admin \
  --serviceaccount=default:admin-sa

echo "✓ K8s 靶场已创建 (kind)"
```

---

## 🧪 验证脚本

### 脚本 1: 代码审计验证

```bash
#!/bin/bash
# verify_code_audit.sh

echo "=== 代码审计能力验证 ==="
cd /tmp/vero-test-code

# 启动 Vero
curl -X POST http://localhost:8080/start \
  -H "Content-Type: application/json" \
  -d '{"target": "/tmp/vero-test-code"}'

# 监听事件流
curl -N http://localhost:8080/events &
EVENT_PID=$!

# 等待战役完成 (60 秒)
sleep 60

# 检查结果
echo ""
echo "=== 验证结果 ==="
echo "1. 检查是否调用 semgrep_scan:"
curl -s http://localhost:8080/api/campaigns | jq '.[0].events[] | select(.data.tool == "semgrep_scan")'

echo ""
echo "2. 检查是否调用 bandit_scan:"
curl -s http://localhost:8080/api/campaigns | jq '.[0].events[] | select(.data.tool == "bandit_scan")'

echo ""
echo "3. 检查是否发现 finding (硬编码密码):"
curl -s http://localhost:8080/api/campaigns | jq '.[0].events[] | select(.data.kind == "finding" and (.data.label | contains("password")))'

kill $EVENT_PID
```

### 脚本 2: 云渗透验证

```bash
#!/bin/bash
# verify_cloud.sh

echo "=== 云渗透能力验证 ==="

# 启动 Vero (目标: AWS 环境)
curl -X POST http://localhost:8080/start \
  -H "Content-Type: application/json" \
  -d '{"target": "aws://test"}'

sleep 60

# 检查结果
echo "=== 验证结果 ==="
echo "1. 检查是否调用 aws_s3_enum:"
curl -s http://localhost:8080/api/campaigns | jq '.[0].events[] | select(.data.tool == "aws_s3_enum")'

echo ""
echo "2. 检查是否发现公开桶:"
curl -s http://localhost:8080/api/campaigns | jq '.[0].events[] | select(.data.label | contains("公开桶"))'

echo ""
echo "3. 检查是否调用 aws_iam_privesc:"
curl -s http://localhost:8080/api/campaigns | jq '.[0].events[] | select(.data.tool == "aws_iam_privesc")'
```

### 脚本 3: K8s 验证

```bash
#!/bin/bash
# verify_k8s.sh

echo "=== K8s/容器渗透验证 ==="

# 启动 Vero (目标: K8s 集群)
curl -X POST http://localhost:8080/start \
  -H "Content-Type: application/json" \
  -d '{"target": "k8s://default"}'

sleep 60

# 检查结果
echo "=== 验证结果 ==="
echo "1. 检查是否调用 k8s_enum_pods:"
curl -s http://localhost:8080/api/campaigns | jq '.[0].events[] | select(.data.tool == "k8s_enum_pods")'

echo ""
echo "2. 检查是否调用 k8s_rbac_check:"
curl -s http://localhost:8080/api/campaigns | jq '.[0].events[] | select(.data.tool == "k8s_rbac_check")'

echo ""
echo "3. 检查是否发现 cluster-admin:"
curl -s http://localhost:8080/api/campaigns | jq '.[0].events[] | select(.data.label | contains("cluster-admin"))'
```

### 脚本 4: 反射学习验证

```bash
#!/bin/bash
# verify_reflexion.sh

echo "=== 反射学习验证 ==="

# 启动 Vero (故意触发失败)
curl -X POST http://localhost:8080/start \
  -H "Content-Type: application/json" \
  -d '{"target": "invalid-target"}'

sleep 30

# 检查 lessons 表
sqlite3 vero.db "SELECT * FROM lessons ORDER BY created_at DESC LIMIT 5"

echo ""
echo "=== 验证结果 ==="
echo "1. 检查失败模式分类:"
sqlite3 vero.db "SELECT mode, COUNT(*) FROM lessons GROUP BY mode"

echo ""
echo "2. 检查解决方案建议:"
sqlite3 vero.db "SELECT tool, reason, solution FROM lessons LIMIT 3"

echo ""
echo "3. 检查 retry 次数 (应有自动 retry 记录):"
curl -s http://localhost:8080/api/campaigns | jq '.[0].events[] | select(.data.tool == "port_scan") | .data.args.retry'
```

---

## 📊 验证检查清单

### 代码审计 (CodeAuditPack)
- [ ] LLM 自动调用 `semgrep_scan` (检测到 git-repo 指纹)
- [ ] LLM 自动调用 `bandit_scan` (检测到 .py 文件)
- [ ] Parser 提取 CVE/CWE (dependency-check)
- [ ] 严重度标准化 (ERROR → critical)

### 云渗透 (CloudPackEnhanced)
- [ ] LLM 自动调用 `aws_s3_enum` (检测到 AWS 环境)
- [ ] Parser 提取公开桶 (ACL 含 AllUsers)
- [ ] LLM 自动调用 `aws_iam_privesc` (发现 IAM 用户)
- [ ] MITRE 映射 (T1078/T1552.005)

### K8s/容器 (K8sPackEnhanced)
- [ ] LLM 自动调用 `k8s_enum_pods` (检测到 K8s 环境)
- [ ] Parser 提取 :latest 警告
- [ ] LLM 自动调用 `k8s_rbac_check`
- [ ] Parser 提取 cluster-admin 危险权限
- [ ] LLM 自动调用 `k8s_node_exploit` (发现特权 Pod)

### 反射学习 (Reflexion)
- [ ] 失败自动分类 (network/permission/...)
- [ ] lessons 表持久化 (SQLite)
- [ ] 自动 retry (超时 → 增加 timeout 参数)
- [ ] Few-shot 注入 (LLM prompt 含历史教训)

---

## 🐛 常见问题

### Q1: LLM 不调用新工具
**原因**: 指纹函数未激活, 或 LLM 不知道工具存在  
**解决**: 检查 `RegisterDefaults` 是否注册, 检查 LLM prompt 是否含工具列表

### Q2: 工具执行失败
**原因**: 依赖未安装  
**解决**: 运行 `curl http://localhost:8080/api/dependencies` 检查缺失工具

### Q3: Parser 不提取观察
**原因**: 输出格式不匹配, JSON 解析失败  
**解决**: 检查工具输出 (stdout), 调试 Parser 正则

### Q4: 反射学习不生效
**原因**: lessons 表未创建, 或 OnFailure 未调用  
**解决**: 检查 `vero.db` 是否有 lessons 表, 检查 loop.go 是否调用 Reflector

---

## 🚀 快速验证命令

```bash
# 一键启动所有验证
cd /tmp/vero-test
bash verify_code_audit.sh > code_audit_result.txt 2>&1 &
bash verify_cloud.sh > cloud_result.txt 2>&1 &
bash verify_k8s.sh > k8s_result.txt 2>&1 &
bash verify_reflexion.sh > reflexion_result.txt 2>&1

# 等待全部完成
wait

# 汇总结果
echo "=== 验证结果汇总 ==="
grep -E "✓|✗|PASS|FAIL" *_result.txt
```

---

**预计耗时**: 2-3 小时 (靶场搭建 1h + 验证执行 1h + 结果分析 0.5-1h)
