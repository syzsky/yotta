package workflow

import (
	"context"
	"errors"
	"github.com/yottaapp/yotta/internal/apperr"
	"github.com/yottaapp/yotta/internal/communityclient"
	"github.com/yottaapp/yotta/internal/nativeoidc"
)

func WithCommunity(client *communityclient.Client) Option {
	return func(s *Service) { s.community = client }
}
func (s *Service) WorkflowReviews(ctx context.Context, id, cursor string) (communityclient.Page, error) {
	if s.community == nil {
		return communityclient.Page{}, communityError(errors.New("community not configured"))
	}
	page, err := s.community.List(ctx, id, cursor)
	return page, communityError(err)
}
func (s *Service) MyWorkflowReview(ctx context.Context, id string) (*communityclient.Review, error) {
	if s.community == nil {
		return nil, communityError(errors.New("community not configured"))
	}
	review, err := s.community.Mine(ctx, id)
	return review, communityError(err)
}
func (s *Service) WorkflowReviewReplies(ctx context.Context, id, parent, cursor string) (communityclient.ReplyPage, error) {
	if s.community == nil {
		return communityclient.ReplyPage{}, communityError(errors.New("community not configured"))
	}
	page, err := s.community.Replies(ctx, id, parent, cursor)
	return page, communityError(err)
}
func (s *Service) SaveWorkflowReview(ctx context.Context, draft communityclient.Draft) (communityclient.Review, error) {
	if s.community == nil {
		return communityclient.Review{}, communityError(errors.New("community not configured"))
	}
	review, err := s.community.Save(ctx, draft)
	return review, communityError(err)
}
func (s *Service) DeleteWorkflowReview(ctx context.Context, id string) error {
	if s.community == nil {
		return communityError(errors.New("community not configured"))
	}
	return communityError(s.community.Delete(ctx, id))
}
func (s *Service) ReplyToWorkflowReview(ctx context.Context, id, parent, content string) (communityclient.Reply, error) {
	if s.community == nil {
		return communityclient.Reply{}, communityError(errors.New("community not configured"))
	}
	reply, err := s.community.Reply(ctx, id, parent, content)
	return reply, communityError(err)
}
func (s *Service) DeleteWorkflowReply(ctx context.Context, id, reply string) error {
	if s.community == nil {
		return communityError(errors.New("community not configured"))
	}
	return communityError(s.community.DeleteReply(ctx, id, reply))
}
func communityError(cause error) error {
	if cause == nil {
		return nil
	}
	if errors.Is(cause, nativeoidc.ErrAuthenticationRequired) {
		return projectError("workflow.community.authentication_required", apperr.CategoryPolicy, nil, false, cause)
	}
	if errors.Is(cause, context.Canceled) {
		return projectError("workflow.community.cancelled", apperr.CategoryInfrastructure, nil, false, cause)
	}
	var problem communityclient.Problem
	if errors.As(cause, &problem) {
		switch problem.Code {
		case "hub.authentication_required":
			return projectError("workflow.community.authentication_required", apperr.CategoryPolicy, nil, false, cause)
		case "hub.review.invalid", "hub.management.invalid", "hub.management.conflict":
			return projectError("workflow.community.invalid", apperr.CategoryValidation, nil, false, cause)
		case "hub.review.own_rating":
			return projectError("workflow.community.own_rating", apperr.CategoryPolicy, nil, false, cause)
		case "hub.review.not_found", "hub.management.not_found":
			return projectError("workflow.community.not_found", apperr.CategoryDomain, nil, false, cause)
		}
	}
	return projectError("workflow.community.unavailable", apperr.CategoryInfrastructure, nil, true, cause)
}
