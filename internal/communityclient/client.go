package communityclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TokenSource interface {
	Token(context.Context) (string, error)
}
type Client struct {
	base   string
	tokens TokenSource
	http   *http.Client
}
type Author struct {
	UserKey string `json:"userKey"`
	Name    string `json:"name"`
	Picture string `json:"picture,omitempty"`
}
type Reply struct {
	ID        string `json:"id"`
	Author    Author `json:"author"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}
type Review struct {
	ID             string  `json:"id"`
	Author         Author  `json:"author"`
	Stars          int     `json:"stars"`
	Content        string  `json:"content"`
	ReleaseVersion string  `json:"releaseVersion"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
	Replies        []Reply `json:"replies"`
	RepliesCursor  string  `json:"repliesCursor,omitempty"`
}
type ReplyPage struct {
	Items      []Reply `json:"items"`
	NextCursor string  `json:"nextCursor,omitempty"`
}
type Summary struct {
	Average         float64 `json:"average"`
	RatingCount     int     `json:"ratingCount"`
	DiscussionCount int     `json:"discussionCount"`
	Distribution    [5]int  `json:"distribution"`
}
type Page struct {
	Items      []Review `json:"items"`
	Summary    Summary  `json:"summary"`
	NextCursor string   `json:"nextCursor,omitempty"`
}
type Draft struct {
	WorkflowID string `json:"workflowId"`
	ReleaseID  string `json:"releaseId"`
	Stars      int    `json:"stars"`
	Content    string `json:"content"`
}
type Problem struct {
	Code        string `json:"code"`
	OperationID string `json:"operationId"`
}

func (p Problem) Error() string         { return p.Code }
func (p Problem) CorrelationID() string { return p.OperationID }

func New(base string, tokens TokenSource, allowHTTP bool) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || u.Host == "" || u.User != nil {
		return nil, errors.New("invalid community URL")
	}
	if u.Scheme != "https" && !(allowHTTP && u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1")) {
		return nil, errors.New("community URL requires HTTPS or explicit loopback HTTP")
	}
	return &Client{base: u.String(), tokens: tokens, http: &http.Client{Timeout: 30 * time.Second}}, nil
}
func path(id string) string { return "/v1/workflows/" + url.PathEscape(id) }
func (c *Client) List(ctx context.Context, id, cursor string) (Page, error) {
	var page Page
	err := c.do(ctx, "GET", path(id)+"/reviews?cursor="+url.QueryEscape(cursor), nil, false, &page)
	return page, err
}
func (c *Client) Mine(ctx context.Context, id string) (*Review, error) {
	var result struct {
		Review *Review `json:"review"`
	}
	err := c.do(ctx, "GET", path(id)+"/reviews/mine", nil, true, &result)
	return result.Review, err
}
func (c *Client) Replies(ctx context.Context, id, parent, cursor string) (ReplyPage, error) {
	var page ReplyPage
	err := c.do(ctx, "GET", path(id)+"/reviews/"+url.PathEscape(parent)+"/replies?cursor="+url.QueryEscape(cursor), nil, false, &page)
	return page, err
}
func (c *Client) Save(ctx context.Context, draft Draft) (Review, error) {
	var review Review
	err := c.do(ctx, "PUT", path(draft.WorkflowID)+"/reviews/mine", map[string]any{"stars": draft.Stars, "content": draft.Content, "releaseId": draft.ReleaseID}, true, &review)
	return review, err
}
func (c *Client) Delete(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", path(id)+"/reviews/mine", nil, true, nil)
}
func (c *Client) Reply(ctx context.Context, id, parent, content string) (Reply, error) {
	var reply Reply
	err := c.do(ctx, "POST", path(id)+"/reviews/"+url.PathEscape(parent)+"/replies", map[string]string{"content": content}, true, &reply)
	return reply, err
}
func (c *Client) DeleteReply(ctx context.Context, id, reply string) error {
	return c.do(ctx, "DELETE", path(id)+"/replies/"+url.PathEscape(reply), nil, true, nil)
}
func (c *Client) do(ctx context.Context, method, path string, body any, authenticated bool, target any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	id := uuid.NewString()
	request, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("X-Request-ID", id)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if authenticated {
		if c.tokens == nil {
			return Problem{Code: "hub.authentication_required", OperationID: id}
		}
		token, err := c.tokens.Token(ctx)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return errors.Join(Problem{Code: "hub.unavailable", OperationID: id}, err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 16<<20+1))
	if err != nil {
		return err
	}
	if len(raw) > 16<<20 {
		return errors.New("community response exceeds limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		p := Problem{Code: "hub.unavailable", OperationID: response.Header.Get("X-Request-ID")}
		_ = json.Unmarshal(raw, &p)
		return p
	}
	if target != nil && response.StatusCode != 204 {
		return json.Unmarshal(raw, target)
	}
	return nil
}
