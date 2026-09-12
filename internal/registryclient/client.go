package registryclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
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
	Listing        Listing
	IdempotencyKey string
	ReleaseVersion string
	Title          string
	Summary        string
	ReleaseNotes   string
	Examples       []Example
	Bundle         io.Reader
	Filename       string
}

type Example struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Creator struct {
	QualityAuthor bool   `json:"qualityAuthor,omitempty"`
	Picture       string `json:"picture,omitempty"`
	UserKey       string `json:"userKey"`
	DisplayName   string `json:"displayName,omitempty"`
}

type WorkflowRelease struct {
	Official           bool                `json:"official"`
	Recommended        bool                `json:"recommended"`
	DownloadCount      int64               `json:"downloadCount"`
	Dependencies       []DependencySummary `json:"dependencies"`
	Listing            Listing             `json:"listing"`
	Facts              BundleFacts         `json:"facts"`
	ReleaseID          string              `json:"releaseId"`
	PublisherNamespace string              `json:"publisherNamespace"`
	WorkflowID         string              `json:"workflowId"`
	ReleaseVersion     string              `json:"releaseVersion"`
	SourceHash         string              `json:"sourceHash"`
	BundleDigest       string              `json:"bundleDigest"`
	Title              string              `json:"title"`
	Summary            string              `json:"summary"`
	ReleaseNotes       string              `json:"releaseNotes,omitempty"`
	Examples           []Example           `json:"examples"`
	Creator            Creator             `json:"creator"`
	Availability       string              `json:"availability"`
	PublishedAt        string              `json:"publishedAt"`
}

type SearchResult struct {
	Kind     string          `json:"kind"`
	Workflow WorkflowRelease `json:"workflow,omitempty"`
	NodePack NodePackRelease `json:"nodePack,omitempty"`
	Node     NodeProjection  `json:"node,omitempty"`
}

type SearchPage struct {
	Facets     Facets         `json:"facets"`
	Items      []SearchResult `json:"items"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

type Environment struct {
	YottaVersion    string   `json:"yottaVersion"`
	OperatingSystem string   `json:"operatingSystem"`
	Architecture    string   `json:"architecture"`
	RuntimeProfiles []string `json:"runtimeProfiles,omitempty"`
}

type NodeRef struct {
	NodeTypeID     string `json:"nodeTypeId"`
	NodeVersion    string `json:"nodeVersion"`
	SemanticDigest string `json:"semanticDigest"`
}

type PortProjection struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type NodeProjection struct {
	NodeRef            NodeRef          `json:"nodeRef"`
	PackageID          string           `json:"packageId"`
	PackageVersion     string           `json:"packageVersion"`
	ContributionDigest string           `json:"contributionDigest"`
	Name               string           `json:"name"`
	Summary            string           `json:"summary"`
	Category           string           `json:"category,omitempty"`
	Inputs             []PortProjection `json:"inputs"`
	Outputs            []PortProjection `json:"outputs"`
}

type RuntimeVariant struct {
	VariantID        string   `json:"variantId"`
	RuntimeFamily    string   `json:"runtimeFamily"`
	RuntimeProfile   string   `json:"runtimeProfile"`
	OperatingSystems []string `json:"operatingSystems"`
	Architectures    []string `json:"architectures"`
	ManifestDigest   string   `json:"manifestDigest"`
	ArtifactDigest   string   `json:"artifactDigest"`
	DownloadBytes    int64    `json:"downloadBytes"`
}

type NodePackRelease struct {
	ReleaseID          string           `json:"releaseId"`
	PublisherNamespace string           `json:"publisherNamespace"`
	PackageID          string           `json:"packageId"`
	PackageVersion     string           `json:"packageVersion"`
	ContributionDigest string           `json:"contributionDigest"`
	Title              string           `json:"title"`
	Summary            string           `json:"summary"`
	ReleaseNotes       string           `json:"releaseNotes,omitempty"`
	Listing            Listing          `json:"listing"`
	Nodes              []NodeProjection `json:"nodes"`
	Variants           []RuntimeVariant `json:"variants"`
	Creator            Creator          `json:"creator"`
	Availability       string           `json:"availability"`
	PublishedAt        string           `json:"publishedAt"`
}

type InstalledNodePack struct {
	PackageID      string `json:"packageId"`
	PackageVersion string `json:"packageVersion"`
	ManifestDigest string `json:"manifestDigest"`
}

type PlannedNodePack struct {
	ReleaseID          string         `json:"releaseId"`
	PackageID          string         `json:"packageId"`
	PackageVersion     string         `json:"packageVersion"`
	ContributionDigest string         `json:"contributionDigest"`
	ManifestDigest     string         `json:"manifestDigest"`
	Title              string         `json:"title"`
	Reason             string         `json:"reason"`
	SelectedVariant    RuntimeVariant `json:"selectedVariant"`
	DownloadBytes      int64          `json:"downloadBytes"`
}

type UserAction struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type InstallPlan struct {
	PlanID                     string              `json:"planId"`
	PlanDigest                 string              `json:"planDigest"`
	ResolvedWorkflowRelease    WorkflowRelease     `json:"resolvedWorkflowRelease"`
	NodePacksToInstall         []PlannedNodePack   `json:"nodePacksToInstall"`
	NodePacksAlreadySatisfied  []InstalledNodePack `json:"nodePacksAlreadySatisfied"`
	Updates                    []PlannedNodePack   `json:"updates"`
	IncompatibleRequirements   []UserAction        `json:"incompatibleRequirements"`
	RuntimeConfigurationNeeded []UserAction        `json:"runtimeConfigurationNeeded"`
	DownloadBytes              int64               `json:"downloadBytes"`
	ExpiresAt                  string              `json:"expiresAt"`
}

type Problem struct {
	OperationID string `json:"operationId,omitempty"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Status      int    `json:"status"`
	Code        string `json:"code"`
	Detail      string `json:"detail"`
}

