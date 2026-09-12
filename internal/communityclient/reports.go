package communityclient

import (
	"context"
	"net/url"
)

type ReportDraft struct {
	ID           string `json:"id"`
	WorkflowID   string `json:"workflowId"`
	Reason       string `json:"reason"`
	Description  string `json:"description"`
	OriginalWork string `json:"originalWork"`
}
type Report struct {
	ID           string `json:"id"`
	WorkflowID   string `json:"workflowId"`
	Reason       string `json:"reason"`
	Description  string `json:"description"`
	OriginalWork string `json:"originalWork"`
	State        string `json:"state"`
	Result       string `json:"result"`
	CreatedAt    string `json:"createdAt"`
}
type ReportPage struct {
	Items []Report `json:"items"`
	Total int      `json:"total"`
}

func (c *Client) SubmitReport(ctx context.Context, draft ReportDraft) (Report, error) {
	var result Report
	err := c.do(ctx, "POST", path(draft.WorkflowID)+"/reports", map[string]any{"id": draft.ID, "reason": draft.Reason, "description": draft.Description, "originalWork": draft.OriginalWork}, true, &result)
	return result, err
}
func (c *Client) MyReports(ctx context.Context, workflowID string) (ReportPage, error) {
	var result ReportPage
	err := c.do(ctx, "GET", "/v1/reports/mine?q="+url.QueryEscape(workflowID)+"&limit=100", nil, true, &result)
	return result, err
}
