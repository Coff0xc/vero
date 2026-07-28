package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"redcell/internal/audit"
	"redcell/internal/store"
)

// 端到端集成: 对一个可控假靶启动真实战役, 自动批准 HITL, 验证编排跑到 done 并落库。
// 不依赖真实靶场 —— 工具即便失败(环境无 nuclei 等), 编排仍应走到 done。
func TestCampaignEndToEnd(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	auditor := audit.New(filepath.Join(t.TempDir(), "a.jsonl"), filepath.Join(t.TempDir(), "r.jsonl"))
	srv := New(st, auditor, fstest.MapFS{"index.html": {Data: []byte("ok")}})
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	victim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "test-victim")
		w.WriteHeader(http.StatusOK)
	}))
	defer victim.Close()

	resp, err := http.Get(ts.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	done := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var e struct {
				Kind string         `json:"kind"`
				Data map[string]any `json:"data"`
			}
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &e) != nil {
				continue
			}
			if e.Kind == "hitl_request" {
				key, _ := e.Data["key"].(string)
				b, _ := json.Marshal(map[string]any{"key": key, "approved": true})
				http.Post(ts.URL+"/approve", "application/json", strings.NewReader(string(b)))
			}
			if e.Kind == "done" {
				close(done)
				return
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	body, _ := json.Marshal(map[string]any{"target": victim.URL})
	http.Post(ts.URL+"/start", "application/json", strings.NewReader(string(body)))

	select {
	case <-done:
	case <-time.After(120 * time.Second):
		t.Fatal("战役超时未结束")
	}

	cs, err := st.ListCampaigns(5)
	if err != nil || len(cs) == 0 {
		t.Fatalf("战役应落库: %v %+v", err, cs)
	}
}
