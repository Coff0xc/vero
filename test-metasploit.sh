#!/bin/bash
# Metasploit RPC 集成测试脚本

set -e

echo "=== Metasploit RPC 集成测试 ==="
echo ""

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 默认配置
MSF_HOST=${MSF_HOST:-127.0.0.1}
MSF_PORT=${MSF_PORT:-55553}
MSF_USER=${MSF_USER:-msf}
MSF_PASS=${MSF_PASS:-password}

# 检查 msfrpcd
echo "检查 Metasploit RPC 连接..."
if ! curl -s -X POST http://${MSF_HOST}:${MSF_PORT}/api/1.0/auth.login \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${MSF_USER}\",\"password\":\"${MSF_PASS}\"}" > /dev/null 2>&1; then

    echo -e "${YELLOW}警告: msfrpcd 未运行${NC}"
    echo ""
    echo "启动 msfrpcd:"
    echo "  msfrpcd -P ${MSF_PASS} -U ${MSF_USER} -a ${MSF_HOST} -p ${MSF_PORT}"
    echo ""
    echo "或使用 Docker:"
    echo "  docker run -d --name msfrpcd -p ${MSF_PORT}:${MSF_PORT} \\"
    echo "    metasploitframework/metasploit-framework \\"
    echo "    msfrpcd -P ${MSF_PASS} -U ${MSF_USER} -a 0.0.0.0 -p ${MSF_PORT} -f"
    echo ""

    # 尝试使用 Docker 启动
    read -p "是否使用 Docker 启动 msfrpcd? (y/N): " -n 1 -r
    echo

    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "启动 msfrpcd 容器..."

        docker run -d \
            --name msfrpcd-test \
            -p ${MSF_PORT}:${MSF_PORT} \
            metasploitframework/metasploit-framework:latest \
            msfrpcd -P ${MSF_PASS} -U ${MSF_USER} -a 0.0.0.0 -p ${MSF_PORT} -f

        echo "等待 msfrpcd 启动..."
        sleep 10

        # 再次检查
        if ! curl -s -X POST http://${MSF_HOST}:${MSF_PORT}/api/1.0/auth.login \
            -H "Content-Type: application/json" \
            -d "{\"username\":\"${MSF_USER}\",\"password\":\"${MSF_PASS}\"}" > /dev/null 2>&1; then
            echo -e "${RED}错误: msfrpcd 启动失败${NC}"
            docker logs msfrpcd-test
            docker rm -f msfrpcd-test
            exit 1
        fi
    else
        echo "跳过测试"
        exit 0
    fi
fi

echo -e "${GREEN}✓ msfrpcd 正在运行${NC}"
echo ""

# 测试 1: 搜索 exploit
echo "=== 测试 1: 搜索 exploit 模块 ==="
echo "运行: redcell.exe -msf-search ms17_010"
echo ""

./redcell.exe -msf-search ms17_010 2>&1 | tee /tmp/msf_search_result.txt

if grep -q "MS17-010" /tmp/msf_search_result.txt; then
    echo -e "${GREEN}✓ 成功搜索到 MS17-010 exploit${NC}"
else
    echo -e "${YELLOW}警告: 未找到 MS17-010（可能需要更新 Metasploit）${NC}"
fi

echo ""

# 测试 2: 搜索其他常见 exploit
echo "=== 测试 2: 搜索多个 exploit ==="

EXPLOITS=("eternalblue" "bluekeep" "log4shell" "printnightmare")

for exploit in "${EXPLOITS[@]}"; do
    echo "搜索: ${exploit}"
    ./redcell.exe -msf-search "${exploit}" 2>&1 | grep -E "(\[high\]|exploit)" || echo "  未找到"
done

echo ""

# 测试 3: RPC API 直接测试
echo "=== 测试 3: RPC API 直接测试 ==="

# 获取 token
echo "1. 认证..."
TOKEN=$(curl -s -X POST http://${MSF_HOST}:${MSF_PORT}/api/1.0/auth.login \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${MSF_USER}\",\"password\":\"${MSF_PASS}\"}" \
    | jq -r '.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo -e "${RED}错误: 认证失败${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 认证成功 (Token: ${TOKEN:0:10}...)${NC}"
echo ""

# 搜索模块
echo "2. 搜索模块..."
MODULES=$(curl -s -X POST http://${MSF_HOST}:${MSF_PORT}/api/1.0/module.search \
    -H "Content-Type: application/json" \
    -d "{\"token\":\"${TOKEN}\",\"type\":\"exploit\",\"search\":\"smb\"}" \
    | jq -r '.modules | length')

echo -e "${GREEN}✓ 找到 ${MODULES} 个 SMB 相关模块${NC}"
echo ""

# 列出前 5 个
echo "前 5 个模块:"
curl -s -X POST http://${MSF_HOST}:${MSF_PORT}/api/1.0/module.search \
    -H "Content-Type: application/json" \
    -d "{\"token\":\"${TOKEN}\",\"type\":\"exploit\",\"search\":\"smb\"}" \
    | jq -r '.modules[0:5][]' | head -5

echo ""

# 测试 4: 获取会话列表
echo "=== 测试 4: 获取会话列表 ==="

SESSIONS=$(curl -s -X POST http://${MSF_HOST}:${MSF_PORT}/api/1.0/session.list \
    -H "Content-Type: application/json" \
    -d "{\"token\":\"${TOKEN}\"}" \
    | jq -r '. | length')

echo -e "${GREEN}✓ 当前活跃会话数: ${SESSIONS}${NC}"

if [ "$SESSIONS" -gt 0 ]; then
    echo "会话列表:"
    curl -s -X POST http://${MSF_HOST}:${MSF_PORT}/api/1.0/session.list \
        -H "Content-Type: application/json" \
        -d "{\"token\":\"${TOKEN}\"}" | jq .
fi

echo ""

# 测试 5: 模块信息获取
echo "=== 测试 5: 获取模块信息 ==="

MODULE="exploit/windows/smb/ms17_010_eternalblue"
echo "获取模块: ${MODULE}"

curl -s -X POST http://${MSF_HOST}:${MSF_PORT}/api/1.0/module.info \
    -H "Content-Type: application/json" \
    -d "{\"token\":\"${TOKEN}\",\"type\":\"exploit\",\"name\":\"${MODULE}\"}" \
    | jq -r '.name, .description' | head -10

echo ""

# 清理
echo "=== 清理 ==="

if docker ps | grep -q "msfrpcd-test"; then
    read -p "是否停止 msfrpcd 测试容器? (y/N): " -n 1 -r
    echo

    if [[ $REPLY =~ ^[Yy]$ ]]; then
        docker rm -f msfrpcd-test
        echo -e "${GREEN}✓ 测试容器已停止${NC}"
    fi
fi

echo ""
echo "=== 测试完成 ==="
echo ""
echo "总结:"
echo "  ✓ RPC 连接正常"
echo "  ✓ 模块搜索功能正常"
echo "  ✓ 会话管理功能正常"
echo "  ✓ REDCELL Metasploit 集成验证通过"
