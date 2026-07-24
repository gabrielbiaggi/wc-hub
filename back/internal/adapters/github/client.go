package github

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`)
var shaPattern = regexp.MustCompile(`^[a-fA-F0-9]{7,64}$`)
var workflowPathPattern = regexp.MustCompile(`^\.github/workflows/[A-Za-z0-9_.-]+\.ya?ml$`)

type Config struct {
	Token        []byte
	Repositories []string
	HTTPClient   *http.Client
}
type Client struct {
	token        []byte
	repositories []string
	http         *http.Client
}
type Repository struct {
	ID              int64     `json:"id"`
	FullName        string    `json:"full_name"`
	Description     string    `json:"description"`
	DefaultBranch   string    `json:"default_branch"`
	HTMLURL         string    `json:"html_url"`
	Private         bool      `json:"private"`
	Archived        bool      `json:"archived"`
	UpdatedAt       time.Time `json:"updated_at"`
	OpenIssuesCount int       `json:"open_issues_count"`
	StargazersCount int       `json:"stargazers_count"`
	ForksCount      int       `json:"forks_count"`
	Size            int       `json:"size"`
	Language        string    `json:"language"`
	Visibility      string    `json:"visibility"`
	Permissions     struct {
		Admin, Maintain, Push, Triage, Pull bool
	} `json:"permissions"`
}
type WorkflowRun struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	DisplayTitle string    `json:"display_title"`
	Event        string    `json:"event"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	HTMLURL      string    `json:"html_url"`
	HeadBranch   string    `json:"head_branch"`
	HeadSHA      string    `json:"head_sha"`
	RunNumber    int       `json:"run_number"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type Release struct {
	ID          int64     `json:"id"`
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	HTMLURL     string    `json:"html_url"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
}
type Commit struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Commit  struct {
		Message string `json:"message"`
		Author  struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Stats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
		Total     int `json:"total"`
	} `json:"stats"`
	Files []CommitFile `json:"files,omitempty"`
}
type CommitFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename,omitempty"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Changes          int    `json:"changes"`
	Patch            string `json:"patch,omitempty"`
}
type Workflow struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	State     string    `json:"state"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type WorkflowFile struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	SHA      string `json:"sha"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}
type Project struct {
	Repository   Repository    `json:"repository"`
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
	Releases     []Release     `json:"releases"`
	Error        string        `json:"error,omitempty"`
}
type Overview struct {
	GeneratedAt time.Time `json:"generated_at"`
	Projects    []Project `json:"projects"`
	Warnings    []string  `json:"warnings"`
}

func New(config Config) (*Client, error) {
	if len(config.Token) < 20 {
		return nil, errors.New("GitHub fine-grained token is required")
	}
	repos := []string{}
	seen := map[string]bool{}
	for _, repo := range config.Repositories {
		repo = strings.TrimSpace(repo)
		if !validRepository(repo) {
			return nil, errors.New("invalid GitHub repository allowlist entry")
		}
		if !seen[repo] {
			seen[repo] = true
			repos = append(repos, repo)
		}
	}
	if len(repos) == 0 {
		return nil, errors.New("GitHub repository allowlist is required")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, MaxIdleConns: 20, IdleConnTimeout: 60 * time.Second}}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || req.URL.Hostname() != "api.github.com" {
			return errors.New("GitHub redirect rejected")
		}
		if len(via) > 4 {
			return errors.New("too many GitHub redirects")
		}
		return nil
	}
	return &Client{token: append([]byte(nil), config.Token...), repositories: repos, http: &copyClient}, nil
}

func validRepository(value string) bool {
	if !repoPattern.MatchString(value) {
		return false
	}
	parts := strings.SplitN(value, "/", 2)
	return parts[0] != "." && parts[0] != ".." && parts[1] != "." && parts[1] != ".."
}

