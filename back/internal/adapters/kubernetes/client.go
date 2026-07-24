package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type Config struct {
	Endpoint       string
	TokenPath      string
	CAPath         string
	KubeconfigPath string
}

type Client struct {
	clientset kubernetes.Interface
	config    *rest.Config
	source    string
}

type Metadata struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	UID               string            `json:"uid"`
	CreationTimestamp time.Time         `json:"creationTimestamp"`
	Labels            map[string]string `json:"labels"`
}
type Condition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	Reason             string    `json:"reason"`
	Message            string    `json:"message"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
}
type Node struct {
	Metadata Metadata `json:"metadata"`
	Status   struct {
		Conditions []Condition `json:"conditions"`
		NodeInfo   struct {
			KubeletVersion string `json:"kubeletVersion"`
			OSImage        string `json:"osImage"`
			Architecture   string `json:"architecture"`
		} `json:"nodeInfo"`
		Capacity map[string]string `json:"capacity"`
	} `json:"status"`
}
type Deployment struct {
	Metadata Metadata `json:"metadata"`
	Spec     struct {
		Replicas int `json:"replicas"`
	} `json:"spec"`
	Status struct {
		Replicas            int `json:"replicas"`
		ReadyReplicas       int `json:"readyReplicas"`
		AvailableReplicas   int `json:"availableReplicas"`
		UnavailableReplicas int `json:"unavailableReplicas"`
	} `json:"status"`
}
type Pod struct {
	Metadata Metadata `json:"metadata"`
	Status   struct {
		Phase             string `json:"phase"`
		Reason            string `json:"reason"`
		Message           string `json:"message"`
		PodIP             string `json:"podIP"`
		HostIP            string `json:"hostIP"`
		ContainerStatuses []struct {
			Name         string `json:"name"`
			Ready        bool   `json:"ready"`
			RestartCount int    `json:"restartCount"`
		} `json:"containerStatuses"`
		Conditions []Condition `json:"conditions"`
	} `json:"status"`
}
type Event struct {
	Metadata       Metadata  `json:"metadata"`
	Type           string    `json:"type"`
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	Count          int       `json:"count"`
	FirstTimestamp time.Time `json:"firstTimestamp"`
	LastTimestamp  time.Time `json:"lastTimestamp"`
	Regarding      struct {
		Kind      string `json:"kind"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		UID       string `json:"uid"`
	} `json:"regarding"`
	InvolvedObject struct {
		Kind      string `json:"kind"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		UID       string `json:"uid"`
	} `json:"involvedObject"`
}
type IngressItem struct {
	Metadata Metadata `json:"metadata"`
	Rules    []string `json:"rules"`
}
type ConfigMapItem struct {
	Metadata Metadata `json:"metadata"`
	DataKeys []string `json:"dataKeys"`
}
type SecretItem struct {
	Metadata Metadata `json:"metadata"`
	Type     string   `json:"type"`
	DataKeys []string `json:"dataKeys"`
}
type PVCItem struct {
	Metadata Metadata `json:"metadata"`
	Phase    string   `json:"phase"`
	Capacity string   `json:"capacity"`
}
type Overview struct {
	CapturedAt  time.Time       `json:"captured_at"`
	GeneratedAt time.Time       `json:"generated_at"`
	Source      string          `json:"source,omitempty"`
	Nodes       []Node          `json:"nodes"`
	Deployments []Deployment    `json:"deployments"`
	ProblemPods []Pod           `json:"problem_pods"`
	Pods        []Pod           `json:"pods"`
	Events      []Event         `json:"events"`
	Ingresses   []IngressItem   `json:"ingresses"`
	ConfigMaps  []ConfigMapItem `json:"config_maps"`
	Secrets     []SecretItem    `json:"secrets"`
	PVCs        []PVCItem       `json:"pvcs"`
	Warnings    []string        `json:"warnings"`
}

var resourceNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)

// New resolves credentials in the same order used by workloads: in-cluster
// ServiceAccount first, then KUBECONFIG or ~/.kube/config, followed by the
// explicitly configured token files retained for existing installations.
func New(input Config) (*Client, error) {
	attempts := make([]error, 0, 3)
	if config, err := rest.InClusterConfig(); err == nil {
		return newClient(config, "in-cluster ServiceAccount")
	} else {
		attempts = append(attempts, fmt.Errorf("in-cluster configuration: %w", err))
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if path := strings.TrimSpace(input.KubeconfigPath); path != "" {
		rules.ExplicitPath = path
	}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	if config, err := kubeconfig.ClientConfig(); err == nil {
		return newClient(config, "kubeconfig")
	} else {
		attempts = append(attempts, fmt.Errorf("kubeconfig: %w", err))
	}

	if strings.TrimSpace(input.Endpoint) != "" {
		config, err := manualConfig(input)
		if err == nil {
			return newClient(config, "explicit endpoint and token")
		}
		attempts = append(attempts, fmt.Errorf("explicit endpoint: %w", err))
	}
	return nil, fmt.Errorf("Kubernetes configuration failed: %w", errors.Join(attempts...))
}

func manualConfig(input Config) (*rest.Config, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(input.Endpoint), "/"))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("Kubernetes API endpoint must be an absolute HTTPS URL")
	}
	if strings.TrimSpace(input.TokenPath) == "" {
		return nil, errors.New("Kubernetes service account token path is required")
	}
	return &rest.Config{Host: parsed.String(), BearerTokenFile: input.TokenPath, TLSClientConfig: rest.TLSClientConfig{CAFile: input.CAPath}, Timeout: 20 * time.Second}, nil
}

func newClient(config *rest.Config, source string) (*Client, error) {
	if config == nil || strings.TrimSpace(config.Host) == "" {
		return nil, errors.New("Kubernetes client configuration has no API server")
	}
	copy := rest.CopyConfig(config)
	copy.Timeout = 20 * time.Second
	clientset, err := kubernetes.NewForConfig(copy)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes client from %s: %w", source, err)
	}
	return &Client{clientset: clientset, config: copy, source: source}, nil
}

func (c *Client) Overview(ctx context.Context) (Overview, error) {
	if c == nil || c.clientset == nil {
		return Overview{}, errors.New("Kubernetes adapter is not configured")
	}
	now := time.Now().UTC()
	result := Overview{CapturedAt: now, GeneratedAt: now, Source: c.source, Nodes: []Node{}, Deployments: []Deployment{}, Pods: []Pod{}, ProblemPods: []Pod{}, Events: []Event{}, Warnings: []string{}}
	nodes, err := c.listNodes(ctx)
	if err != nil {
		return result, err
	}
	for _, item := range nodes {
		result.Nodes = append(result.Nodes, mapNode(item))
	}
	deployments, err := c.listDeployments(ctx)
	if err != nil {
		return result, err
	}
	for _, item := range deployments {
		result.Deployments = append(result.Deployments, mapDeployment(item))
	}
	pods, err := c.listPods(ctx)
	if err != nil {
		return result, err
	}
	for _, item := range pods {
		mapped := mapPod(item)
		result.Pods = append(result.Pods, mapped)
		if mapped.Status.Phase != "Running" && mapped.Status.Phase != "Succeeded" || hasContainerProblem(mapped) {
			result.ProblemPods = append(result.ProblemPods, mapped)
		}
	}
	events, err := c.listEvents(ctx)
	if err != nil {
		return result, err
	}
	for _, item := range events {
		if item.Type == corev1.EventTypeWarning {
			result.Events = append(result.Events, mapEvent(item))
		}
	}
	sort.Slice(result.Events, func(i, j int) bool { return result.Events[i].LastTimestamp.Before(result.Events[j].LastTimestamp) })
	if len(result.Events) > 50 {
		result.Events = result.Events[len(result.Events)-50:]
	}

	if ingresses, err := c.clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{Limit: 200}); err == nil {
		for _, item := range ingresses.Items {
			ing := IngressItem{Metadata: mapMetadata(&item.ObjectMeta)}
			for _, r := range item.Spec.Rules {
				ing.Rules = append(ing.Rules, r.Host)
			}
			result.Ingresses = append(result.Ingresses, ing)
		}
	}
	if configMaps, err := c.clientset.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{Limit: 200}); err == nil {
		for _, item := range configMaps.Items {
			cm := ConfigMapItem{Metadata: mapMetadata(&item.ObjectMeta), DataKeys: make([]string, 0, len(item.Data))}
			for k := range item.Data {
				cm.DataKeys = append(cm.DataKeys, k)
			}
			result.ConfigMaps = append(result.ConfigMaps, cm)
		}
	}
	if secrets, err := c.clientset.CoreV1().Secrets("").List(ctx, metav1.ListOptions{Limit: 200}); err == nil {
		for _, item := range secrets.Items {
			sec := SecretItem{Metadata: mapMetadata(&item.ObjectMeta), Type: string(item.Type), DataKeys: make([]string, 0, len(item.Data))}
			for k := range item.Data {
				sec.DataKeys = append(sec.DataKeys, k)
			}
			result.Secrets = append(result.Secrets, sec)
		}
	}
	if pvcs, err := c.clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{Limit: 200}); err == nil {
		for _, item := range pvcs.Items {
			pvc := PVCItem{Metadata: mapMetadata(&item.ObjectMeta), Phase: string(item.Status.Phase)}
			if qty, ok := item.Status.Capacity[corev1.ResourceStorage]; ok {
				pvc.Capacity = qty.String()
			}
			result.PVCs = append(result.PVCs, pvc)
		}
	}

	return result, nil
}

func (c *Client) RESTConfig() *rest.Config {
	if c == nil {
		return nil
	}
	return c.config
}

func (c *Client) Clientset() kubernetes.Interface {
	if c == nil {
		return nil
	}
	return c.clientset
}

func (c *Client) PodLogs(ctx context.Context, namespace, pod, container string, tailLines int) (string, error) {
	if !validPodTarget(namespace, pod, container) {
		return "", errors.New("Kubernetes pod target is invalid")
	}
	if tailLines <= 0 || tailLines > 5000 {
		tailLines = 500
	}
	tail := int64(tailLines)
	stream, err := c.clientset.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{Container: container, TailLines: &tail, Timestamps: true}).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("read Kubernetes pod logs: %w", err)
	}
	defer stream.Close()
	contents, err := io.ReadAll(io.LimitReader(stream, 2<<20))
	if err != nil {
		return "", fmt.Errorf("read Kubernetes pod log stream: %w", err)
	}
	return string(contents), nil
}

func (c *Client) Exec(ctx context.Context, namespace, pod, container string, command []string) (string, error) {
	if !validPodTarget(namespace, pod, container) || len(command) == 0 || len(command) > 32 {
		return "", errors.New("Kubernetes exec input is invalid")
	}
	for _, item := range command {
		if strings.TrimSpace(item) == "" || len(item) > 4096 {
			return "", errors.New("Kubernetes command is invalid")
		}
	}
	request := c.clientset.CoreV1().RESTClient().Post().Resource("pods").Namespace(namespace).Name(pod).SubResource("exec").VersionedParams(&corev1.PodExecOptions{Container: container, Command: command, Stdout: true, Stderr: true, TTY: false}, metav1.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(c.config, http.MethodPost, request.URL())
	if err != nil {
		return "", fmt.Errorf("create Kubernetes exec session: %w", err)
	}
	var stdout, stderr bytes.Buffer
	if err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr, Tty: false}); err != nil {
		return "", fmt.Errorf("run Kubernetes exec: %w", err)
	}
	return stdout.String() + stderr.String(), nil
}

func (c *Client) DeploymentAction(ctx context.Context, namespace, name, action string, replicas int) error {
	if !resourceNamePattern.MatchString(namespace) || !resourceNamePattern.MatchString(name) {
		return errors.New("Kubernetes namespace or deployment name is invalid")
	}
	var payload []byte
	switch action {
	case "scale":
		if replicas < 0 || replicas > 1000 {
			return errors.New("Kubernetes replica count is outside the allowed range")
		}
		payload = []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
	case "restart":
		value, err := json.Marshal(map[string]any{"spec": map[string]any{"template": map[string]any{"metadata": map[string]any{"annotations": map[string]string{"kubectl.kubernetes.io/restartedAt": time.Now().UTC().Format(time.RFC3339)}}}}})
		if err != nil {
			return fmt.Errorf("encode Kubernetes restart request: %w", err)
		}
		payload = value
	case "delete":
		if err := c.clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("delete Kubernetes deployment: %w", err)
		}
		return nil
	default:
		return errors.New("Kubernetes deployment action is unsupported")
	}
	if _, err := c.clientset.AppsV1().Deployments(namespace).Patch(ctx, name, types.MergePatchType, payload, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("patch Kubernetes deployment: %w", err)
	}
	return nil
}

func (c *Client) listNodes(ctx context.Context) ([]corev1.Node, error) {
	result := []corev1.Node{}
	continueToken := ""
	for {
		page, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 500, Continue: continueToken})
		if err != nil {
			return nil, fmt.Errorf("list Kubernetes nodes: %w", err)
		}
		result = append(result, page.Items...)
		if page.Continue == "" {
			return result, nil
		}
		continueToken = page.Continue
	}
}

func (c *Client) listDeployments(ctx context.Context) ([]appsv1.Deployment, error) {
	result := []appsv1.Deployment{}
	continueToken := ""
	for {
		page, err := c.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{Limit: 500, Continue: continueToken})
		if err != nil {
			return nil, fmt.Errorf("list Kubernetes deployments: %w", err)
		}
		result = append(result, page.Items...)
		if page.Continue == "" {
			return result, nil
		}
		continueToken = page.Continue
	}
}

func (c *Client) listPods(ctx context.Context) ([]corev1.Pod, error) {
	result := []corev1.Pod{}
	continueToken := ""
	for {
		page, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{Limit: 500, Continue: continueToken})
		if err != nil {
			return nil, fmt.Errorf("list Kubernetes pods: %w", err)
		}
		result = append(result, page.Items...)
		if page.Continue == "" {
			return result, nil
		}
		continueToken = page.Continue
	}
}

func (c *Client) listEvents(ctx context.Context) ([]corev1.Event, error) {
	result := []corev1.Event{}
	continueToken := ""
	for {
		page, err := c.clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{Limit: 500, Continue: continueToken})
		if err != nil {
			return nil, fmt.Errorf("list Kubernetes events: %w", err)
		}
		result = append(result, page.Items...)
		if page.Continue == "" {
			return result, nil
		}
		continueToken = page.Continue
	}
}

func mapMetadata(item metav1.Object) Metadata {
	labels := item.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	return Metadata{Name: item.GetName(), Namespace: item.GetNamespace(), UID: string(item.GetUID()), CreationTimestamp: item.GetCreationTimestamp().Time, Labels: labels}
}
func mapNode(item corev1.Node) Node {
	mapped := Node{Metadata: mapMetadata(&item)}
	for _, condition := range item.Status.Conditions {
		mapped.Status.Conditions = append(mapped.Status.Conditions, Condition{Type: string(condition.Type), Status: string(condition.Status), Reason: condition.Reason, Message: condition.Message, LastTransitionTime: condition.LastTransitionTime.Time})
	}
	mapped.Status.NodeInfo.KubeletVersion, mapped.Status.NodeInfo.OSImage, mapped.Status.NodeInfo.Architecture = item.Status.NodeInfo.KubeletVersion, item.Status.NodeInfo.OSImage, item.Status.NodeInfo.Architecture
	mapped.Status.Capacity = map[string]string{}
	for name, quantity := range item.Status.Capacity {
		mapped.Status.Capacity[string(name)] = quantity.String()
	}
	return mapped
}
func mapDeployment(item appsv1.Deployment) Deployment {
	mapped := Deployment{Metadata: mapMetadata(&item)}
	if item.Spec.Replicas != nil {
		mapped.Spec.Replicas = int(*item.Spec.Replicas)
	}
	mapped.Status.Replicas, mapped.Status.ReadyReplicas, mapped.Status.AvailableReplicas, mapped.Status.UnavailableReplicas = int(item.Status.Replicas), int(item.Status.ReadyReplicas), int(item.Status.AvailableReplicas), int(item.Status.UnavailableReplicas)
	return mapped
}
func mapPod(item corev1.Pod) Pod {
	mapped := Pod{Metadata: mapMetadata(&item)}
	mapped.Status.Phase, mapped.Status.Reason, mapped.Status.Message, mapped.Status.PodIP, mapped.Status.HostIP = string(item.Status.Phase), item.Status.Reason, item.Status.Message, item.Status.PodIP, item.Status.HostIP
	for _, condition := range item.Status.Conditions {
		mapped.Status.Conditions = append(mapped.Status.Conditions, Condition{Type: string(condition.Type), Status: string(condition.Status), Reason: condition.Reason, Message: condition.Message, LastTransitionTime: condition.LastTransitionTime.Time})
	}
	for _, status := range item.Status.ContainerStatuses {
		mapped.Status.ContainerStatuses = append(mapped.Status.ContainerStatuses, struct {
			Name         string `json:"name"`
			Ready        bool   `json:"ready"`
			RestartCount int    `json:"restartCount"`
		}{Name: status.Name, Ready: status.Ready, RestartCount: int(status.RestartCount)})
	}
	return mapped
}
func mapEvent(item corev1.Event) Event {
	mapped := Event{Metadata: mapMetadata(&item), Type: item.Type, Reason: item.Reason, Message: item.Message, Count: int(item.Count), FirstTimestamp: item.FirstTimestamp.Time, LastTimestamp: item.LastTimestamp.Time}
	mapped.InvolvedObject.Kind, mapped.InvolvedObject.Namespace, mapped.InvolvedObject.Name, mapped.InvolvedObject.UID = item.InvolvedObject.Kind, item.InvolvedObject.Namespace, item.InvolvedObject.Name, string(item.InvolvedObject.UID)
	return mapped
}
func hasContainerProblem(pod Pod) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.RestartCount > 0 || !status.Ready {
			return true
		}
	}
	return false
}
func validPodTarget(namespace, pod, container string) bool {
	return resourceNamePattern.MatchString(namespace) && resourceNamePattern.MatchString(pod) && (container == "" || resourceNamePattern.MatchString(container))
}

func (c *Client) ApplyManifest(ctx context.Context, yamlContent string, dryRun bool) ([]string, error) {
	if c == nil || c.clientset == nil || c.config == nil {
		return nil, errors.New("Kubernetes adapter is not configured")
	}
	dyn, err := dynamic.NewForConfig(c.config)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}
	groupResources, err := restmapper.GetAPIGroupResources(c.clientset.Discovery())
	if err != nil {
		return nil, fmt.Errorf("get api group resources: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	docs := strings.Split(yamlContent, "---")
	applied := make([]string, 0, len(docs))

	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		obj := &unstructured.Unstructured{}
		_, gvk, err := decoder.Decode([]byte(doc), nil, obj)
		if err != nil {
			return applied, fmt.Errorf("decode YAML doc: %w", err)
		}
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return applied, fmt.Errorf("map GVK %s: %w", gvk.String(), err)
		}

		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			ns := obj.GetNamespace()
			if ns == "" {
				ns = "default"
			}
			dr = dyn.Resource(mapping.Resource).Namespace(ns)
		} else {
			dr = dyn.Resource(mapping.Resource)
		}

		data, err := json.Marshal(obj)
		if err != nil {
			return applied, fmt.Errorf("marshal object: %w", err)
		}
		force := true
		patchOpts := metav1.PatchOptions{FieldManager: "wc-hub", Force: &force}
		if dryRun {
			patchOpts.DryRun = []string{metav1.DryRunAll}
		}
		_, err = dr.Patch(ctx, obj.GetName(), types.ApplyPatchType, data, patchOpts)
		if err != nil {
			return applied, fmt.Errorf("apply resource %s/%s: %w", obj.GetKind(), obj.GetName(), err)
		}
		applied = append(applied, fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName()))
	}

	return applied, nil
}
