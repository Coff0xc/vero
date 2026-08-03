package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Coff0xc/vero/internal/report"
)

// handleReportJSON —— 导出 JSON 格式报告。
func (s *Server) handleReportJSON(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")

	// 从数据库加载战役
	campaign, err := s.store.GetCampaign(campaignID)
	if err != nil {
		http.Error(w, "战役不存在", http.StatusNotFound)
		return
	}

	// 检查是否有攻击图
	if campaign.Graph == nil {
		http.Error(w, "战役无攻击图数据", http.StatusNotFound)
		return
	}

	// 生成报告
	startTime := time.Unix(campaign.StartedAt, 0)
	duration := int(time.Since(startTime).Seconds())
	if campaign.EndedAt != nil {
		duration = int(*campaign.EndedAt - campaign.StartedAt)
	}

	rep := report.Generate(
		campaign.Goal, // 使用 Goal 作为目标
		campaign.Graph,
		campaignID,
		duration,
	)

	// 导出 JSON
	data, err := rep.ToJSON()
	if err != nil {
		http.Error(w, "生成报告失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=vero-report-%s.json", campaignID))
	w.Write(data)
}

// handleReportMarkdown —— 导出 Markdown 格式报告。
func (s *Server) handleReportMarkdown(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")

	campaign, err := s.store.GetCampaign(campaignID)
	if err != nil {
		http.Error(w, "战役不存在", http.StatusNotFound)
		return
	}

	if campaign.Graph == nil {
		http.Error(w, "战役无攻击图数据", http.StatusNotFound)
		return
	}

	startTime := time.Unix(campaign.StartedAt, 0)
	duration := int(time.Since(startTime).Seconds())
	if campaign.EndedAt != nil {
		duration = int(*campaign.EndedAt - campaign.StartedAt)
	}

	rep := report.Generate(
		campaign.Goal,
		campaign.Graph,
		campaignID,
		duration,
	)

	md := rep.ToMarkdown()

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=vero-report-%s.md", campaignID))
	w.Write([]byte(md))
}

// handleReportHTML —— 导出 HTML 格式报告。
func (s *Server) handleReportHTML(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")

	campaign, err := s.store.GetCampaign(campaignID)
	if err != nil {
		http.Error(w, "战役不存在", http.StatusNotFound)
		return
	}

	if campaign.Graph == nil {
		http.Error(w, "战役无攻击图数据", http.StatusNotFound)
		return
	}

	startTime := time.Unix(campaign.StartedAt, 0)
	duration := int(time.Since(startTime).Seconds())
	if campaign.EndedAt != nil {
		duration = int(*campaign.EndedAt - campaign.StartedAt)
	}

	rep := report.Generate(
		campaign.Goal,
		campaign.Graph,
		campaignID,
		duration,
	)

	html, err := rep.ToHTML()
	if err != nil {
		http.Error(w, "生成报告失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=vero-report-%s.html", campaignID))
	w.Write([]byte(html))
}

// handleReportsList —— 列出所有历史报告（新增端点）。
func (s *Server) handleReportsList(w http.ResponseWriter, r *http.Request) {
	// TODO: 从数据库查询所有战役
	campaigns, err := s.store.ListCampaigns(100) // 最近 100 个
	if err != nil {
		http.Error(w, "查询失败", http.StatusInternalServerError)
		return
	}

	type reportItem struct {
		CampaignID  string    `json:"campaign_id"`
		Target      string    `json:"target"`
		StartedAt   time.Time `json:"started_at"`
		Duration    int       `json:"duration_sec"`
		FindingCount int      `json:"finding_count"`
		RiskScore   float64   `json:"risk_score"`
	}

	var items []reportItem
	for _, c := range campaigns {
		// 快速统计 (ListCampaigns 不加载攻击图, Graph 恒为 nil —— 走 SQL 轻量统计,
		// 修原版对 nil 遍历 .Nodes 必然空指针崩溃)
		findingCount := s.store.CountFindings(c.ID)

		duration := 0
		startTime := time.Unix(c.StartedAt, 0)
		if c.EndedAt != nil {
			duration = int(*c.EndedAt - c.StartedAt)
		} else {
			duration = int(time.Since(startTime).Seconds())
		}

		items = append(items, reportItem{
			CampaignID:   fmt.Sprintf("%d", c.ID),
			Target:       c.Goal,
			StartedAt:    startTime,
			Duration:     duration,
			FindingCount: findingCount,
			RiskScore:    0, // TODO: 计算风险评分
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"reports": items,
		"total":   len(items),
	})
}