func (c *Client) Overview(ctx context.Context) (Overview, error) {
	result := Overview{GeneratedAt: time.Now().UTC(), Projects: []Project{}, Warnings: []string{}}
	type loaded struct {
		project  Project
		warnings []string
	}
	items := make([]loaded, len(c.repositories))
	semaphore := make(chan struct{}, 6)
	var wait sync.WaitGroup
	for index, fullName := range c.repositories {
		wait.Add(1)
		go func(index int, fullName string) {
			defer wait.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			items[index].project, items[index].warnings = c.loadProject(ctx, fullName)
		}(index, fullName)
	}
	wait.Wait()
	for _, item := range items {
		result.Projects = append(result.Projects, item.project)
		result.Warnings = append(result.Warnings, item.warnings...)
	}
	return result, nil
}

func (c *Client) loadProject(ctx context.Context, fullName string) (Project, []string) {
	project := Project{WorkflowRuns: []WorkflowRun{}, Releases: []Release{}}
	warnings := []string{}
	if err := c.get(ctx, "/repos/"+fullName, &project.Repository); err != nil {
		project.Error = "repository unavailable"
		return project, []string{fullName + " unavailable"}
	}
	var runs struct {
		WorkflowRuns []WorkflowRun `json:"workflow_runs"`
	}
	if err := c.get(ctx, "/repos/"+fullName+"/actions/runs?per_page=20", &runs); err != nil {
		warnings = append(warnings, fullName+" workflows unavailable")
	} else {
		project.WorkflowRuns = runs.WorkflowRuns
	}
	if err := c.get(ctx, "/repos/"+fullName+"/releases?per_page=20", &project.Releases); err != nil {
		warnings = append(warnings, fullName+" releases unavailable")
	}
	return project, warnings
}

