package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockKubernetesClient implements KubernetesClient for testing
type MockKubernetesClient struct {
	pods        []Pod
	deployments []Deployment
	services    []Service
	configMaps  []ConfigMap
	secrets     []Secret
	namespaces  []Namespace
	nodes       []Node
	events      []Event
	shouldError bool
	errorMsg    string
}

func NewMockKubernetesClient() *MockKubernetesClient {
	return &MockKubernetesClient{
		pods: []Pod{
			{
				Name:      "test-pod-1",
				Namespace: "default",
				Status:    "Running",
				Phase:     "Running",
				IP:        "10.0.0.1",
				Node:      "node-1",
				Containers: []ContainerStatus{
					{Name: "main", Ready: true, Image: "nginx:latest", State: "running"},
				},
				Labels:    map[string]string{"app": "test"},
				CreatedAt: time.Now().Add(-time.Hour),
			},
		},
		deployments: []Deployment{
			{
				Name:              "test-deployment",
				Namespace:         "default",
				Replicas:          3,
				ReadyReplicas:     3,
				AvailableReplicas: 3,
				Labels:            map[string]string{"app": "test"},
				Image:             "nginx:latest",
				CreatedAt:         time.Now().Add(-24 * time.Hour),
			},
		},
		services: []Service{
			{
				Name:      "test-service",
				Namespace: "default",
				Type:      "ClusterIP",
				ClusterIP: "10.96.0.1",
				Ports: []ServicePort{
					{Name: "http", Port: 80, TargetPort: 8080, Protocol: "TCP"},
				},
				Labels: map[string]string{"app": "test"},
			},
		},
		configMaps: []ConfigMap{
			{
				Name:      "test-config",
				Namespace: "default",
				Data:      map[string]string{"key": "value"},
				CreatedAt: time.Now().Add(-time.Hour),
			},
		},
		secrets: []Secret{
			{
				Name:      "test-secret",
				Namespace: "default",
				Type:      "Opaque",
				Keys:      []string{"password"},
				CreatedAt: time.Now().Add(-time.Hour),
			},
		},
		namespaces: []Namespace{
			{Name: "default", Status: "Active", Labels: map[string]string{}},
			{Name: "kube-system", Status: "Active", Labels: map[string]string{}},
		},
		nodes: []Node{
			{Name: "node-1", Status: "Ready"},
		},
		events: []Event{},
	}
}

func (m *MockKubernetesClient) SetError(shouldError bool, msg string) {
	m.shouldError = shouldError
	m.errorMsg = msg
}

func (m *MockKubernetesClient) ListPods(ctx context.Context, namespace string, labelSelector string) ([]Pod, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	var result []Pod
	for _, pod := range m.pods {
		if namespace == "" || pod.Namespace == namespace {
			result = append(result, pod)
		}
	}
	return result, nil
}

