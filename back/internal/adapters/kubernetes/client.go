package kubernetes

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type Config struct{ Endpoint, TokenPath, CAPath string }
type Client struct {
	baseURL, tokenPath string
	http               *http.Client
}
type Metadata struct {
	Name, Namespace, UID string
	CreationTimestamp    time.Time         `json:"creationTimestamp"`
	Labels               map[string]string `json:"labels"`
}
type Condition struct {
	Type, Status, Reason, Message string
	LastTransitionTime            time.Time `json:"lastTransitionTime"`
}
type Node struct {
	Metadata Metadata `json:"metadata"`
	Status   struct {
		Conditions []Condition                                            `json:"conditions"`
		NodeInfo   struct{ KubeletVersion, OSImage, Architecture string } `json:"nodeInfo"`
		Capacity   map[string]string                                      `json:"capacity"`
	} `json:"status"`
}
type Deployment struct {
	Metadata Metadata `json:"metadata"`
	Spec     struct {
		Replicas int `json:"replicas"`
	} `json:"spec"`
	Status struct{ Replicas, ReadyReplicas, AvailableReplicas, UnavailableReplicas int } `json:"status"`
}
type Pod struct {
	Metadata Metadata `json:"metadata"`
	Status   struct {
		Phase, Reason, Message, PodIP, HostIP string
		ContainerStatuses                     []struct {
			Name         string
			Ready        bool
			RestartCount int
			State        map[string]json.RawMessage
		} `json:"containerStatuses"`
		Conditions []Condition `json:"conditions"`
	} `json:"status"`
}
type Event struct {
	Metadata                      Metadata `json:"metadata"`
	Type, Reason, Message         string
	Count                         int
	FirstTimestamp, LastTimestamp time.Time
	Regarding                     struct{ Kind, Namespace, Name, UID string } `json:"regarding"`
	InvolvedObject                struct{ Kind, Namespace, Name, UID string } `json:"involvedObject"`
}
type Overview struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Nodes       []Node       `json:"nodes"`
	Deployments []Deployment `json:"deployments"`
	ProblemPods []Pod        `json:"problem_pods"`
	Events      []Event      `json:"events"`
}
type list[T any] struct {
	Items []T `json:"items"`
}

var resourceNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)

func New(config Config) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(config.Endpoint, "/"))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("Kubernetes API endpoint must be an absolute HTTPS URL")
	}
	if config.TokenPath == "" {
		return nil, errors.New("Kubernetes service account token path is required")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if config.CAPath != "" {
		pem, readErr := os.ReadFile(config.CAPath)
		if readErr != nil {
			return nil, fmt.Errorf("read Kubernetes CA: %w", readErr)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("Kubernetes CA bundle is invalid")
		}
		tlsConfig.RootCAs = pool
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig, MaxIdleConns: 20, IdleConnTimeout: 60 * time.Second}
	return &Client{baseURL: parsed.String(), tokenPath: config.TokenPath, http: &http.Client{Timeout: 20 * time.Second, Transport: transport}}, nil
}
func (c *Client) Overview(ctx context.Context) (Overview, error) {
	result := Overview{GeneratedAt: time.Now().UTC(), Nodes: []Node{}, Deployments: []Deployment{}, ProblemPods: []Pod{}, Events: []Event{}}
	var nodes list[Node]
	if err := c.get(ctx, "/api/v1/nodes?limit=500", &nodes); err != nil {
		return result, err
	}
	result.Nodes = nodes.Items
	var deployments list[Deployment]
	if err := c.get(ctx, "/apis/apps/v1/deployments?limit=1000", &deployments); err != nil {
		return result, err
	}
	result.Deployments = deployments.Items
	var pods list[Pod]
	if err := c.get(ctx, "/api/v1/pods?limit=1000", &pods); err != nil {
		return result, err
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase != "Running" && pod.Status.Phase != "Succeeded" || hasContainerProblem(pod) {
			result.ProblemPods = append(result.ProblemPods, pod)
		}
	}
	var events list[Event]
	if err := c.get(ctx, "/api/v1/events?limit=200", &events); err != nil {
		return result, err
	}
	for _, event := range events.Items {
		if event.Type == "Warning" {
			result.Events = append(result.Events, event)
		}
	}
	if len(result.Events) > 50 {
		result.Events = result.Events[len(result.Events)-50:]
	}
	return result, nil
}
func hasContainerProblem(pod Pod) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.RestartCount > 0 || !status.Ready {
			return true
		}
	}
	return false
}

func (c *Client) DeploymentAction(ctx context.Context, namespace, name, action string, replicas int) error {
	if !resourceNamePattern.MatchString(namespace) || !resourceNamePattern.MatchString(name) {
		return errors.New("Kubernetes namespace or deployment name is invalid")
	}
	var payload any
	switch action {
	case "scale":
		if replicas < 0 || replicas > 1000 {
			return errors.New("Kubernetes replica count is outside the allowed range")
		}
		payload = map[string]any{"spec": map[string]any{"replicas": replicas}}
	case "restart":
		payload = map[string]any{"spec": map[string]any{"template": map[string]any{"metadata": map[string]any{"annotations": map[string]string{"kubectl.kubernetes.io/restartedAt": time.Now().UTC().Format(time.RFC3339)}}}}}
	default:
		return errors.New("Kubernetes deployment action is unsupported")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	path := "/apis/apps/v1/namespaces/" + url.PathEscape(namespace) + "/deployments/" + url.PathEscape(name)
	return c.request(ctx, http.MethodPatch, path, body, nil)
}
func (c *Client) get(ctx context.Context, path string, destination any) error {
	return c.request(ctx, http.MethodGet, path, nil, destination)
}

func (c *Client) request(ctx context.Context, method, path string, requestBody []byte, destination any) error {
	if c == nil || c.http == nil {
		return errors.New("Kubernetes adapter is not configured")
	}
	token, err := os.ReadFile(c.tokenPath)
	if err != nil {
		return fmt.Errorf("read Kubernetes service account token: %w", err)
	}
	defer zero(token)
	token = bytes.TrimSpace(token)
	if len(token) < 20 {
		return errors.New("Kubernetes service account token is invalid")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+string(token))
	req.Header.Set("Accept", "application/json")
	if len(requestBody) > 0 {
		req.Header.Set("Content-Type", "application/merge-patch+json")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Kubernetes API request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Kubernetes API returned %d", response.StatusCode)
	}
	if destination == nil || len(body) == 0 {
		return nil
	}
	if err = json.Unmarshal(body, destination); err != nil {
		return fmt.Errorf("decode Kubernetes API response: %w", err)
	}
	return nil
}
func zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