func (c *Client) RunAction(ctx context.Context, fullName string, runID int64, action string) error {
	if !c.repositoryAllowed(fullName) {
		return errors.New("GitHub repository is not allowlisted")
	}
	if runID <= 0 {
		return errors.New("GitHub workflow run ID is invalid")
	}
	if action != "rerun" && action != "cancel" {
		return errors.New("unsupported GitHub workflow action")
	}
	return c.requestJSON(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/actions/runs/%d/%s", fullName, runID, action), nil, nil)
}

func (c *Client) Commits(ctx context.Context, fullName string) ([]Commit, error) {
	if !c.repositoryAllowed(fullName) {
		return nil, errors.New("GitHub repository is not allowlisted")
	}
	items := []Commit{}
	err := c.get(ctx, "/repos/"+fullName+"/commits?per_page=40", &items)
	return items, err
}

func (c *Client) Commit(ctx context.Context, fullName, sha string) (Commit, error) {
	if !c.repositoryAllowed(fullName) || !shaPattern.MatchString(sha) {
		return Commit{}, errors.New("GitHub repository or commit is invalid")
	}
	var item Commit
	err := c.get(ctx, "/repos/"+fullName+"/commits/"+sha, &item)
	return item, err
}

func (c *Client) Workflows(ctx context.Context, fullName string) ([]Workflow, error) {
	if !c.repositoryAllowed(fullName) {
		return nil, errors.New("GitHub repository is not allowlisted")
	}
	result := struct {
		Workflows []Workflow `json:"workflows"`
	}{}
	err := c.get(ctx, "/repos/"+fullName+"/actions/workflows?per_page=100", &result)
	return result.Workflows, err
}

func (c *Client) WorkflowAction(ctx context.Context, fullName string, workflowID int64, action, ref string, inputs map[string]string) error {
	if !c.repositoryAllowed(fullName) || workflowID <= 0 {
		return errors.New("GitHub workflow is invalid")
	}
	path := fmt.Sprintf("/repos/%s/actions/workflows/%d", fullName, workflowID)
	switch action {
	case "enable", "disable":
		return c.requestJSON(ctx, http.MethodPut, path+"/"+action, nil, nil)
	case "dispatch":
		if strings.TrimSpace(ref) == "" || len(ref) > 255 || len(inputs) > 20 {
			return errors.New("GitHub workflow dispatch input is invalid")
		}
		return c.requestJSON(ctx, http.MethodPost, path+"/dispatches", map[string]any{"ref": ref, "inputs": inputs}, nil)
	default:
		return errors.New("unsupported GitHub workflow action")
	}
}

func (c *Client) WorkflowFile(ctx context.Context, fullName, path, ref string) (WorkflowFile, error) {
	if !c.repositoryAllowed(fullName) || !workflowPathPattern.MatchString(path) {
		return WorkflowFile{}, errors.New("GitHub workflow path is invalid")
	}
	var item WorkflowFile
	query := ""
	if strings.TrimSpace(ref) != "" {
		query = "?ref=" + url.QueryEscape(ref)
	}
	err := c.get(ctx, "/repos/"+fullName+"/contents/"+path+query, &item)
	return item, err
}

func (c *Client) UpdateWorkflowFile(ctx context.Context, fullName, path, branch, sha, message, contentBase64 string) error {
	if !c.repositoryAllowed(fullName) || !workflowPathPattern.MatchString(path) || !shaPattern.MatchString(sha) {
		return errors.New("GitHub workflow update target is invalid")
	}
	if strings.TrimSpace(branch) == "" || len(branch) > 255 || strings.TrimSpace(message) == "" || len(message) > 500 || len(contentBase64) > 2<<20 {
		return errors.New("GitHub workflow update input is invalid")
	}
	body := map[string]string{"message": message, "content": contentBase64, "sha": sha, "branch": branch}
	return c.requestJSON(ctx, http.MethodPut, "/repos/"+fullName+"/contents/"+path, body, nil)
}

func (c *Client) repositoryAllowed(fullName string) bool {
	for _, allowed := range c.repositories {
		if allowed == fullName {
			return true
		}
	}
	return false
}
func (c *Client) get(ctx context.Context, path string, destination any) error {
	return c.requestJSON(ctx, http.MethodGet, path, nil, destination)
}

func (c *Client) requestJSON(ctx context.Context, method, path string, payload any, destination any) error {
	cleanPath := path
	rawQuery := ""
	if index := strings.Index(path, "?"); index >= 0 {
		cleanPath = path[:index]
		rawQuery = path[index+1:]
	}
	endpoint := &url.URL{Scheme: "https", Host: "api.github.com", Path: cleanPath, RawQuery: rawQuery}
	var requestBody io.Reader
	if payload != nil {
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return marshalErr
		}
		requestBody = strings.NewReader(string(encoded))
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), requestBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+string(c.token))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 12<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("GitHub API error: status %d", response.StatusCode)
	}
	if destination == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, destination)
}

type CreateReleaseInput struct {
	TagName         string `json:"tag_name"`
	TargetCommitish string `json:"target_commitish,omitempty"`
	Name            string `json:"name"`
	Body            string `json:"body"`
	Draft           bool   `json:"draft"`
	Prerelease      bool   `json:"prerelease"`
}

func (c *Client) CreateRelease(ctx context.Context, fullName string, input CreateReleaseInput) (Release, error) {
	if !c.repositoryAllowed(fullName) {
		return Release{}, errors.New("repository not allowlisted")
	}
	var res Release
	err := c.requestJSON(ctx, http.MethodPost, "/repos/"+fullName+"/releases", input, &res)
	return res, err
}

func (c *Client) DeleteRelease(ctx context.Context, fullName string, releaseID int64) error {
	if !c.repositoryAllowed(fullName) {
		return errors.New("repository not allowlisted")
	}
	return c.requestJSON(ctx, http.MethodDelete, fmt.Sprintf("/repos/%s/releases/%d", fullName, releaseID), nil, nil)
}

func (c *Client) GetWorkflowRunLogs(ctx context.Context, fullName string, runID int64) (string, error) {
	if !c.repositoryAllowed(fullName) {
		return "", errors.New("repository not allowlisted")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/actions/runs/%d/logs", fullName, runID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+string(c.token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.Header.Get("Location") != "" {
		return "Download URL dos logs ZIP: " + resp.Header.Get("Location"), nil
	}
	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}