func (m *MockKubernetesClient) GetPod(ctx context.Context, namespace, name string) (*Pod, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, pod := range m.pods {
		if pod.Name == name && pod.Namespace == namespace {
			return &pod, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockKubernetesClient) DeletePod(ctx context.Context, namespace, name string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockKubernetesClient) GetPodLogs(ctx context.Context, namespace, name, container string, tailLines int) (string, error) {
	if m.shouldError {
		return "", assert.AnError
	}
	return "sample log output\nline 2\nline 3", nil
}

func (m *MockKubernetesClient) ExecInPod(ctx context.Context, namespace, name, container string, cmd []string) (string, error) {
	if m.shouldError {
		return "", assert.AnError
	}
	return "command output", nil
}

func (m *MockKubernetesClient) ListDeployments(ctx context.Context, namespace string) ([]Deployment, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.deployments, nil
}

func (m *MockKubernetesClient) GetDeployment(ctx context.Context, namespace, name string) (*Deployment, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, d := range m.deployments {
		if d.Name == name && d.Namespace == namespace {
			return &d, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockKubernetesClient) ScaleDeployment(ctx context.Context, namespace, name string, replicas int) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockKubernetesClient) RestartDeployment(ctx context.Context, namespace, name string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockKubernetesClient) ListServices(ctx context.Context, namespace string) ([]Service, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.services, nil
}

func (m *MockKubernetesClient) GetService(ctx context.Context, namespace, name string) (*Service, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, s := range m.services {
		if s.Name == name && s.Namespace == namespace {
			return &s, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockKubernetesClient) ListConfigMaps(ctx context.Context, namespace string) ([]ConfigMap, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.configMaps, nil
}

func (m *MockKubernetesClient) GetConfigMap(ctx context.Context, namespace, name string) (*ConfigMap, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, cm := range m.configMaps {
		if cm.Name == name && cm.Namespace == namespace {
			return &cm, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockKubernetesClient) ListSecrets(ctx context.Context, namespace string) ([]Secret, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.secrets, nil
}

func (m *MockKubernetesClient) ListNamespaces(ctx context.Context) ([]Namespace, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.namespaces, nil
}

func (m *MockKubernetesClient) ListNodes(ctx context.Context) ([]Node, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.nodes, nil
}

func (m *MockKubernetesClient) GetEvents(ctx context.Context, namespace string, fieldSelector string) ([]Event, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.events, nil
}

func (m *MockKubernetesClient) Apply(ctx context.Context, yaml string, namespace string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockKubernetesClient) Delete(ctx context.Context, kind, namespace, name string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

// Tests

func TestDefaultKubernetesConfig(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()

	assert.Equal(t, "default", config.Namespace)
	assert.Equal(t, 60*time.Second, config.Timeout)
	assert.False(t, config.InCluster)
}

func TestNewKubernetesAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	assert.NotNil(t, adapter)

	info := adapter.GetServerInfo()
	assert.Equal(t, "kubernetes", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.NotEmpty(t, info.Description)
}

func TestKubernetesAdapter_ListTools(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)
	// Check for expected tool names
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}
	assert.Contains(t, toolNames, "k8s_list_pods")
	assert.Contains(t, toolNames, "k8s_list_deployments")
	assert.Contains(t, toolNames, "k8s_list_services")
}

func TestKubernetesAdapter_ListPods(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "k8s_list_pods", map[string]interface{}{
		"namespace": "default",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestKubernetesAdapter_GetPod(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "k8s_get_pod", map[string]interface{}{
		"namespace": "default",
		"name":      "test-pod-1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestKubernetesAdapter_GetPodLogs(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "k8s_pod_logs", map[string]interface{}{
		"namespace":  "default",
		"name":       "test-pod-1",
		"container":  "main",
		"tail_lines": 100,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestKubernetesAdapter_ListDeployments(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "k8s_list_deployments", map[string]interface{}{
		"namespace": "default",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestKubernetesAdapter_ScaleDeployment(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "k8s_scale_deployment", map[string]interface{}{
		"namespace": "default",
		"name":      "test-deployment",
		"replicas":  5,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestKubernetesAdapter_ListServices(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "k8s_list_services", map[string]interface{}{
		"namespace": "default",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestKubernetesAdapter_ListNamespaces(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "k8s_list_namespaces", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestKubernetesAdapter_ListNodes(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "k8s_list_nodes", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestKubernetesAdapter_GetEvents(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "k8s_get_events", map[string]interface{}{
		"namespace": "default",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestKubernetesAdapter_Apply(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	yaml := `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: nginx
    image: nginx:latest`

	result, err := adapter.CallTool(ctx, "k8s_apply", map[string]interface{}{
		"yaml":      yaml,
		"namespace": "default",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestKubernetesAdapter_Delete(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "k8s_delete", map[string]interface{}{
		"kind":      "pod",
		"namespace": "default",
		"name":      "test-pod",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestKubernetesAdapter_InvalidTool(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	_, err := adapter.CallTool(ctx, "invalid_tool", map[string]interface{}{})

	assert.Error(t, err)
}

func TestKubernetesAdapter_ErrorHandling(t *testing.T) {
	t.Parallel()
	config := DefaultKubernetesConfig()
	client := NewMockKubernetesClient()
	client.SetError(true, "simulated error")
	adapter := NewKubernetesAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "k8s_list_pods", map[string]interface{}{
		"namespace": "default",
	})

	// The adapter returns errors as IsError in the result, not as Go errors
	assert.NoError(t, err) // no Go-level error
	assert.NotNil(t, result)
	assert.True(t, result.IsError) // but result indicates an error
}

func TestPodTypes(t *testing.T) {
	t.Parallel()
	pod := Pod{
		Name:      "test-pod",
		Namespace: "default",
		Status:    "Running",
		Phase:     "Running",
		IP:        "10.0.0.1",
		Node:      "node-1",
		Containers: []ContainerStatus{
			{Name: "main", Ready: true, Image: "nginx", State: "running"},
		},
		Labels:       map[string]string{"app": "test"},
		RestartCount: 0,
	}

	assert.Equal(t, "test-pod", pod.Name)
	assert.Equal(t, "Running", pod.Status)
	assert.Len(t, pod.Containers, 1)
	assert.True(t, pod.Containers[0].Ready)
}

func TestDeploymentTypes(t *testing.T) {
	t.Parallel()
	deployment := Deployment{
		Name:              "test-deployment",
		Namespace:         "default",
		Replicas:          3,
		ReadyReplicas:     3,
		AvailableReplicas: 3,
		Labels:            map[string]string{"app": "test"},
		Image:             "nginx:latest",
	}

	assert.Equal(t, "test-deployment", deployment.Name)
	assert.Equal(t, 3, deployment.Replicas)
	assert.Equal(t, deployment.Replicas, deployment.ReadyReplicas)
}

func TestServiceTypes(t *testing.T) {
	t.Parallel()
	service := Service{
		Name:      "test-service",
		Namespace: "default",
		Type:      "ClusterIP",
		ClusterIP: "10.96.0.1",
		Ports: []ServicePort{
			{Name: "http", Port: 80, TargetPort: 8080, Protocol: "TCP"},
		},
	}

	assert.Equal(t, "test-service", service.Name)
	assert.Equal(t, "ClusterIP", service.Type)
	assert.Len(t, service.Ports, 1)
	assert.Equal(t, 80, service.Ports[0].Port)
}

func TestConfigMapTypes(t *testing.T) {
	t.Parallel()
	cm := ConfigMap{
		Name:      "test-config",
		Namespace: "default",
		Data:      map[string]string{"key1": "value1", "key2": "value2"},
	}

	assert.Equal(t, "test-config", cm.Name)
	assert.Len(t, cm.Data, 2)
	assert.Equal(t, "value1", cm.Data["key1"])
}

func TestSecretTypes(t *testing.T) {
	t.Parallel()
	secret := Secret{
		Name:      "test-secret",
		Namespace: "default",
		Type:      "Opaque",
		Keys:      []string{"password", "api_key"},
	}

	assert.Equal(t, "test-secret", secret.Name)
	assert.Equal(t, "Opaque", secret.Type)
	assert.Len(t, secret.Keys, 2)
}

func TestNamespaceTypes(t *testing.T) {
	t.Parallel()
	ns := Namespace{
		Name:   "production",
		Status: "Active",
		Labels: map[string]string{"env": "prod"},
	}

	assert.Equal(t, "production", ns.Name)
	assert.Equal(t, "Active", ns.Status)
}

func TestNodeTypes(t *testing.T) {
	t.Parallel()
	node := Node{
		Name:   "node-1",
		Status: "Ready",
	}

	assert.Equal(t, "node-1", node.Name)
	assert.Equal(t, "Ready", node.Status)
}
