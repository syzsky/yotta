package workflow

import (
	"context"
	"errors"
	"github.com/yottaapp/yotta/internal/communityclient"
)

func (s *Service) SubmitWorkflowReport(ctx context.Context, draft communityclient.ReportDraft) (communityclient.Report, error) {
	if s.community == nil {
		return communityclient.Report{}, communityError(errors.New("community not configured"))
	}
	result, err := s.community.SubmitReport(ctx, draft)
	return result, communityError(err)
}
func (s *Service) MyWorkflowReports(ctx context.Context, workflowID string) (communityclient.ReportPage, error) {
	if s.community == nil {
		return communityclient.ReportPage{}, communityError(errors.New("community not configured"))
	}
	result, err := s.community.MyReports(ctx, workflowID)
	return result, communityError(err)
}
