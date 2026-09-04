package registryclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const maximumResponseBytes = int64(8 << 20)

var ErrAuthenticationRequired = errors.New("registry authentication is required")

type TokenSource interface {
	Token(context.Context) (string, error)
}

type Client struct {
	baseURL string
	http    *http.Client
	tokens  TokenSource
}

type Options struct {
	BaseURL           string
	HTTPClient        *http.Client
	Tokens            TokenSource
	AllowLoopbackHTTP bool
}

type PublishRequest struct {
	IdempotencyKey string
	ReleaseVersion string
	Title          string
	Summary        string
	ReleaseNotes   string
	Examples       []Example
	Screenshots    []ScreenshotUpload
	Bundle         io.Reader
	Filename       string
}

type Example struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ScreenshotUpload struct {
	Filename string
	Alt      string
	Content  []byte
}

type Creator struct {
	UserKey     string `json:"userKey"`
	DisplayName string `json:"displayName,omitempty"`
}

type WorkflowRelease struct {
	ReleaseID          string       `json:"releaseId"`
	PublisherNamespace string       `json:"publisherNamespace"`
	WorkflowID         string       `json:"workflowId"`
	ReleaseVersion     string       `json:"releaseVersion"`
	SourceHash         string       `json:"sourceHash"`
	BundleDigest       string       `json:"bundleDigest"`
	Title              string       `json:"title"`
	Summary            string       `json:"summary"`
	ReleaseNotes       string       `json:"releaseNotes,omitempty"`
	Examples           []Example    `json:"examples"`
	Screenshots        []Screenshot `json:"screenshots"`
	Creator            Creator      `json:"creator"`
	Availability       string       `json:"availability"`
	PublishedAt        string       `json:"publishedAt"`
}

type Screenshot struct {
	URL string `json:"url"`
	Alt string `json:"alt"`
}

type SearchResult struct {
	Kind     string          `json:"kind"`
	Workflow WorkflowRelease `json:"workflow"`
}

