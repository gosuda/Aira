package agent

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// ContainerOptions configures a new container.
type ContainerOptions struct {
	SessionID   SessionID
	Image       string
	VolumeName  string // persistent repo volume
	ProjectDir  string // mount point inside container
	BranchName  string
	Environment map[string]string
	Cmd         []string // command to run
}

// DockerRuntime manages Docker containers for agent sessions.
type DockerRuntime struct {
	client       *client.Client
	imageDefault string
	cpuLimit     string
	memLimit     string
}

func NewDockerRuntime(host, imageDefault, cpuLimit, memLimit string) (*DockerRuntime, error) {
	opts := []client.Opt{
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	}

	c, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("agent.NewDockerRuntime: %w", err)
	}

	return &DockerRuntime{
		client:       c,
		imageDefault: imageDefault,
		cpuLimit:     cpuLimit,
		memLimit:     memLimit,
	}, nil
}

// CreateContainer creates a new container for an agent session.
// Mounts the repo volume, sets environment, applies resource limits.
func (d *DockerRuntime) CreateContainer(ctx context.Context, opts ContainerOptions) (string, error) {
	image := opts.Image
	if image == "" {
		image = d.imageDefault
	}

	env := make([]string, 0, len(opts.Environment)+2)
	env = append(env,
		"AIRA_SESSION_ID="+opts.SessionID.String(),
		"AIRA_BRANCH="+opts.BranchName,
	)
	for k, v := range opts.Environment {
		env = append(env, k+"="+v)
	}

	memLimit, err := parseMemoryLimit(d.memLimit)
	if err != nil {
		return "", fmt.Errorf("agent.DockerRuntime.CreateContainer: %w", err)
	}

	cpuQuota, err := parseCPULimit(d.cpuLimit)
	if err != nil {
		return "", fmt.Errorf("agent.DockerRuntime.CreateContainer: %w", err)
	}

	cfg := &container.Config{
		Image:      image,
		Env:        env,
		Cmd:        opts.Cmd,
		WorkingDir: opts.ProjectDir,
	}

	hostCfg := &container.HostConfig{
		Resources: container.Resources{
			Memory:   memLimit,
			CPUQuota: cpuQuota,
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: opts.VolumeName,
				Target: opts.ProjectDir,
			},
		},
		NetworkMode: "none",
	}

	name := "aira-agent-" + opts.SessionID.String()

	resp, err := d.client.ContainerCreate(ctx, cfg, hostCfg, &network.NetworkingConfig{}, nil, name)
	if err != nil {
		return "", fmt.Errorf("agent.DockerRuntime.CreateContainer: %w", err)
	}

	return resp.ID, nil
}

// StartContainer starts a created container.
func (d *DockerRuntime) StartContainer(ctx context.Context, containerID string) error {
	err := d.client.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("agent.DockerRuntime.StartContainer: %w", err)
	}
	return nil
}

// StopContainer stops a running container with a timeout.
func (d *DockerRuntime) StopContainer(ctx context.Context, containerID string) error {
	timeout := 30 // seconds
	stopOpts := container.StopOptions{Timeout: &timeout}
	err := d.client.ContainerStop(ctx, containerID, stopOpts)
	if err != nil {
		return fmt.Errorf("agent.DockerRuntime.StopContainer: %w", err)
	}
	return nil
}

// RemoveContainer removes a stopped container.
func (d *DockerRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	err := d.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		RemoveVolumes: false,
		Force:         false,
	})
	if err != nil {
		return fmt.Errorf("agent.DockerRuntime.RemoveContainer: %w", err)
	}
	return nil
}

// StreamLogs returns a reader for container stdout/stderr.
func (d *DockerRuntime) StreamLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	reader, err := d.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
	})
	if err != nil {
		return nil, fmt.Errorf("agent.DockerRuntime.StreamLogs: %w", err)
	}
	return reader, nil
}

// WaitContainer waits for container to exit, returns exit code.
func (d *DockerRuntime) WaitContainer(ctx context.Context, containerID string) (int64, error) {
	waitCh, errCh := d.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	select {
	case result := <-waitCh:
		if result.Error != nil {
			return result.StatusCode, fmt.Errorf("agent.DockerRuntime.WaitContainer: %s", result.Error.Message)
		}
		return result.StatusCode, nil
	case err := <-errCh:
		return -1, fmt.Errorf("agent.DockerRuntime.WaitContainer: %w", err)
	case <-ctx.Done():
		return -1, fmt.Errorf("agent.DockerRuntime.WaitContainer: %w", ctx.Err())
	}
}

// ExecInContainer runs a command inside a running container and writes input to its stdin.
// Used by backends to send HITL responses to agent processes.
func (d *DockerRuntime) ExecInContainer(ctx context.Context, containerID string, cmd []string, stdinData []byte) error {
	execCfg := container.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  len(stdinData) > 0,
		AttachStdout: false,
		AttachStderr: false,
	}

	resp, err := d.client.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return fmt.Errorf("agent.DockerRuntime.ExecInContainer: create: %w", err)
	}

	attachResp, err := d.client.ContainerExecAttach(ctx, resp.ID, container.ExecAttachOptions{})
	if err != nil {
		return fmt.Errorf("agent.DockerRuntime.ExecInContainer: attach: %w", err)
	}
	defer attachResp.Close()

	if len(stdinData) > 0 {
		_, err = attachResp.Conn.Write(stdinData)
		if err != nil {
			return fmt.Errorf("agent.DockerRuntime.ExecInContainer: write stdin: %w", err)
		}
		err = attachResp.CloseWrite()
		if err != nil {
			return fmt.Errorf("agent.DockerRuntime.ExecInContainer: close write: %w", err)
		}
	}

	return nil
}

// Client returns the underlying Docker client for advanced operations.
func (d *DockerRuntime) Client() *client.Client {
	return d.client
}

// Close closes the Docker client.
func (d *DockerRuntime) Close() error {
	err := d.client.Close()
	if err != nil {
		return fmt.Errorf("agent.DockerRuntime.Close: %w", err)
	}
	return nil
}
