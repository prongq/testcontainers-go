package testcontainers

import (
	"context"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"

	"github.com/testcontainers/testcontainers-go/internal/testcontainersdocker"
)

// DockerClient is a wrapper around the docker client that is used by testcontainers-go.
// It implements the SystemAPIClient interface in order to cache the docker info and reuse it.
type DockerClient struct {
	*client.Client // client is embedded into our own client
}

var (
	dockerInfo     types.Info // dockerInfo stores the docker info to be reused in the Info method
	dockerInfoOnce sync.Once
)

// implements SystemAPIClient interface
var _ client.SystemAPIClient = &DockerClient{}

// Events returns a channel to listen to events that happen to the docker daemon.
func (c *DockerClient) Events(ctx context.Context, options types.EventsOptions) (<-chan events.Message, <-chan error) {
	return c.Client.Events(ctx, options)
}

// Info returns information about the docker server. The result of Info is cached
// and reused every time Info is called.
// It will also print out the docker server info, and the resolved Docker paths, to the default logger.
func (c *DockerClient) Info(ctx context.Context) (types.Info, error) {
	var err error
	dockerInfoOnce.Do(func() {
		dockerInfo, err = c.Client.Info(ctx)
		if err != nil {
			// reset the state of the sync.Once so that the next call to Info will try again
			dockerInfoOnce = sync.Once{}
			return
		}

		infoMessage := `%v - Connected to docker: 
  Server Version: %v
  API Version: %v
  Operating System: %v
  Total Memory: %v MB
  Resolved Docker Host: %s
  Resolved Docker Socket Path: %s
`

		Logger.Printf(infoMessage, packagePath,
			dockerInfo.ServerVersion, c.Client.ClientVersion(),
			dockerInfo.OperatingSystem, dockerInfo.MemTotal/1024/1024,
			testcontainersdocker.ExtractDockerHost(ctx),
			testcontainersdocker.ExtractDockerSocket(ctx),
		)
	})

	return dockerInfo, err
}

// RegistryLogin logs into a Docker registry.
func (c *DockerClient) RegistryLogin(ctx context.Context, auth registry.AuthConfig) (registry.AuthenticateOKBody, error) {
	return c.Client.RegistryLogin(ctx, auth)
}

// DiskUsage returns the disk usage of all images.
func (c *DockerClient) DiskUsage(ctx context.Context, options types.DiskUsageOptions) (types.DiskUsage, error) {
	return c.Client.DiskUsage(ctx, options)
}

// Ping pings the docker server.
func (c *DockerClient) Ping(ctx context.Context) (types.Ping, error) {
	return c.Client.Ping(ctx)
}

// Deprecated: Use NewDockerClientWithOpts instead.
func NewDockerClient() (*client.Client, error) {
	cli, err := NewDockerClientWithOpts(context.Background())
	if err != nil {
		return nil, err
	}

	return cli.Client, nil
}

func NewDockerClientWithOpts(ctx context.Context, opt ...client.Opt) (*DockerClient, error) {
	dockerClient, err := testcontainersdocker.NewClient(ctx, opt...)
	if err != nil {
		return nil, err
	}

	tcClient := DockerClient{
		Client: dockerClient,
	}

	if _, err = tcClient.Info(ctx); err != nil {
		// Fallback to environment, including the original options
		if len(opt) == 0 {
			opt = []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
		}

		dockerClient, err := client.NewClientWithOpts(opt...)
		if err != nil {
			return nil, err
		}

		tcClient.Client = dockerClient
	}
	defer tcClient.Close()

	return &tcClient, nil
}
