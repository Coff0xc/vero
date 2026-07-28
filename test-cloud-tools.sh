#!/bin/bash
# 云环境工具测试脚本

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=== 云环境工具测试 ==="
echo ""

# 测试 AWS IMDS
echo "=== 测试 1: AWS EC2 IMDS ==="
echo ""

if curl -s -m 2 http://169.254.169.254/latest/meta-data/ > /dev/null 2>&1; then
    echo -e "${GREEN}✓ 检测到 AWS EC2 环境${NC}"
    echo ""

    echo "运行: redcell.exe -cloud-aws"
    ./redcell.exe -cloud-aws 2>&1 | tee /tmp/aws_result.txt

    if grep -q "AccessKeyId" /tmp/aws_result.txt; then
        echo -e "${GREEN}✓ 成功提取 AWS 临时凭证${NC}"
    else
        echo -e "${YELLOW}警告: 未找到凭证（可能无 IAM 角色）${NC}"
    fi
else
    echo -e "${YELLOW}⊘ 非 AWS EC2 环境，跳过测试${NC}"
fi

echo ""

# 测试 Azure IMDS
echo "=== 测试 2: Azure VM IMDS ==="
echo ""

if curl -s -m 2 -H "Metadata:true" http://169.254.169.254/metadata/instance?api-version=2021-02-01 > /dev/null 2>&1; then
    echo -e "${GREEN}✓ 检测到 Azure VM 环境${NC}"
    echo ""

    echo "运行: redcell.exe -cloud-azure"
    ./redcell.exe -cloud-azure 2>&1 | tee /tmp/azure_result.txt

    if grep -q "access_token" /tmp/azure_result.txt; then
        echo -e "${GREEN}✓ 成功提取 Azure 访问令牌${NC}"
    else
        echo -e "${YELLOW}警告: 未找到令牌（可能无托管标识）${NC}"
    fi
else
    echo -e "${YELLOW}⊘ 非 Azure VM 环境，跳过测试${NC}"
fi

echo ""

# 测试 GCP IMDS
echo "=== 测试 3: GCP Compute Engine IMDS ==="
echo ""

if curl -s -m 2 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/ > /dev/null 2>&1; then
    echo -e "${GREEN}✓ 检测到 GCP Compute Engine 环境${NC}"
    echo ""

    echo "运行: redcell.exe -cloud-gcp"
    ./redcell.exe -cloud-gcp 2>&1 | tee /tmp/gcp_result.txt

    if grep -q "access_token" /tmp/gcp_result.txt; then
        echo -e "${GREEN}✓ 成功提取 GCP 服务账号令牌${NC}"
    else
        echo -e "${YELLOW}警告: 未找到令牌（可能无服务账号）${NC}"
    fi
else
    echo -e "${YELLOW}⊘ 非 GCP Compute Engine 环境，跳过测试${NC}"
fi

echo ""

# 测试 S3 公开访问
echo "=== 测试 4: AWS S3 公开桶扫描 ==="
echo ""

read -p "输入 S3 桶名称进行测试 (留空跳过): " BUCKET_NAME

if [ -n "$BUCKET_NAME" ]; then
    echo ""
    echo "运行: redcell.exe -cloud-s3 ${BUCKET_NAME}"
    ./redcell.exe -cloud-s3 "${BUCKET_NAME}" 2>&1 | tee /tmp/s3_result.txt

    if grep -q "PUBLIC BUCKET" /tmp/s3_result.txt; then
        echo -e "${RED}✗ 警告: 桶可公开访问！${NC}"
    elif grep -q "AUTHENTICATED READ" /tmp/s3_result.txt; then
        echo -e "${YELLOW}⊘ 桶需要身份验证${NC}"
    else
        echo -e "${GREEN}✓ 桶访问受限${NC}"
    fi
else
    echo -e "${YELLOW}⊘ 跳过 S3 测试${NC}"
fi

echo ""

# 环境检测总结
echo "=== 环境检测总结 ==="
echo ""

DETECTED=false

if curl -s -m 2 http://169.254.169.254/latest/meta-data/ > /dev/null 2>&1; then
    echo -e "${BLUE}[AWS]${NC} EC2 实例"
    DETECTED=true
fi

if curl -s -m 2 -H "Metadata:true" http://169.254.169.254/metadata/instance?api-version=2021-02-01 > /dev/null 2>&1; then
    echo -e "${BLUE}[Azure]${NC} 虚拟机"
    DETECTED=true
fi

if curl -s -m 2 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/ > /dev/null 2>&1; then
    echo -e "${BLUE}[GCP]${NC} Compute Engine"
    DETECTED=true
fi

if [ "$DETECTED" = false ]; then
    echo -e "${YELLOW}本地环境（非云平台）${NC}"
    echo ""
    echo "建议在以下环境测试云工具："
    echo "  • AWS EC2 实例（需附加 IAM 角色）"
    echo "  • Azure VM（需启用托管标识）"
    echo "  • GCP Compute Engine（需附加服务账号）"
fi

echo ""
echo "=== 测试完成 ==="
