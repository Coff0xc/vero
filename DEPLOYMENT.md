# Vero 部署指南

**版本**: 1.1.0  
**更新日期**: 2026-08-03

---

## 📋 目录

1. [Docker 部署](#docker-部署)
2. [Docker Compose 部署](#docker-compose-部署)
3. [Kubernetes 部署](#kubernetes-部署)
4. [生产环境配置](#生产环境配置)
5. [Web 工作台功能](#web-工作台功能)
6. [安全加固](#安全加固)
7. [监控与日志](#监控与日志)
8. [故障排除](#故障排除)

---

## 🐳 Docker 部署

### 1. 构建镜像

```bash
# 克隆仓库
git clone https://github.com/your-org/VERO
cd VERO

# 构建镜像
docker build -t VERO:latest .

# 验证构建
docker images | grep VERO
```

### 2. 运行容器

```bash
# 基础运行
docker run -d \
  --name VERO \
  -p 8000:8000 \
  -v $(pwd)/data:/app/data \
  VERO:latest

# 带 API Key
docker run -d \
  --name VERO \
  -p 8000:8000 \
  -e ANTHROPIC_API_KEY="sk-ant-..." \
  -v $(pwd)/data:/app/data \
  VERO:latest

# 访问
open http://localhost:8000
```

### 3. 独立工具模式

```bash
# 容器逃逸检测 (在特权容器内)
docker run --rm --privileged \
  -v /var/run/docker.sock:/var/run/docker.sock \
  VERO:latest \
  -container-escape check

# S3 检测
docker run --rm \
  VERO:latest \
  -cloud-s3 my-bucket

# 自检模式
docker run --rm VERO:latest -selfcheck
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

> 💡 API Keys 也可在 Web 控制台「设置」页运行期配置(写入 `vero.config.json`, 权限 0600)。注意: 配置文件优先级高于环境变量, 保存后以「设置」页值为准。

创建目录结构：

```bash
mkdir -p data wordlists audit
```

### 2. 启动服务

```bash
# 启动所有服务 (Vero + Metasploit + PostgreSQL)
docker-compose up -d

# 只启动 Vero
docker-compose up -d VERO

# 查看日志
docker-compose logs -f VERO

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
docker build -t your-registry.io/VERO:1.0.0 .

# 推送到镜像仓库
docker push your-registry.io/VERO:1.0.0

# 更新 k8s-deployment.yaml 中的镜像地址
sed -i 's|image: VERO:latest|image: your-registry.io/VERO:1.0.0|g' k8s-deployment.yaml
```

### 3. 创建 Secret

```bash
# 创建 API Keys Secret
kubectl create secret generic VERO-secrets \
  --from-literal=ANTHROPIC_API_KEY="sk-ant-..." \
  --from-literal=DEEPSEEK_API_KEY="sk-..." \
  --from-literal=MSF_PASSWORD="your-password" \
  -n VERO --dry-run=client -o yaml | kubectl apply -f -
```

### 4. 部署应用

```bash
# 部署所有资源
kubectl apply -f k8s-deployment.yaml

# 验证部署
kubectl get all -n VERO

# 查看 Pod 状态
kubectl get pods -n VERO -w

# 查看日志
kubectl logs -f deployment/VERO -n VERO
```

### 5. 访问服务

#### 方式 1: Port Forward (开发环境)

```bash
kubectl port-forward -n VERO svc/VERO 8000:8000
# 访问 http://localhost:8000
```

#### 方式 2: Ingress (生产环境)

```bash
# 配置 DNS
echo "your-k8s-cluster-ip VERO.example.com" >> /etc/hosts

# 访问
open https://VERO.example.com
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
kubectl get deployment VERO -n VERO

# 暂停服务 (设为 0)
kubectl scale deployment VERO --replicas=0 -n VERO

# 恢复服务
kubectl scale deployment VERO --replicas=1 -n VERO
```

### 7. 更新部署

```bash
# 滚动更新
kubectl set image deployment/VERO \
  VERO=your-registry.io/VERO:1.1.0 \
  -n VERO

# 查看更新状态
kubectl rollout status deployment/VERO -n VERO

# 回滚到上一版本
kubectl rollout undo deployment/VERO -n VERO

# 查看历史版本
kubectl rollout history deployment/VERO -n VERO
```

---

## 🔒 生产环境配置

### 1. 环境变量

```bash
# 推荐配置
Vero_DB=/app/data/VERO.db
Vero_PORT=8000
ANTHROPIC_API_KEY=sk-ant-...
DEEPSEEK_API_KEY=sk-...

# 可选配置
LOG_LEVEL=info
MAX_WORKERS=10
TIMEOUT=300
VERO_MODEL=claude-opus-4-8   # 模型名, 留空 = 引擎默认
```

> 引擎选择、API Key、模型名、思考强度与决策预算也可在 Web 控制台「设置」页运行期配置(见 [Web 工作台功能](#web-工作台功能)), 无需重启。读取优先级: 配置文件 `vero.config.json` > 环境变量 > 默认值; 在「设置」页保存过的值会覆盖同名的环境变量。

### 2. 资源限制

#### Docker

```bash
docker run -d \
  --name VERO \
  --memory="2g" \
  --cpus="2.0" \
  -p 8000:8000 \
  VERO:latest
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
docker volume create VERO-data

# 使用命名卷
docker run -d \
  -v VERO-data:/app/data \
  VERO:latest
```

#### Kubernetes PVC

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: VERO-data
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
docker network create --driver bridge VERO-net

# 在网络中运行
docker run -d \
  --name VERO \
  --network VERO-net \
  VERO:latest
```

#### Kubernetes NetworkPolicy

已包含在 `k8s-deployment.yaml` 中，限制：
- Ingress: 只允许 Ingress Controller
- Egress: 允许 DNS + Metasploit RPC + 外部访问

---

## 🖥️ Web 工作台功能

> 本版本新增的 Web 控制台能力:「设置」面板、工具自动安装、全中文界面与思考展示、战役阶段进度条。均随镜像内置, 无需额外配置即可使用。

### 1. 工作台「设置」面板

Web 控制台新增第 5 个 Tab「设置」(前端 `web/src/components/SettingsPanel.tsx`), 用于在运行期配置决策引擎与模型参数, 无需重启:

- **决策引擎**: 自动 / Claude / DeepSeek / 脚本 四选一(下拉带中文说明)。
  - `自动`: 已配置任一 API Key 时使用真实模型, 否则回退脚本模式;
  - `Claude` / `DeepSeek`: 强制使用对应模型, 未配置对应 Key 时发起战役将回退脚本模式;
  - `脚本`: 固定确定性脚本序列, 无需任何 Key。
- **API Key**: `ANTHROPIC_API_KEY` / `DEEPSEEK_API_KEY` 密码框, 显示「已配置/未配置」徽标并可一键清除。界面不回显明文(后端只回布尔 `has_anthropic` / `has_deepseek`); 密钥留空表示「不修改已配置值」, 显式清空才发送 `clear_anthropic` / `clear_deepseek`。
- **模型名**: 留空 = 引擎默认(`claude-opus-4-8` / `deepseek-chat`)。
- **思考强度**: 0~1 滑块, 低 = 稳健, 高 = 发散(对应 LLM temperature)。
- **决策预算**: 单次战役的决策迭代轮数上限(默认 10)。
- **恢复默认**: 一键恢复引擎 `auto` / 思考强度 0.2 / 预算 10。

后端 API:

- `GET /api/config` — 返回 `engine` / `model` / `temperature` / `max_budget` / `has_anthropic` / `has_deepseek`(密钥只回布尔, 不回明文)。
- `POST /api/config` — 可设置 `engine` / `anthropic_key` / `deepseek_key` / `clear_anthropic` / `clear_deepseek` / `model` / `temperature` / `max_budget`; 字段可部分提交, 空 key 字段 = 不改。

配置存储于工作目录下的 `vero.config.json`(权限 0600, 密钥只写盘不回显)。读取优先级: **配置文件 > 环境变量 > 默认值** —— 在「设置」页保存过的值会覆盖同名的环境变量; 生产环境建议优先使用 Secret 注入(见 [安全加固](#安全加固))。

### 2. 工具自动下载安装

为解决「工具列表齐全但本机缺二进制、能力悬空」的问题, 本版本支持一键自动安装缺失工具:

- **二进制工具**(`nuclei` v3.3.9、`ffuf` v2.1.0): 自动下载到 `<工作目录>/tools/bin` 并注入进程 PATH(仅本进程, 不动系统 PATH)。版本与校验和为硬编码白名单(SHA256 校验, 防供应链投毒), 仅支持 **amd64** 平台; 其他架构会明确拒绝并提示手动安装。
- **Python 系工具**(`nxc`→`netexec`、`impacket`、`pypykatz`、`secretsdump`→`impacket`、`lsass_dump`→`pypykatz`、`sam_dump`→`impacket`): 通过 `pip install --user` 安装(不污染系统环境, 优先 `python3`, Windows 兼容 `python` / `py`); 系统托管 Python 拒绝裸 `--user`(PEP 668)时自动追加 `--break-system-packages` 重试一次。

Web 工具管理页(ToolManager)为每个缺失工具区分「自动下载(二进制)」/「一键安装(pip)」按钮, 顶部「全部自动安装」批量安装所有缺失工具。

后端 API:

- `POST /api/tools/install` — 单工具安装, body `{name, type?}`, `type` 可选 `binary` / `pip`, 缺省按工具自动判定; 显式提供时须与实际安装途径一致, 否则返回 422。
- `POST /api/tools/install-all` — 批量安装缺失工具, 支持 `{names, types}` 过滤, 全程串行; 单项失败不影响其余项, 部分失败整体仍返回 200。
- `GET/POST /api/tools/verify` — 工具可用性校验, 每个工具新增三态 `install_type`(`binary` / `pip` / `none`), 以及 `installable`(可下载的二进制名)、`pip_hint`(手动 pip 命令)。

> 镜像内已预装 nuclei / ffuf / netexec / impacket(见 Dockerfile, 与自动安装同一版本与 SHA256)。自动安装主要面向镜像未覆盖的宿主环境(如裸机、Windows 开发机)。

### 3. 全中文界面与思考展示

- 前端新增 `web/src/lib/i18n.ts` 集中中文文案映射: 事件标签(思考 / 工具 / 授权请求 / 计划等)、工具级别(利用级等)、攻击图节点状态(已证实 / 待验证)、决策引擎中文说明、战役阶段。
- 信号流 SignalStream 全中文展示; `step` 事件展开显示「思考 L{级} · 工具」+「▍推理 {why}」, `plan` 事件以高亮块整段展示计划推理 `rationale`(即 LLM 每一步的思考内容)。

### 4. 战役阶段进度条

- 前端新增 StageProgress 组件: 待命 → 侦察 → 扫描 → 利用 → 完成, 由 SSE 事件推断当前阶段(只前进不后退), 实时显示当前动作与工具名, 集成进 KPI 面板。

---

## 🛡️ 安全加固

### 1. 镜像安全

```bash
# 扫描漏洞
docker scan VERO:latest

# 使用 Trivy 扫描
trivy image VERO:latest

# 签名镜像 (使用 Docker Content Trust)
export DOCKER_CONTENT_TRUST=1
docker push your-registry.io/VERO:1.0.0
```

### 2. 运行时安全

#### Docker 安全选项

```bash
docker run -d \
  --name VERO \
  --security-opt=no-new-privileges \
  --cap-drop=ALL \
  --cap-add=NET_BIND_SERVICE \
  --read-only \
  --tmpfs /tmp \
  VERO:latest
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
kubectl create secret generic VERO-secrets \
  --from-file=api-key=./api-key.txt \
  -n VERO

# 从 Vault (如果使用 HashiCorp Vault)
kubectl create secret generic VERO-secrets \
  --from-literal=ANTHROPIC_API_KEY="$(vault kv get -field=value secret/VERO/api-key)" \
  -n VERO
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
  name: VERO
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
spec:
  tls:
  - hosts:
    - VERO.example.com
    secretName: VERO-tls
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
  name: VERO-config
data:
  AUDIT_LOG: "/app/audit/audit.jsonl"
  AUDIT_LEVEL: "info"
```

#### 日志轮转

```bash
# Docker (使用 logrotate)
cat > /etc/logrotate.d/VERO <<EOF
/app/audit/audit.jsonl {
    daily
    rotate 7
    compress
    missingok
    notifempty
}
EOF
```

### 6. 运行时配置与工具下载安全

- **密钥存储**: 工作台「设置」页保存的密钥写入工作目录 `vero.config.json`(权限 0600), Web 界面与 API 永不回显明文(仅返回「是否已配置」)。容器化生产环境建议仍优先使用环境变量或 Kubernetes Secret 注入(见上文 Secret 管理), 而非在界面录入。
- **工具自动下载**: 仅内置 nuclei / ffuf 两个纯编译二进制, 版本与 SHA256 校验和硬编码为白名单, 校验失败即拒绝安装(防供应链投毒), 且仅限 amd64 平台; Python 系工具仅通过 `pip --user` 安装, 不修改系统 Python 环境(见 [Web 工作台功能](#web-工作台功能))。

---

## 📊 监控与日志

### 1. 健康检查

#### Docker

```bash
# 添加健康检查
docker run -d \
  --name VERO \
  --health-cmd="curl -f http://localhost:8000/ || exit 1" \
  --health-interval=30s \
  --health-timeout=3s \
  --health-retries=3 \
  VERO:latest

# 查看健康状态
docker inspect --format='{{.State.Health.Status}}' VERO
```

#### Kubernetes

已包含在 `k8s-deployment.yaml`:
- livenessProbe: 检测服务存活
- readinessProbe: 检测服务就绪

### 2. 日志收集

#### Docker 日志

```bash
# 查看日志
docker logs -f VERO

# 导出日志
docker logs VERO > VERO.log 2>&1

# 使用日志驱动 (发送到 ELK/Splunk)
docker run -d \
  --log-driver=syslog \
  --log-opt syslog-address=tcp://logstash:5000 \
  VERO:latest
```

#### Kubernetes 日志

```bash
# 查看 Pod 日志
kubectl logs -f deployment/VERO -n VERO

# 查看所有副本日志
kubectl logs -l app=VERO -n VERO --all-containers=true

# 导出日志
kubectl logs deployment/VERO -n VERO > VERO.log
```

### 3. Prometheus 监控 (可选)

添加 Prometheus 指标端点 (需修改代码):

```go
// cmd/VERO/main.go
import "github.com/prometheus/client_golang/prometheus/promhttp"

http.Handle("/metrics", promhttp.Handler())
```

ServiceMonitor 配置:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: VERO
  namespace: VERO
spec:
  selector:
    matchLabels:
      app: VERO
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
docker logs VERO

# Kubernetes
kubectl describe pod <pod-name> -n VERO
kubectl logs <pod-name> -n VERO --previous
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
docker exec VERO env | grep Vero
```

### 2. 无法访问服务

**症状**: `curl http://localhost:8000` 超时

**诊断**:
```bash
# 检查容器状态
docker ps -a | grep VERO

# 检查端口映射
docker port VERO

# 检查防火墙
iptables -L -n | grep 8000
```

**解决**:
```bash
# 重新映射端口
docker run -d -p 8000:8000 VERO:latest

# 检查服务监听
docker exec VERO netstat -tlnp | grep 8000
```

### 3. 数据库锁定

**症状**: `database is locked`

**原因**: 多个进程同时访问 SQLite

**解决**:
```bash
# 确保只有一个副本
kubectl scale deployment VERO --replicas=1 -n VERO

# 检查文件锁
lsof /app/data/VERO.db
```

### 4. OOM (内存不足)

**症状**: 容器被 OOM Killer 杀死

**诊断**:
```bash
# Docker
docker stats VERO

# Kubernetes
kubectl top pod -n VERO
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
docker exec VERO sqlite3 /app/data/VERO.db ".backup /app/data/backup-$(date +%Y%m%d).db"
docker cp VERO:/app/data/backup-$(date +%Y%m%d).db ./backups/

# 自动备份 (Kubernetes CronJob)
apiVersion: batch/v1
kind: CronJob
metadata:
  name: VERO-backup
  namespace: VERO
spec:
  schedule: "0 2 * * *"  # 每天凌晨 2 点
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: VERO:latest
            command: ["/bin/sh", "-c"]
            args:
            - sqlite3 /app/data/VERO.db ".backup /app/data/backup-$(date +%Y%m%d).db"
            volumeMounts:
            - name: data
              mountPath: /app/data
          restartPolicy: OnFailure
          volumes:
          - name: data
            persistentVolumeClaim:
              claimName: VERO-data
```

### 3. 版本管理

```bash
# 镜像标签策略
your-registry.io/VERO:latest       # 最新版本 (不推荐生产使用)
your-registry.io/VERO:1.0.0        # 语义化版本
your-registry.io/VERO:1.0.0-alpine # 变体版本
your-registry.io/VERO:sha-abc123   # Git commit SHA
```

---

## 🔗 相关资源

- **Dockerfile 最佳实践**: https://docs.docker.com/develop/develop-images/dockerfile_best-practices/
- **Kubernetes 安全**: https://kubernetes.io/docs/concepts/security/
- **Helm Charts** (待开发): `helm install VERO ./charts/VERO`

---

**最后更新**: 2026-08-03  
**维护者**: Vero Team
