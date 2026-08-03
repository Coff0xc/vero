# Vero Benchmark 评估报告

**状态**: 模板/占位 —— 本文件将由 `bash benchmark/run_benchmark.sh` 的实测结果回填。
以下所有表格为**待实测占位**, 不包含任何 mock 数据(旧版"Vero 100% vs 传统 0%"对比
为未实测的模拟, 已从本报告移除, 见 `benchmark/README.md` Legacy 说明)。

---

## 1. 实测流程

```bash
bash benchmark/run_benchmark.sh
# 产出: benchmark/results/<scenario>/result.json + benchmark/results/benchmark-result.json
```

## 2. 结果回填(实测后填写)

| 场景 | recall | precision | 误报率 | 幻觉率 | 证据违规 | 攻击链 | verdict |
|------|--------|-----------|--------|--------|---------|--------|---------|
| juice-shop | — | — | — | — | — | — | — |
| dvwa | — | — | — | — | — | — | — |
| log4shell | — | — | — | — | — | — | — |

## 3. 指标定义

见 [benchmark/README.md 指标定义节](README.md#指标定义)。

- **幻觉率** = `evidence_violations / confirmed`(证据逐字回查失败占比, 产品内核真算法)
- **误报率** = confirmed finding 中未命中 ground truth 的占比
- **攻击链** = confirmed 边 BFS 从 service 到 foothold 是否存在路径

## 4. 判读原则

1. `evidence_violations > 0` 是首要红旗 —— 证据驱动架构的核心承诺是零幻觉,
   任何违规都必须人工复核(`details.evidence_violation_nodes`)。
2. `recall=0` 多为**工具链覆盖缺口**(如 log4shell 场景缺 cves 模板与 OOB 服务器),
   是能力边界信号, 不是缺陷。
3. `decoy_hits > 0` 是高危误报 —— 把无关漏洞当真的情况最伤可信度。

## 5. 已知工具链覆盖边界(截至 2026-08)

| 缺口 | 影响 | 补齐路径 |
|------|------|---------|
| web_vuln_scan 仅 `-tags tech,exposure,misconfig` | log4shell 等 CVE 无法命中 | nuclei 全模板库或 cves tag |
| 无 OOB LDAP/DNS 回连服务器 | 无法证明 log4j JNDI 利用回连 | 部署 marshalsec + dnslog |
| 无登录后会话工具 | DVWA 漏洞页不可达 | 增加带 cookie 的会话工具 |
| 无真实利用后阶段工具(webshell/横向) | 攻击链 `attack_chain_success` 恒 false | 接入 shell/横向移动工具 |

---

**最后实测时间**: —
**负责人**: Vero Team
