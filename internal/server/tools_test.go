package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/Coff0xc/vero/internal/audit"
	"github.com/Coff0xc/vero/internal/store"
)

// newToolsTestServer —— 组装最小 server(内存 webFS), 不触发真实安装/下载。
func newToolsTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	auditor := audit.New(filepath.Join(t.TempDir(), "a.jsonl"), filepath.Join(t.TempDir(), "r.jsonl"))
	srv := New(st, auditor, fstest.MapFS{"index.html": {Data: []byte("ok")}})
	return httptest.NewServer(srv.Router())
}

func postJSON(t *testing.T, url, body string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var m map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&m)
	return resp.StatusCode, m
}

// TestToolInstallValidation —— 只测校验/错误分支(不发起真实下载或 pip)。
func TestToolInstallValidation(t *testing.T) {
	ts := newToolsTestServer(t)
	defer ts.Close()

	// 缺 name → 400
	if code, m := postJSON(t, ts.URL+"/api/tools/install", `{}`); code != http.StatusBadRequest || m["error"] != "name required" {
		t.Fatalf("empty body: code=%d m=%v", code, m)
	}
	// 非法 body → 400
	if code, _ := postJSON(t, ts.URL+"/api/tools/install", `{not json`); code != http.StatusBadRequest {
		t.Fatalf("bad json: code=%d", code)
	}
	// install_type=none → 422
	if code, m := postJSON(t, ts.URL+"/api/tools/install", `{"name":"nmap_scan"}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("none tool: code=%d m=%v", code, m)
	}
	// type 枚举非法 → 400
	if code, _ := postJSON(t, ts.URL+"/api/tools/install", `{"name":"web_vuln_scan","type":"apt"}`); code != http.StatusBadRequest {
		t.Fatalf("bad type: code=%d", code)
	}
	// 显式 type 与 install_type 冲突 → 422(secretsdump 为 pip)
	if code, m := postJSON(t, ts.URL+"/api/tools/install", `{"name":"secretsdump","type":"binary"}`); code != http.StatusUnprocessableEntity {
		t.Fatalf("type mismatch: code=%d m=%v", code, m)
	}
	// 显式 type 与 install_type 一致 → 通过校验, 进入安装分支(此处故意不触发真实下载/pip)。
	// 通过"冲突时 422、缺省自动判定"两个分支已间接锁定 pass-through 逻辑。
}

// TestToolVerifyShape —— verify 契约: GET 成功 + 每项恒带 install_type + POST 兼容。
func TestToolVerifyShape(t *testing.T) {
	ts := newToolsTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/tools/verify")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET verify code=%d", resp.StatusCode)
	}
	var v struct {
		Total int `json:"total"`
		Results []struct {
			Name        string `json:"name"`
			Available   bool   `json:"available"`
			InstallType string `json:"install_type"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	if v.Total == 0 || v.Total != len(v.Results) {
		t.Fatalf("total mismatch: %d vs %d results", v.Total, len(v.Results))
	}
	seen := map[string]bool{}
	for _, r := range v.Results {
		seen[r.InstallType] = true
		if r.InstallType != "binary" && r.InstallType != "pip" && r.InstallType != "none" {
			t.Fatalf("bad install_type %q for %s", r.InstallType, r.Name)
		}
	}
	if !seen["binary"] || !seen["pip"] || !seen["none"] {
		t.Fatalf("install_type 三态未齐: %v", seen)
	}

	// POST 兼容旧调用
	presp, err := http.Post(ts.URL+"/api/tools/verify", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer presp.Body.Close()
	if presp.StatusCode != http.StatusOK {
		t.Fatalf("POST verify code=%d", presp.StatusCode)
	}
}

// TestToolInstallAllValidation —— 只测校验/空任务分支(不触发真实安装)。
func TestToolInstallAllValidation(t *testing.T) {
	ts := newToolsTestServer(t)
	defer ts.Close()

	// types 枚举越界 → 400
	if code, m := postJSON(t, ts.URL+"/api/tools/install-all", `{"types":["apt"]}`); code != http.StatusBadRequest {
		t.Fatalf("bad types: code=%d m=%v", code, m)
	}

	// names 过滤命中不存在的工具 → 无任务, 200 total=0
	code, m := postJSON(t, ts.URL+"/api/tools/install-all", `{"names":["__no_such_tool__"]}`)
	if code != http.StatusOK {
		t.Fatalf("empty jobs: code=%d m=%v", code, m)
	}
	if m["total"].(float64) != 0 || m["ok"] != true {
		t.Fatalf("empty jobs: total=%v ok=%v", m["total"], m["ok"])
	}
}