func (problem Problem) Error() string         { return problem.Code }
func (problem Problem) CorrelationID() string { return problem.OperationID }

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
		client = &http.Client{Timeout: 60 * time.Second}
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
	request.Header.Set("X-Request-ID", uuid.NewString())
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

func (client *Client) GetNodePackRelease(ctx context.Context, releaseID string) (NodePackRelease, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/v1/node-pack-releases/"+url.PathEscape(releaseID), nil)
	if err != nil {
		return NodePackRelease{}, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return NodePackRelease{}, err
	}
	defer response.Body.Close()
	var release NodePackRelease
	if err := decodeResponse(response, &release); err != nil {
		return NodePackRelease{}, err
	}
	normalizeNodePackRelease(&release)
	return release, nil
}

func (client *Client) CreateInstallPlan(ctx context.Context, releaseID string, environment Environment) (InstallPlan, error) {
	return client.createInstallPlan(ctx, map[string]any{"kind": "workflow", "workflowReleaseId": releaseID}, environment, []InstalledNodePack{})
}

func (client *Client) CreateNodePackInstallPlan(ctx context.Context, releaseID string, environment Environment, installed []InstalledNodePack) (InstallPlan, error) {
	if installed == nil {
		installed = []InstalledNodePack{}
	}
	return client.createInstallPlan(ctx, map[string]any{"kind": "node-pack", "nodePackReleaseId": releaseID}, environment, installed)
}

func (client *Client) CreateNodeInstallPlan(ctx context.Context, nodeTypeID, preferredVersion string, environment Environment, installed []InstalledNodePack) (InstallPlan, error) {
	if installed == nil {
		installed = []InstalledNodePack{}
	}
	target := map[string]any{"kind": "node", "nodeTypeId": nodeTypeID}
	if strings.TrimSpace(preferredVersion) != "" {
		target["preferredVersion"] = strings.TrimSpace(preferredVersion)
	}
	return client.createInstallPlan(ctx, target, environment, installed)
}

func (client *Client) createInstallPlan(ctx context.Context, target map[string]any, environment Environment, installed []InstalledNodePack) (InstallPlan, error) {
	body, err := json.Marshal(map[string]any{
		"target": target, "environment": environment, "installedNodePacks": installed,
	})
	if err != nil {
		return InstallPlan{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/v1/install-plans", strings.NewReader(string(body)))
	if err != nil {
		return InstallPlan{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if err := client.authorizeDelivery(request); err != nil {
		return InstallPlan{}, err
	}
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
	return client.downloadVerified(ctx, "/v1/artifacts/"+url.PathEscape(digest), digest)
}
func (client *Client) DownloadWorkflow(ctx context.Context, releaseID, digest string) ([]byte, error) {
	return client.downloadVerified(ctx, "/v1/workflow-releases/"+url.PathEscape(releaseID)+"/download", digest)
}
func (client *Client) downloadVerified(ctx context.Context, route, digest string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+route, nil)
	if err != nil {
		return nil, err
	}
	if err := client.authorizeDelivery(request); err != nil {
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
	listing, err := json.Marshal(input.Listing)
	if err != nil {
		return closeWith(err)
	}
	if err := multipartWriter.WriteField("listing", string(listing)); err != nil {
		return closeWith(err)
	}
	if err := multipartWriter.WriteField("examples", string(examples)); err != nil {
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