type SearchPage struct {
	Items      []SearchResult `json:"items"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

type Environment struct {
	YottaVersion    string   `json:"yottaVersion"`
	OperatingSystem string   `json:"operatingSystem"`
	Architecture    string   `json:"architecture"`
	RuntimeProfiles []string `json:"runtimeProfiles,omitempty"`
}

type InstallPlan struct {
	PlanID                     string          `json:"planId"`
	PlanDigest                 string          `json:"planDigest"`
	ResolvedWorkflowRelease    WorkflowRelease `json:"resolvedWorkflowRelease"`
	IncompatibleRequirements   []Problem       `json:"incompatibleRequirements"`
	RuntimeConfigurationNeeded []Problem       `json:"runtimeConfigurationNeeded"`
	DownloadBytes              int64           `json:"downloadBytes"`
	ExpiresAt                  string          `json:"expiresAt"`
}

type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

func (problem Problem) Error() string { return problem.Code }

func New(options Options) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(options.BaseURL), "/"))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("registry base URL must be absolute and contain no credentials")
	}
	if parsed.Scheme != "https" && !(options.AllowLoopbackHTTP && parsed.Scheme == "http" && isLoopback(parsed.Hostname())) {
		return nil, errors.New("registry base URL must use HTTPS or explicitly allowed loopback HTTP")
	}
	client := options.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &Client{baseURL: parsed.String(), http: client, tokens: options.Tokens}, nil
}

func (client *Client) PublishWorkflow(ctx context.Context, input PublishRequest) (WorkflowRelease, error) {
	if client.tokens == nil {
		return WorkflowRelease{}, ErrAuthenticationRequired
	}
	if input.Bundle == nil {
		return WorkflowRelease{}, errors.New("registry publish requires a workflow bundle")
	}
	if size := len(strings.TrimSpace(input.IdempotencyKey)); size < 16 || size > 128 {
		return WorkflowRelease{}, errors.New("registry publish requires a valid idempotency key")
	}
	token, err := client.tokens.Token(ctx)
	if err != nil {
		return WorkflowRelease{}, err
	}
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- writePublication(multipartWriter, writer, input)
	}()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/v1/publications/workflows", reader)
	if err != nil {
		_ = reader.CloseWithError(err)
		return WorkflowRelease{}, err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	request.Header.Set("Idempotency-Key", strings.TrimSpace(input.IdempotencyKey))
	response, err := client.http.Do(request)
	if err != nil {
		_ = reader.CloseWithError(err)
		<-writeDone
		return WorkflowRelease{}, err
	}
	defer response.Body.Close()
	if writeErr := <-writeDone; writeErr != nil {
		return WorkflowRelease{}, writeErr
	}
	var release WorkflowRelease
	if err := decodeResponse(response, &release); err != nil {
		return WorkflowRelease{}, err
	}
	return release, nil
}

func (client *Client) Search(ctx context.Context, text string, limit int) (SearchPage, error) {
	query := url.Values{"q": {strings.TrimSpace(text)}}
	if limit != 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/v1/catalog/search?"+query.Encode(), nil)
	if err != nil {
		return SearchPage{}, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return SearchPage{}, err
	}
	defer response.Body.Close()
	var page SearchPage
	if err := decodeResponse(response, &page); err != nil {
		return SearchPage{}, err
	}
	return page, nil
}

func (client *Client) GetWorkflowRelease(ctx context.Context, releaseID string) (WorkflowRelease, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/v1/workflow-releases/"+url.PathEscape(releaseID), nil)
	if err != nil {
		return WorkflowRelease{}, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return WorkflowRelease{}, err
	}
	defer response.Body.Close()
	var release WorkflowRelease
	if err := decodeResponse(response, &release); err != nil {
		return WorkflowRelease{}, err
	}
	return release, nil
}

func (client *Client) CreateInstallPlan(ctx context.Context, releaseID string, environment Environment) (InstallPlan, error) {
	body, err := json.Marshal(map[string]any{
		"target":      map[string]any{"kind": "workflow", "workflowReleaseId": releaseID},
		"environment": environment, "installedNodePacks": []any{},
	})
	if err != nil {
		return InstallPlan{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/v1/install-plans", strings.NewReader(string(body)))
	if err != nil {
		return InstallPlan{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return InstallPlan{}, err
	}
	defer response.Body.Close()
	var plan InstallPlan
	if err := decodeResponse(response, &plan); err != nil {
		return InstallPlan{}, err
	}
	return plan, nil
}

func (client *Client) DownloadArtifact(ctx context.Context, digest string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/v1/artifacts/"+url.PathEscape(digest), nil)
	if err != nil {
		return nil, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var ignored any
		return nil, decodeResponse(response, &ignored)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, 1<<30+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > 1<<30 {
		return nil, errors.New("registry artifact is too large")
	}
	sum := sha256.Sum256(content)
	if "sha256:"+hex.EncodeToString(sum[:]) != digest {
		return nil, errors.New("registry artifact digest does not match")
	}
	return content, nil
}

func writePublication(multipartWriter *multipart.Writer, pipe *io.PipeWriter, input PublishRequest) error {
	closeWith := func(err error) error {
		if err != nil {
			_ = pipe.CloseWithError(err)
			return err
		}
		if err := multipartWriter.Close(); err != nil {
			_ = pipe.CloseWithError(err)
			return err
		}
		return pipe.Close()
	}
	for name, value := range map[string]string{
		"releaseVersion": input.ReleaseVersion, "title": input.Title, "summary": input.Summary,
		"releaseNotes": input.ReleaseNotes,
	} {
		if err := multipartWriter.WriteField(name, value); err != nil {
			return closeWith(err)
		}
	}
	examples, err := json.Marshal(input.Examples)
	if err != nil {
		return closeWith(err)
	}
	if err := multipartWriter.WriteField("examples", string(examples)); err != nil {
		return closeWith(err)
	}
	alts := make([]string, 0, len(input.Screenshots))
	for _, screenshot := range input.Screenshots {
		alts = append(alts, screenshot.Alt)
	}
	encodedAlts, err := json.Marshal(alts)
	if err != nil {
		return closeWith(err)
	}
	if err := multipartWriter.WriteField("screenshotAlts", string(encodedAlts)); err != nil {
		return closeWith(err)
	}
	filename := strings.TrimSpace(input.Filename)
	if filename == "" {
		filename = "workflow.yotta-workflow"
	}
	part, err := multipartWriter.CreateFormFile("bundle", filename)
	if err != nil {
		return closeWith(err)
	}
	if _, err := io.Copy(part, input.Bundle); err != nil {
		return closeWith(err)
	}
	for _, screenshot := range input.Screenshots {
		part, err := multipartWriter.CreateFormFile("screenshots", screenshot.Filename)
		if err != nil {
			return closeWith(err)
		}
		if _, err := part.Write(screenshot.Content); err != nil {
			return closeWith(err)
		}
	}
	return closeWith(nil)
}

func decodeResponse(response *http.Response, target any) error {
	reader := io.LimitReader(response.Body, maximumResponseBytes+1)
	raw, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if int64(len(raw)) > maximumResponseBytes {
		return errors.New("registry response is too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var problem Problem
		if err := json.Unmarshal(raw, &problem); err != nil || problem.Code == "" {
			return fmt.Errorf("registry request failed with HTTP %d", response.StatusCode)
		}
		return problem
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return errors.New("registry returned an invalid response")
	}
	return nil
}

func isLoopback(host string) bool {
	return strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1"
}
