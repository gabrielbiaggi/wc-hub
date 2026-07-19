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
	return c.request(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/actions/runs/%d/%s", fullName, runID, action), nil)
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
	return c.request(ctx, http.MethodGet, path, destination)
}

func (c *Client) request(ctx context.Context, method, path string, destination any) error {
	cleanPath := path
	rawQuery := ""
	if index := strings.Index(path, "?"); index >= 0 {
		cleanPath = path[:index]
		rawQuery = path[index+1:]
	}
	endpoint := &url.URL{Scheme: "https", Host: "api.github.com", Path: cleanPath, RawQuery: rawQuery}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+string(c.token))
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
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
		return fmt.Errorf("GitHub API returned %d", response.StatusCode)
	}
	if destination == nil || len(body) == 0 {
		return nil
	}
	if err = json.Unmarshal(body, destination); err != nil {
		return fmt.Errorf("decode GitHub response: %w", err)
	}
	return nil
}
