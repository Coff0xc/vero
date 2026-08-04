# Vero 渗透测试智能体 - 能力融合路线图

## 已完成 ✅

### 核心框架 (4350b03 + 2ecad22)
- [x] **参数规格系统** - 25+ 工具 ArgSpec + 执行前校验
- [x] **长程记忆** - 老步骤保留证据单行，逐字可溯源
- [x] **后渗透闭环** - msf_session_cmd 在失陷主机执行命令
- [x] **注入防护** - 数据边界框架三层强化
- [x] **预算扩容** - 8→20 步全栈同步
- [x] **工具依赖检测** - 7 个核心工具自动检测 + API (抄 Dark-Moon)
- [x] **Browser Agent 框架** - Playwright 封装 + 4 工具 (抄 PentAGI)

### 现有场景包
- [x] **Recon Pack** - port_scan, http_probe, fetch_page, extract_endpoints
- [x] **Web Pack** - web_vuln_scan, ffuf_dir_brute, ffuf_vhost_enum, exploit_sqli
- [x] **AD Pack** - smb_enum, kerberoast, nxc_* (7 个工具)
- [x] **Post-Exploit Pack** - secretsdump, lsass_dump, sam_dump
- [x] **Metasploit Pack** - msf_search, msf_exploit, msf_sessions, msf_session_cmd

---

## 16 项目能力融合计划 (优先级排序)

### 第 1 阶段: 核心增强 (3-5 天) 🔥

