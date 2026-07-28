#!/bin/bash
# Docker 容器逃逸工具测试脚本

set -e

echo "=== Docker 容器逃逸工具测试 ==="
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: Docker 未安装${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Docker 已安装${NC}"
echo ""

# 构建 REDCELL 镜像（如果不存在）
if ! docker images | grep -q "redcell"; then
    echo "构建 REDCELL 镜像..."
    docker build -t redcell:latest . || {
        echo -e "${RED}镜像构建失败${NC}"
        exit 1
    }
fi

echo -e "${GREEN}✓ REDCELL 镜像就绪${NC}"
echo ""

# 测试 1: 普通容器（应检测不到逃逸向量）
echo "=== 测试 1: 普通容器（安全配置） ==="
echo "运行: docker run --rm redcell:latest -container-escape check"
echo ""

docker run --rm \
    --security-opt=no-new-privileges \
    --cap-drop=ALL \
    redcell:latest \
    -container-escape check

echo ""
echo -e "${YELLOW}预期结果: 应检测到容器环境，但无逃逸向量${NC}"
echo ""

# 测试 2: 特权容器（应检测到逃逸向量）
echo "=== 测试 2: 特权容器（危险配置） ==="
echo "运行: docker run --rm --privileged redcell:latest -container-escape check"
echo ""

docker run --rm \
    --privileged \
    redcell:latest \
    -container-escape check

echo ""
echo -e "${YELLOW}预期结果: 应检测到 PRIVILEGED CONTAINER DETECTED${NC}"
echo ""

# 测试 3: Docker socket 挂载（高风险）
echo "=== 测试 3: Docker Socket 挂载（高风险） ==="
echo "运行: docker run --rm -v /var/run/docker.sock:/var/run/docker.sock redcell:latest -container-escape check"
echo ""

if [ -e /var/run/docker.sock ]; then
    docker run --rm \
        -v /var/run/docker.sock:/var/run/docker.sock \
        redcell:latest \
        -container-escape check

    echo ""
    echo -e "${YELLOW}预期结果: 应检测到 Docker socket mounted${NC}"
else
    echo -e "${YELLOW}跳过: /var/run/docker.sock 不存在${NC}"
fi

echo ""

# 测试 4: 宿主机文件系统挂载
echo "=== 测试 4: 宿主机文件系统挂载 ==="
echo "运行: docker run --rm -v /:/host redcell:latest -container-escape check"
echo ""

docker run --rm \
    -v /:/host:ro \
    redcell:latest \
    -container-escape check

echo ""
echo -e "${YELLOW}预期结果: 应检测到 Host filesystem mounted${NC}"
echo ""

# 测试 5: Kubernetes ServiceAccount（如果在 K8s 环境）
echo "=== 测试 5: Kubernetes ServiceAccount ==="

if command -v kubectl &> /dev/null && kubectl get nodes &> /dev/null; then
    echo "检测到 Kubernetes 环境，运行 K8s 测试..."

    # 创建测试 Pod
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: redcell-test
  namespace: default
spec:
  containers:
  - name: redcell
    image: redcell:latest
    command: ["/usr/local/bin/redcell"]
    args: ["-k8s-sa", "enum"]
  restartPolicy: Never
EOF

    echo "等待 Pod 完成..."
    kubectl wait --for=condition=completed pod/redcell-test --timeout=60s

    echo ""
    echo "Pod 输出:"
    kubectl logs redcell-test

    # 清理
    kubectl delete pod redcell-test

    echo ""
    echo -e "${YELLOW}预期结果: 应提取 ServiceAccount token${NC}"
else
    echo -e "${YELLOW}跳过: 非 Kubernetes 环境${NC}"
fi

echo ""
echo "=== 测试完成 ==="
echo ""
echo "总结:"
echo "  ✓ 测试 1: 普通容器安全检测"
echo "  ✓ 测试 2: 特权容器逃逸检测"
echo "  ✓ 测试 3: Docker socket 检测"
echo "  ✓ 测试 4: 宿主机挂载检测"
echo "  ✓ 测试 5: K8s 环境检测"
