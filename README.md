# Vero

**Evidence-Driven Autonomous Red Team Agent**

An AI-powered penetration testing agent that requires proof for every finding.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org)

---

## Features

### Evidence-Driven Architecture

Every finding must have tool output as evidence. The type system enforces this constraint:

```go
type Finding struct {
    Title    string
    Severity string
    Evidence Evidence  // Required, not optional
}

func (g *AttackGraph) Confirm(id string, ev Evidence) error {
    if ev == (Evidence{}) {
        return fmt.Errorf("evidence required")
    }
    // ...
}
```

No evidence = no confirmation.

### Human-in-the-Loop Safety

Actions are classified by danger level (L0-L4). High-risk operations require approval:

- **L0-L2**: Auto-execute (scans, enumeration)
- **L3-L4**: Require approval (exploitation, lateral movement)

### Attack Graph

Discoveries are stored as nodes with evidence chains. You can trace how each finding was discovered.

### 32 Built-in Tools

- **Network**: nmap, masscan, rustscan
- **Web**: nuclei, ffuf, sqlmap, nikto
- **Exploit**: metasploit, searchsploit
- **Cloud**: aws-cli, az-cli, gcloud
- **Container**: docker, kubectl, trivy
- **AD**: bloodhound, crackmapexec, mimikatz

### 7 Scenario Packs

Pre-configured tool combinations for common engagements:

- Web Application Testing
- Active Directory
- Cloud (AWS/Azure/GCP)
- Container/Kubernetes
- Post-Exploitation
- External Reconnaissance
- Internal Network

---

## Quick Start

### Requirements

- Go 1.26+
- (Optional) Real tools: nmap, nuclei, curl

### Build

```bash
go build -o vero ./cmd/vero
```

### Run

```bash
# Start web UI
./vero
# Open http://localhost:8000

# CLI mode
./vero -selfcheck  # Offline test (no API key needed)
./vero -nmap target.com  # Real scan
```

### With Claude API

```bash
export ANTHROPIC_API_KEY=sk-ant-...
export VERO_MODEL=claude-opus-4-8
./vero
```

---

## Architecture

```
Tools (32 built-in)
  ↓
Core (Evidence Graph + Verify + HITL)
  ↓
Scenarios (7 packs: Web/AD/Cloud/Container)
  ↓
Reports (Markdown/HTML/JSON)
```

**Core Loop**:
1. LLM proposes action
2. System checks danger level
3. Tool executes (if approved)
4. Evidence captured
5. Graph updated
6. Repeat

---

## Documentation

- **[User Manual](USER_MANUAL.md)** - Complete guide (60 pages)
- **[Deployment Guide](DEPLOYMENT.md)** - Docker/K8s setup (50 pages)
- **[Project Summary](PROJECT_SUMMARY.md)** - Technical details

---

## Development

```bash
# Run tests
make test

# Development mode (hot reload)
make dev-server  # Backend on :8000
make dev-web     # Frontend on :5173

# Build production binary
make build
```

---

## Security & Ethics

**Vero is for authorized penetration testing only.**

- ⚠️ Requires authorization before use
- ✅ HITL gates prevent accidental damage
- ✅ Audit logs for compliance
- ✅ Evidence chain for legal defensibility

**DO NOT use for unauthorized testing. That's illegal.**

---

## License

MIT License - See [LICENSE](LICENSE)

---

## Contributing

Contributions welcome. Open an issue or PR.

---

**Author**: coff0xc