#### 1. **代码审计能力** (抄 DeepAudit + BugTraceAI)
**目标**: 静态代码扫描 + 污点分析 + LLM 辅助审计
```
- [ ] CodeAuditPack 场景包
  - [ ] semgrep_scan - 规则引擎扫描 (SAST)
  - [ ] bandit_scan - Python 安全检查
  - [ ] eslint_security - JS/TS 漏洞检测
  - [ ] codeql_analyze - 污点分析 (需 GitHub token)
  - [ ] dependency_check - 依赖漏洞扫描 (OWASP)
- [ ] Parser: CVE 提取 + 污点路径可视化
- [ ] 工具依赖: semgrep, bandit, codeql-cli
```
**参考项目**: 
- DeepAudit (https://github.com/deepaudit/deepaudit) - 污点分析 + LLM
- BugTraceAI (https://github.com/bugtrace/bugtrace-ai) - SAST 集成

#### 2. **云渗透能力** (抄 Shannon + NOVA)
**目标**: AWS/Azure/GCP 配置审计 + 权限提升
```
- [ ] CloudPack 场景包
  - [ ] aws_enum_s3 - S3 桶枚举 + 公开检测
  - [ ] aws_iam_privesc - IAM 权限提升路径
  - [ ] azure_tenant_enum - Azure AD 枚举
  - [ ] gcp_project_enum - GCP 项目资产发现
  - [ ] cloud_metadata_exploit - SSRF → 元数据服务
- [ ] 工具依赖: aws-cli, az-cli, gcloud, ScoutSuite, CloudSploit
```
**参考项目**:
- Shannon (https://github.com/vxcontrol/shannon) - 隔离沙箱 + 云 API
- NOVA (https://github.com/nova-project/nova) - 多云编排

#### 3. **容器/K8s 渗透** (抄 Reaper + ThreatCanvas)
**目标**: Docker 逃逸 + K8s RBAC 提权
```
- [ ] K8sPack 场景包
  - [ ] k8s_enum_pods - Pod 枚举 + ServiceAccount token
  - [ ] k8s_rbac_check - RBAC 权限矩阵分析
  - [ ] docker_escape - 容器逃逸检测 (CAP_SYS_ADMIN, cgroup)
  - [ ] helm_scan - Helm Chart 配置审计
- [ ] 工具依赖: kubectl, kubeletctl, amicontained
```
**参考项目**:
- Reaper (https://github.com/reaper-security/reaper) - K8s 攻击链
- ThreatCanvas (https://github.com/threatcanvas/threatcanvas) - 容器威胁建模

---

### 第 2 阶段: 智能增强 (5-7 天) 🧠

#### 4. **多模态能力** (抄 PentAGI)
**目标**: 图像识别 + 音频分析 + PDF 解析
```
- [ ] 扩展 Browser Agent
  - [ ] OCR 截图识别验证码/敏感信息
  - [ ] 视频流量分析 (WebRTC)
- [ ] 文档解析工具
  - [ ] pdf_extract - 提取文本/元数据 (pdfminer)
  - [ ] image_ocr - Tesseract OCR
```

#### 5. **反射学习强化** (抄 Xalgorix Reflexion)
**目标**: 失败案例自动总结 + 策略调整
```
- [ ] 增强 Reflector 接口
  - [ ] 失败模式分类 (网络/权限/误报)
  - [ ] 策略库持久化 (SQLite lessons 表)
  - [ ] 自动 retry with 调整参数
- [ ] LLM Prompt 优化
  - [ ] Few-shot 示例动态注入
  - [ ] 错误堆栈智能压缩
```

#### 6. **协同编排** (抄 AttackMate 分布式)
**目标**: 多 Agent 并行 + 任务分配
```
- [ ] 分布式引擎
  - [ ] Agent 池管理 (Redis 队列)
  - [ ] 子任务拆分 (端口扫描 → 批量并行)
  - [ ] 结果汇聚 + 去重
```

---

### 第 3 阶段: 生态集成 (7-10 天) 🔌

#### 7. **C2 框架集成** (抄 Mythic)
**目标**: Cobalt Strike / Sliver / Havoc 互操作
```
- [ ] C2Pack 场景包
  - [ ] cs_beacon_deploy - 部署 Beacon
  - [ ] sliver_implant - Sliver 植入
  - [ ] c2_pivot - 代理/隧道建立
```

#### 8. **漏洞利用库** (抄 OWASP Nettacker)
**目标**: CVE 自动化利用 + PoC 管理
```
- [ ] ExploitDB 集成
  - [ ] searchsploit 查询
  - [ ] PoC 自动适配参数
- [ ] 自定义 Exploit 模板系统
```

#### 9. **钓鱼/社工工具** (抄 HackerGPT)
**目标**: 邮件模板生成 + 钓鱼页面克隆
```
- [ ] PhishPack 场景包
  - [ ] email_template_gen - LLM 生成钓鱼邮件
  - [ ] web_clone - 网站克隆 (HTTrack)
  - [ ] credential_harvest - 凭据收集服务器
```

---

### 第 4 阶段: 高级特性 (10-15 天) 🚀

#### 10. **对抗性 AI** (抄 Abyss)
**目标**: WAF 绕过 + IDS 规避
```
- [ ] Evasion 模块
  - [ ] payload_obfuscate - Payload 混淆
  - [ ] traffic_shape - 流量整形 (延迟/抖动)
  - [ ] decoy_scan - 诱饵扫描分散防御
```

#### 11. **报告自动化** (抄 Dark-Moon)
**目标**: CVSS 评分 + 修复建议 + 合规映射
```
- [x] 已有 report.md / report.json (基础)
- [ ] 增强报告
  - [ ] CVSS 3.1 评分计算
  - [ ] CWE/MITRE ATT&CK 映射
  - [ ] 修复优先级排序
  - [ ] Jira/GitHub Issue 自动创建
```

#### 12. **持续监控** (抄 PentestAI)
**目标**: 定时扫描 + 变化告警
```
- [ ] Scheduler 模块
  - [ ] Cron 表达式调度
  - [ ] 基线比对 (新增端口/服务告警)
  - [ ] Webhook 通知 (Slack/钉钉)
```

---

## 技术债务 & 优化

### 性能优化
- [ ] 工具输出流式解析 (不等全部完成再 parse)
- [ ] Agent 状态快照 + 断点续跑
- [ ] 大规模扫描内存优化 (C 段 → 批量迭代)

### 安全加固
- [ ] 工具沙箱隔离 (seccomp / Docker)
- [ ] API Key 加密存储 (不明文 config.json)
- [ ] 审计日志完整性校验 (防篡改)

### 用户体验
- [ ] 前端依赖告警 UI (Settings 新 tab)
- [ ] 实时日志流 (SSE 推 tool stdout)
- [ ] 攻击图交互优化 (节点折叠/过滤)

---

## 实施策略

### 原则
1. **每次迭代可验证** - 每个 Pack 都有对应测试用例
2. **工具独立可降级** - 缺依赖时优雅跳过，不阻塞主流程
3. **证据链完整** - 所有 Parser 保留逐字可查的 Excerpt
4. **版权合规** - 不直接复制代码，仅参考设计模式

### 验证方式
- **单元测试** - 每个工具的 Args/Parser
- **集成测试** - 靶场环境 (DVWA, HackTheBox)
- **压力测试** - 100+ 目标并发扫描

### 里程碑
- **M1 (1 周)**: 代码审计 + 云渗透 Pack 上线
- **M2 (2 周)**: K8s Pack + 多模态 + 反射学习
- **M3 (3 周)**: C2 集成 + 漏洞库 + 钓鱼工具
- **M4 (4 周)**: 对抗 AI + 高级报告 + 持续监控

---

## 参考项目清单

| # | 项目 | 核心能力 | 优先级 | 状态 |
|---|------|---------|-------|------|
| 1 | Shannon | 隔离沙箱 + 云 API | ⭐⭐⭐ | ✅ 已抄框架设计 |
| 2 | PentAGI | Memory + Browser Agent | ⭐⭐⭐ | ✅ 已抄 Browser |
| 3 | Dark-Moon | 依赖检测 + 报告 | ⭐⭐⭐ | ✅ 已抄依赖检测 |
| 4 | Mythic | C2 互操作 | ⭐⭐⭐ | ✅ 已抄 session_cmd |
| 5 | DeepAudit | 代码审计 | ⭐⭐⭐ | 🔄 第 1 阶段 |
| 6 | BugTraceAI | SAST 集成 | ⭐⭐⭐ | 🔄 第 1 阶段 |
| 7 | NOVA | 多云编排 | ⭐⭐⭐ | 🔄 第 1 阶段 |
| 8 | Reaper | K8s 攻击链 | ⭐⭐ | 🔄 第 1 阶段 |
| 9 | ThreatCanvas | 容器威胁建模 | ⭐⭐ | 🔄 第 1 阶段 |
| 10 | Xalgorix | Reflexion 反射学习 | ⭐⭐ | 🔄 第 2 阶段 |
| 11 | AttackMate | 分布式编排 | ⭐⭐ | 🔄 第 2 阶段 |
| 12 | OWASP Nettacker | 漏洞利用库 | ⭐⭐ | 🔄 第 3 阶段 |
| 13 | HackerGPT | 社工/钓鱼 | ⭐ | 🔄 第 3 阶段 |
| 14 | Abyss | 对抗 AI | ⭐ | 🔄 第 4 阶段 |
| 15 | PentestAI | 持续监控 | ⭐ | 🔄 第 4 阶段 |
| 16 | bughunter-ai | 赏金猎人模式 | ⭐ | 🔄 第 4 阶段 |

---

**当前进度**: 核心框架 ✅ | 第 1 阶段启动 🚀
**下一步**: 实现 CodeAuditPack (semgrep + bandit)
