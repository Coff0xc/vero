// Package store —— SQLite 持久化(modernc.org/sqlite 纯 Go 无 CGO)。
//
// 落盘: 战役(campaign) + 攻击图快照(nodes/edges) + 事件流。
// 用途: 战役可回溯、重启不丢、多战役对比 —— Python 原版是内存 dict, 这是成品化新增层。
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/Coff0xc/vero/internal/core"
)

const schema = `
CREATE TABLE IF NOT EXISTS campaigns (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    goal TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    ended_at INTEGER,
    confirmed INTEGER DEFAULT 0,
    hypothesis INTEGER DEFAULT 0,
    evidence_violations INTEGER DEFAULT 0,
    status TEXT DEFAULT 'running'
);
CREATE TABLE IF NOT EXISTS graph_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    campaign_id INTEGER NOT NULL,
    ts INTEGER NOT NULL,
    nodes TEXT NOT NULL,
    edges TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    campaign_id INTEGER NOT NULL,
    ts INTEGER NOT NULL,
    kind TEXT NOT NULL,
    data TEXT NOT NULL
);`

// Store —— 持久化句柄(database/sql 连接池, 并发安全)。
type Store struct {
	db *sql.DB
}

// Open —— 打开/建库(path 为文件路径; 建表幂等)。
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite 单写者: 串行化 DB 访问, 避免战役写与查询并发的 SQLITE_BUSY
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// StartCampaign —— 开一个战役, 返回 id。
func (s *Store) StartCampaign(goal string) (int64, error) {
	res, err := s.db.Exec("INSERT INTO campaigns(goal, started_at) VALUES(?, ?)", goal, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SaveEvent —— 落一条事件(供战役回放)。
func (s *Store) SaveEvent(campaignID int64, e core.Event) error {
	data, _ := json.Marshal(e.Data)
	_, err := s.db.Exec("INSERT INTO events(campaign_id, ts, kind, data) VALUES(?,?,?,?)",
		campaignID, time.Now().Unix(), e.Kind, string(data))
	return err
}

// SaveSnapshot —— 落一份攻击图快照(nodes/edges 存 JSON)。
func (s *Store) SaveSnapshot(campaignID int64, g *core.AttackGraph) error {
	nodes, _ := json.Marshal(g.Nodes)
	edges, _ := json.Marshal(g.Edges)
	_, err := s.db.Exec("INSERT INTO graph_snapshots(campaign_id, ts, nodes, edges) VALUES(?,?,?,?)",
		campaignID, time.Now().Unix(), string(nodes), string(edges))
	return err
}

// EndCampaign —— 收尾战役, 回填 KPI(confirmed/hypothesis/证据违规)。
func (s *Store) EndCampaign(campaignID int64, confirmed, hypothesis, violations int) error {
	_, err := s.db.Exec(
		"UPDATE campaigns SET ended_at=?, confirmed=?, hypothesis=?, evidence_violations=?, status='done' WHERE id=?",
		time.Now().Unix(), confirmed, hypothesis, violations, campaignID)
	return err
}

// Campaign —— 一行战役记录。
type Campaign struct {
	ID                 int64             `json:"id"`
	Goal               string            `json:"goal"`
	StartedAt          int64             `json:"started_at"`
	EndedAt            *int64            `json:"ended_at"`
	Confirmed          int               `json:"confirmed"`
	Hypothesis         int               `json:"hypothesis"`
	EvidenceViolations int               `json:"evidence_violations"`
	Status             string            `json:"status"`
	Graph              *core.AttackGraph `json:"graph,omitempty"` // 新增：攻击图
}

// ListCampaigns —— 最近战役列表(倒序)。
func (s *Store) ListCampaigns(limit int) ([]Campaign, error) {
	rows, err := s.db.Query(
		`SELECT id, goal, started_at, ended_at, confirmed, hypothesis, evidence_violations, status
         FROM campaigns ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Campaign{}
	for rows.Next() {
		var c Campaign
		if err := rows.Scan(&c.ID, &c.Goal, &c.StartedAt, &c.EndedAt,
			&c.Confirmed, &c.Hypothesis, &c.EvidenceViolations, &c.Status); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCampaign —— 获取单个战役详情（包含最新攻击图快照）。
func (s *Store) GetCampaign(id string) (*Campaign, error) {
	var campaignID int64
	if _, err := fmt.Sscanf(id, "%d", &campaignID); err != nil {
		return nil, fmt.Errorf("无效的战役 ID: %s", id)
	}

	var c Campaign
	err := s.db.QueryRow(
		`SELECT id, goal, started_at, ended_at, confirmed, hypothesis, evidence_violations, status
         FROM campaigns WHERE id = ?`, campaignID).Scan(
		&c.ID, &c.Goal, &c.StartedAt, &c.EndedAt,
		&c.Confirmed, &c.Hypothesis, &c.EvidenceViolations, &c.Status)
	if err != nil {
		return nil, err
	}

	// 加载最新攻击图快照
	var nodesJSON, edgesJSON string
	err = s.db.QueryRow(
		`SELECT nodes, edges FROM graph_snapshots
         WHERE campaign_id = ? ORDER BY ts DESC LIMIT 1`, campaignID).Scan(&nodesJSON, &edgesJSON)

	if err == nil {
		// 解析攻击图
		g := core.NewAttackGraph()
		if err := json.Unmarshal([]byte(nodesJSON), &g.Nodes); err == nil {
			if err := json.Unmarshal([]byte(edgesJSON), &g.Edges); err == nil {
				// 重建 Order
				for id := range g.Nodes {
					g.Order = append(g.Order, id)
				}
				c.Graph = g
			}
		}
	}
	// 如果没有快照，Graph 保持 nil

	return &c, nil
}
