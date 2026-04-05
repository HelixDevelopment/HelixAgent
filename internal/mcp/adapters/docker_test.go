package adapters

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockDockerClient implements DockerClient for testing
type MockDockerClient struct {
	containers  []Container
	images      []Image
	networks    []Network
	volumes     []Volume
	shouldError bool
}

func NewMockDockerClient() *MockDockerClient {
	return &MockDockerClient{
		containers: []Container{
			{
				ID:      "abc123def456789012345678901234567890123456789012345678901234",
				Names:   []string{"/test-container"},
				Image:   "nginx:latest",
				ImageID: "sha256:abc123def456789012345678901234567890123456789012345678901234",
				Command: "nginx -g daemon off;",
				Created: time.Now().Add(-time.Hour),
				State:   "running",
				Status:  "Up 1 hour",
				Ports: []Port{
					{PrivatePort: 80, PublicPort: 8080, Type: "tcp"},
				},
				Labels: map[string]string{"app": "web"},
			},
		},
		images: []Image{
			{
				ID:       "sha256:abc123def456789012345678901234567890123456789012345678901234",
				RepoTags: []string{"nginx:latest"},
				Created:  time.Now().Add(-24 * time.Hour),
				Size:     141000000,
			},
		},
		networks: []Network{
			{
				ID:     "net123def456789012345678901234567890123456789012345678901234",
				Name:   "bridge",
				Driver: "bridge",
				Scope:  "local",
			},
		},
		volumes: []Volume{
			{
				Name:       "test-volume",
				Driver:     "local",
				Mountpoint: "/var/lib/docker/volumes/test-volume/_data",
				Scope:      "local",
			},
		},
	}
}

func (m *MockDockerClient) SetError(shouldError bool) {
	m.shouldError = shouldError
}

func (m *MockDockerClient) ListContainers(ctx context.Context, all bool) ([]Container, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.containers, nil
}

func (m *MockDockerClient) GetContainer(ctx context.Context, id string) (*ContainerDetails, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, c := range m.containers {
		// Match by full ID, short ID (12 chars), or container name
		if c.ID == id || (len(c.ID) >= 12 && c.ID[:12] == id) || (len(c.Names) > 0 && c.Names[0] == "/"+id) {
			return &ContainerDetails{
				Container: c,
				Config:    ContainerConfig{Image: c.Image},
				NetworkSettings: NetworkSettings{
					IPAddress: "172.17.0.2",
				},
			}, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockDockerClient) CreateContainer(ctx context.Context, config ContainerConfig) (string, error) {
	if m.shouldError {
		return "", assert.AnError
	}
	return "new-container-id", nil
}

func (m *MockDockerClient) StartContainer(ctx context.Context, id string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockDockerClient) StopContainer(ctx context.Context, id string, timeout int) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockDockerClient) RestartContainer(ctx context.Context, id string, timeout int) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockDockerClient) RemoveContainer(ctx context.Context, id string, force bool) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockDockerClient) ContainerLogs(ctx context.Context, id string, tail int) (io.ReadCloser, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return io.NopCloser(strings.NewReader("log line 1\nlog line 2\nlog line 3")), nil
}

func (m *MockDockerClient) ExecInContainer(ctx context.Context, id string, cmd []string) (string, error) {
	if m.shouldError {
		return "", assert.AnError
	}
	return "command output", nil
}

func (m *MockDockerClient) ListImages(ctx context.Context) ([]Image, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.images, nil
}

func (m *MockDockerClient) PullImage(ctx context.Context, ref string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockDockerClient) RemoveImage(ctx context.Context, id string, force bool) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockDockerClient) BuildImage(ctx context.Context, dockerfile, tag string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockDockerClient) ListNetworks(ctx context.Context) ([]Network, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.networks, nil
}

func (m *MockDockerClient) CreateNetwork(ctx context.Context, name, driver string) (string, error) {
	if m.shouldError {
		return "", assert.AnError
	}
	return "new-network-id", nil
}

func (m *MockDockerClient) RemoveNetwork(ctx context.Context, id string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockDockerClient) ListVolumes(ctx context.Context) ([]Volume, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.volumes, nil
}

func (m *MockDockerClient) CreateVolume(ctx context.Context, name string) (*Volume, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return &Volume{
		Name:       name,
		Driver:     "local",
		Mountpoint: "/var/lib/docker/volumes/" + name + "/_data",
		Scope:      "local",
	}, nil
}

func (m *MockDockerClient) RemoveVolume(ctx context.Context, name string, force bool) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockDockerClient) SystemInfo(ctx context.Context) (*SystemInfo, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return &SystemInfo{
		Containers:        5,
		ContainersRunning: 3,
		Images:            10,
		ServerVersion:     "24.0.0",
		OperatingSystem:   "Docker Desktop",
		NCPU:              8,
		MemTotal:          16000000000,
	}, nil
}

