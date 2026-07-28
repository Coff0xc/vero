# Vero 部署指南

**版本**: 1.0.0  
**更新日期**: 2026-07-28

---

## 📋 目录

1. [Docker 部署](#docker-部署)
2. [Docker Compose 部署](#docker-compose-部署)
3. [Kubernetes 部署](#kubernetes-部署)
4. [生产环境配置](#生产环境配置)
5. [安全加固](#安全加固)
6. [监控与日志](#监控与日志)
7. [故障排除](#故障排除)

---

## 🐳 Docker 部署

### 1. 构建镜像

```bash
# 克隆仓库
git clone https://github.com/your-org/redcell
cd redcell

# 构建镜像
docker build -t redcell:latest .

# 验证构建
docker images | grep redcell
```

### 2. 运行容器

```bash
# 基础运行
docker run -d \
  --name redcell \
  -p 8000:8000 \
  -v $(pwd)/data:/app/data \
  redcell:latest

# 带 API Key
docker run -d \
  --name redcell \
  -p 8000:8000 \
  -e ANTHROPIC_API_KEY="sk-ant-..." \
  -v $(pwd)/data:/app/data \
  redcell:latest

# 访问
open http://localhost:8000
```

### 3. 独立工具模式

```bash
# 容器逃逸检测 (在特权容器内)
docker run --rm --privileged \
  -v /var/run/docker.sock:/var/run/docker.sock \
  redcell:latest \
  -container-escape check

# S3 检测
docker run --rm \
  redcell:latest \
  -cloud-s3 my-bucket

# 自检模式
docker run --rm redcell:latest -selfcheck
```

---

## 🎼 Docker Compose 部署

### 1. 环境准备

创建 `.env` 文件：

```bash
# API Keys (可选)
ANTHROPIC_API_KEY=sk-ant-...
DEEPSEEK_API_KEY=sk-...

# Metasploit RPC 密码
MSF_PASSWORD=your-strong-password-here
```

创建目录结构：

```bash
mkdir -p data wordlists audit
```

### 2. 启动服务

```bash
# 启动所有服务 (Vero + Metasploit + PostgreSQL)
docker-compose up -d

# 只启动 Vero
docker-compose up -d redcell

# 查看日志
docker-compose logs -f redcell

# 查看状态
docker-compose ps
```

### 3. 验证部署

```bash
# 检查 Vero
curl http://localhost:8000/

# 检查 Metasploit RPC
curl -X POST http://localhost:55553/api/1.0/auth.login \
  -H "Content-Type: application/json" \
  -d '{"username":"msf","password":"your-strong-password-here"}'
```

### 4. 停止服务

```bash
# 停止所有服务
docker-compose down

# 停止并删除数据卷
docker-compose down -v
```

---

## ☸️ Kubernetes 部署

### 1. 前置要求

- Kubernetes 1.25+
- kubectl 配置完成
- Ingress Controller (可选)
- cert-manager (可选，用于 TLS)

### 2. 准备镜像

```bash
# 构建镜像
docker build -t your-registry.io/redcell:1.0.0 .

# 推送到镜像仓库
docker push your-registry.io/redcell:1.0.0

# 更新 k8s-deployment.yaml 中的镜像地址
sed -i 's|image: redcell:latest|image: your-registry.io/redcell:1.0.0|g' k8s-deployment.yaml
```

### 3. 创建 Secret

```bash
# 创建 API Keys Secret
kubectl create secret generic redcell-secrets \
  --from-literal=ANTHROPIC_API_KEY="sk-ant-..." \
  --from-literal=DEEPSEEK_API_KEY="sk-..." \
  --from-literal=MSF_PASSWORD="your-password" \
  -n redcell --dry-run=client -o yaml | kubectl apply -f -
```

### 4. 部署应用

```bash
# 部署所有资源
kubectl apply -f k8s-deployment.yaml

# 验证部署
kubectl get all -n redcell

# 查看 Pod 状态
kubectl get pods -n redcell -w

# 查看日志
kubectl logs -f deployment/redcell -n redcell
```

### 5. 访问服务

#### 方式 1: Port Forward (开发环境)

```bash
kubectl port-forward -n redcell svc/redcell 8000:8000
# 访问 http://localhost:8000
```

#### 方式 2: Ingress (生产环境)

```bash
# 配置 DNS
echo "your-k8s-cluster-ip redcell.example.com" >> /etc/hosts

# 访问
open https://redcell.example.com
```

#### 方式 3: NodePort

修改 `k8s-deployment.yaml` 中的 Service:

```yaml
spec:
  type: NodePort
  ports:
  - port: 8000
    targetPort: http
    nodePort: 30800  # 可选，自动分配则删除此行
```

访问 `http://<node-ip>:30800`

### 6. 扩缩容

⚠️ **注意**: Vero 使用 SQLite，不支持多实例。

```bash
# 查看当前副本数
kubectl get deployment redcell -n redcell

# 暂停服务 (设为 0)
kubectl scale deployment redcell --replicas=0 -n redcell

# 恢复服务
kubectl scale deployment redcell --replicas=1 -n redcell
```

### 7. 更新部署

```bash
# 滚动更新
kubectl set image deployment/redcell \
  redcell=your-registry.io/redcell:1.1.0 \
  -n redcell

# 查看更新状态
kubectl rollout status deployment/redcell -n redcell

# 回滚到上一版本
kubectl rollout undo deployment/redcell -n redcell

# 查看历史版本
kubectl rollout history deployment/redcell -n redcell
```

---

## 🔒 生产环境配置

### 1. 环境变量

```bash
# 推荐配置
Vero_DB=/app/data/redcell.db
Vero_PORT=8000
ANTHROPIC_API_KEY=sk-ant-...
DEEPSEEK_API_KEY=sk-...

# 可选配置
LOG_LEVEL=info
MAX_WORKERS=10
TIMEOUT=300
```

### 2. 资源限制

#### Docker

```bash
docker run -d \
  --name redcell \
  --memory="2g" \
  --cpus="2.0" \
  -p 8000:8000 \
  redcell:latest
```

#### Kubernetes

```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "2000m"
```

### 3. 持久化配置

#### Docker Volume

```bash
# 创建命名卷
docker volume create redcell-data

# 使用命名卷
docker run -d \
  -v redcell-data:/app/data \
  redcell:latest
```

#### Kubernetes PVC

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: redcell-data
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: fast-ssd  # 使用 SSD 存储类
```

### 4. 网络配置

#### Docker 网络隔离

```bash
# 创建自定义网络
docker network create --driver bridge redcell-net

# 在网络中运行
docker run -d \
  --name redcell \
  --network redcell-net \
  redcell:latest
```

#### Kubernetes NetworkPolicy

已包含在 `k8s-deployment.yaml` 中，限制：
- Ingress: 只允许 Ingress Controller
- Egress: 允许 DNS + Metasploit RPC + 外部访问

---

## 🛡️ 安全加固

### 1. 镜像安全

```bash
# 扫描漏洞
docker scan redcell:latest

# 使用 Trivy 扫描
trivy image redcell:latest

# 签名镜像 (使用 Docker Content Trust)
export DOCKER_CONTENT_TRUST=1
docker push your-registry.io/redcell:1.0.0
```

### 2. 运行时安全

#### Docker 安全选项

```bash
docker run -d \
  --name redcell \
  --security-opt=no-new-privileges \
  --cap-drop=ALL \
  --cap-add=NET_BIND_SERVICE \
  --read-only \
  --tmpfs /tmp \
  redcell:latest
```

#### Kubernetes Pod Security

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
  readOnlyRootFilesystem: true
```

### 3. Secret 管理

#### 使用 Kubernetes Secrets

```bash
# 从文件创建
kubectl create secret generic redcell-secrets \
  --from-file=api-key=./api-key.txt \
  -n redcell

# 从 Vault (如果使用 HashiCorp Vault)
kubectl create secret generic redcell-secrets \
  --from-literal=ANTHROPIC_API_KEY="$(vault kv get -field=value secret/redcell/api-key)" \
  -n redcell
```

#### 使用 Sealed Secrets (加密 Secret)

```bash
# 安装 Sealed Secrets Controller
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.18.0/controller.yaml

# 创建 Sealed Secret
kubeseal -f secret.yaml -w sealed-secret.yaml

# 部署 Sealed Secret
kubectl apply -f sealed-secret.yaml
```

### 4. 网络安全

#### TLS 配置 (Ingress)

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: redcell
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
spec:
  tls:
  - hosts:
    - redcell.example.com
    secretName: redcell-tls
```

#### 防火墙规则

```bash
# Docker (iptables)
iptables -A INPUT -p tcp --dport 8000 -s 10.0.0.0/8 -j ACCEPT
iptables -A INPUT -p tcp --dport 8000 -j DROP

# Kubernetes (NetworkPolicy 已包含)
```

### 5. 审计日志

#### 启用审计日志

```yaml
# ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: redcell-config
data:
  AUDIT_LOG: "/app/audit/audit.jsonl"
  AUDIT_LEVEL: "info"
```

#### 日志轮转

```bash
# Docker (使用 logrotate)
cat > /etc/logrotate.d/redcell <<EOF
/app/audit/audit.jsonl {
    daily
    rotate 7
    compress
    missingok
    notifempty
}
EOF
```

---

## 📊 监控与日志

### 1. 健康检查

#### Docker

```bash
# 添加健康检查
docker run -d \
  --name redcell \
  --health-cmd="curl -f http://localhost:8000/ || exit 1" \
  --health-interval=30s \
  --health-timeout=3s \
  --health-retries=3 \
  redcell:latest

# 查看健康状态
docker inspect --format='{{.State.Health.Status}}' redcell
```

#### Kubernetes

已包含在 `k8s-deployment.yaml`:
- livenessProbe: 检测服务存活
- readinessProbe: 检测服务就绪

### 2. 日志收集

#### Docker 日志

```bash
# 查看日志
docker logs -f redcell

# 导出日志
docker logs redcell > redcell.log 2>&1

# 使用日志驱动 (发送到 ELK/Splunk)
docker run -d \
  --log-driver=syslog \
  --log-opt syslog-address=tcp://logstash:5000 \
  redcell:latest
```

#### Kubernetes 日志

```bash
# 查看 Pod 日志
kubectl logs -f deployment/redcell -n redcell

# 查看所有副本日志
kubectl logs -l app=redcell -n redcell --all-containers=true

# 导出日志
kubectl logs deployment/redcell -n redcell > redcell.log
```

### 3. Prometheus 监控 (可选)

添加 Prometheus 指标端点 (需修改代码):

```go
// cmd/redcell/main.go
import "github.com/prometheus/client_golang/prometheus/promhttp"

http.Handle("/metrics", promhttp.Handler())
```

ServiceMonitor 配置:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: redcell
  namespace: redcell
spec:
  selector:
    matchLabels:
      app: redcell
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

---

## 🐛 故障排除

### 1. 容器无法启动

**症状**: 容器状态为 `CrashLoopBackOff`

**诊断**:
```bash
# Docker
docker logs redcell

# Kubernetes
kubectl describe pod <pod-name> -n redcell
kubectl logs <pod-name> -n redcell --previous
```

**常见原因**:
- 数据库文件权限问题
- 端口冲突
- 缺少环境变量

**解决**:
```bash
# 修复权限
chown -R 1000:1000 ./data

# 检查端口占用
netstat -tulpn | grep 8000

# 验证环境变量
docker exec redcell env | grep Vero
```

### 2. 无法访问服务

**症状**: `curl http://localhost:8000` 超时

**诊断**:
```bash
# 检查容器状态
docker ps -a | grep redcell

# 检查端口映射
docker port redcell

# 检查防火墙
iptables -L -n | grep 8000
```

**解决**:
```bash
# 重新映射端口
docker run -d -p 8000:8000 redcell:latest

# 检查服务监听
docker exec redcell netstat -tlnp | grep 8000
```

### 3. 数据库锁定

**症状**: `database is locked`

**原因**: 多个进程同时访问 SQLite

**解决**:
```bash
# 确保只有一个副本
kubectl scale deployment redcell --replicas=1 -n redcell

# 检查文件锁
lsof /app/data/redcell.db
```

### 4. OOM (内存不足)

**症状**: 容器被 OOM Killer 杀死

**诊断**:
```bash
# Docker
docker stats redcell

# Kubernetes
kubectl top pod -n redcell
```

**解决**:
```yaml
# 增加内存限制
resources:
  limits:
    memory: "4Gi"  # 从 2Gi 增加到 4Gi
```

---

## 📝 最佳实践

### 1. 生产环境 Checklist

- [ ] 使用非 root 用户运行
- [ ] 启用 TLS/HTTPS
- [ ] 配置资源限制
- [ ] 启用健康检查
- [ ] 配置审计日志
- [ ] 设置日志轮转
- [ ] 启用网络策略
- [ ] 使用 Secret 管理 API Keys
- [ ] 定期备份数据库
- [ ] 配置监控告警

### 2. 备份策略

```bash
# 手动备份 (Docker)
docker exec redcell sqlite3 /app/data/redcell.db ".backup /app/data/backup-$(date +%Y%m%d).db"
docker cp redcell:/app/data/backup-$(date +%Y%m%d).db ./backups/

# 自动备份 (Kubernetes CronJob)
apiVersion: batch/v1
kind: CronJob
metadata:
  name: redcell-backup
  namespace: redcell
spec:
  schedule: "0 2 * * *"  # 每天凌晨 2 点
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: redcell:latest
            command: ["/bin/sh", "-c"]
            args:
            - sqlite3 /app/data/redcell.db ".backup /app/data/backup-$(date +%Y%m%d).db"
            volumeMounts:
            - name: data
              mountPath: /app/data
          restartPolicy: OnFailure
          volumes:
          - name: data
            persistentVolumeClaim:
              claimName: redcell-data
```

### 3. 版本管理

```bash
# 镜像标签策略
your-registry.io/redcell:latest       # 最新版本 (不推荐生产使用)
your-registry.io/redcell:1.0.0        # 语义化版本
your-registry.io/redcell:1.0.0-alpine # 变体版本
your-registry.io/redcell:sha-abc123   # Git commit SHA
```

---

## 🔗 相关资源

- **Dockerfile 最佳实践**: https://docs.docker.com/develop/develop-images/dockerfile_best-practices/
- **Kubernetes 安全**: https://kubernetes.io/docs/concepts/security/
- **Helm Charts** (待开发): `helm install redcell ./charts/redcell`

---

**最后更新**: 2026-07-28  
**维护者**: Vero Team
