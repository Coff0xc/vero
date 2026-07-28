package store

import (
	"path/filepath"
	"testing"

	"redcell/internal/core"
)

func TestStoreRoundtrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	id, err := s.StartCampaign("拿下内网 foothold")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveEvent(id, core.Event{Kind: "step", Data: map[string]any{"tool": "fake_scan"}}); err != nil {
		t.Fatal(err)
	}
	g := core.NewAttackGraph()
	g.UpsertNode(&core.Node{ID: "host:x", Type: "host", Label: "x"})
	if err := s.SaveSnapshot(id, g); err != nil {
		t.Fatal(err)
	}
	if err := s.EndCampaign(id, 5, 1, 0); err != nil {
		t.Fatal(err)
	}

	cs, err := s.ListCampaigns(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 {
		t.Fatalf("应有 1 个战役, got %d", len(cs))
	}
	if cs[0].Confirmed != 5 || cs[0].Status != "done" || cs[0].EndedAt == nil {
		t.Fatalf("战役应落盘且回填 KPI: %+v", cs[0])
	}
}