// Tests

func TestDefaultDockerConfig(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()

	assert.Equal(t, "unix:///var/run/docker.sock", config.Host)
	assert.Equal(t, "1.43", config.APIVersion)
	assert.False(t, config.TLSVerify)
	assert.Equal(t, 60*time.Second, config.Timeout)
}

func TestNewDockerAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	assert.NotNil(t, adapter)

	info := adapter.GetServerInfo()
	assert.Equal(t, "docker", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
}

func TestDockerAdapter_ListTools(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}
	assert.Contains(t, toolNames, "docker_list_containers")
	assert.Contains(t, toolNames, "docker_list_images")
}

func TestDockerAdapter_ListContainers(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "docker_list_containers", map[string]interface{}{
		"all": false,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestDockerAdapter_GetContainer(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "docker_get_container", map[string]interface{}{
		"id": "abc123def456",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDockerAdapter_StartContainer(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "docker_start_container", map[string]interface{}{
		"id": "abc123def456",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDockerAdapter_StopContainer(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "docker_stop_container", map[string]interface{}{
		"id":      "abc123",
		"timeout": 10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDockerAdapter_ContainerLogs(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "docker_container_logs", map[string]interface{}{
		"id":   "abc123",
		"tail": 100,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDockerAdapter_ListImages(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "docker_list_images", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDockerAdapter_PullImage(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "docker_pull_image", map[string]interface{}{
		"image": "nginx:latest",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDockerAdapter_ListNetworks(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "docker_list_networks", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDockerAdapter_ListVolumes(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "docker_list_volumes", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDockerAdapter_SystemInfo(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "docker_system_info", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDockerAdapter_InvalidTool(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	adapter := NewDockerAdapter(config, client)

	ctx := context.Background()
	_, err := adapter.CallTool(ctx, "invalid_tool", map[string]interface{}{})

	assert.Error(t, err)
}

func TestDockerAdapter_ErrorHandling(t *testing.T) {
	t.Parallel()
	config := DefaultDockerConfig()
	client := NewMockDockerClient()
	client.SetError(true)
	adapter := NewDockerAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "docker_list_containers", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Type tests

func TestContainerTypes(t *testing.T) {
	t.Parallel()
	container := Container{
		ID:     "abc123def456789012345678901234567890123456789012345678901234",
		Names:  []string{"/web-app"},
		Image:  "nginx:latest",
		State:  "running",
		Status: "Up 2 hours",
		Ports:  []Port{{PrivatePort: 80, PublicPort: 8080, Type: "tcp"}},
		Labels: map[string]string{"env": "production"},
	}

	assert.Equal(t, "abc123def456789012345678901234567890123456789012345678901234", container.ID)
	assert.Contains(t, container.Names, "/web-app")
	assert.Equal(t, "running", container.State)
}

func TestImageTypes(t *testing.T) {
	t.Parallel()
	image := Image{
		ID:       "sha256:abc123def456789012345678901234567890123456789012345678901234",
		RepoTags: []string{"nginx:latest", "nginx:1.21"},
		Size:     141000000,
	}

	assert.Equal(t, "sha256:abc123def456789012345678901234567890123456789012345678901234", image.ID)
	assert.Len(t, image.RepoTags, 2)
}

func TestNetworkTypes(t *testing.T) {
	t.Parallel()
	network := Network{
		ID:     "net123def456789012345678901234567890123456789012345678901234",
		Name:   "my-network",
		Driver: "bridge",
		Scope:  "local",
	}

	assert.Equal(t, "my-network", network.Name)
	assert.Equal(t, "bridge", network.Driver)
}

func TestVolumeTypes(t *testing.T) {
	t.Parallel()
	volume := Volume{
		Name:       "data-volume",
		Driver:     "local",
		Mountpoint: "/var/lib/docker/volumes/data-volume/_data",
		Scope:      "local",
	}

	assert.Equal(t, "data-volume", volume.Name)
	assert.Equal(t, "local", volume.Driver)
}

func TestPortTypes(t *testing.T) {
	t.Parallel()
	port := Port{
		IP:          "0.0.0.0",
		PrivatePort: 80,
		PublicPort:  8080,
		Type:        "tcp",
	}

	assert.Equal(t, 80, port.PrivatePort)
	assert.Equal(t, 8080, port.PublicPort)
	assert.Equal(t, "tcp", port.Type)
}

func TestContainerConfigTypes(t *testing.T) {
	t.Parallel()
	config := ContainerConfig{
		Image:      "nginx:latest",
		Cmd:        []string{"nginx", "-g", "daemon off;"},
		Env:        []string{"NGINX_HOST=localhost"},
		WorkingDir: "/app",
		Labels:     map[string]string{"version": "1.0"},
	}

	assert.Equal(t, "nginx:latest", config.Image)
	assert.Len(t, config.Cmd, 3)
	assert.Len(t, config.Env, 1)
}
