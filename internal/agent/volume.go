package agent

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

const (
	gitImage       = "alpine/git:latest"
	volumeMountDir = "/repo"
)

// VolumeManager handles persistent Docker volumes for repository access.
type VolumeManager struct {
	client *client.Client
}

func NewVolumeManager(dockerClient *client.Client) *VolumeManager {
	return &VolumeManager{client: dockerClient}
}

// EnsureVolume creates a Docker volume if it doesn't exist.
func (vm *VolumeManager) EnsureVolume(ctx context.Context, volumeName string) error {
	_, err := vm.client.VolumeCreate(ctx, volume.CreateOptions{
		Name: volumeName,
	})
	if err != nil {
		return fmt.Errorf("agent.VolumeManager.EnsureVolume: %w", err)
	}
	return nil
}

// CloneRepo clones a git repo into a volume via a temporary container.
// Only runs if the volume is empty (first use).
func (vm *VolumeManager) CloneRepo(ctx context.Context, volumeName, repoURL string) error {
	// Check if already cloned by testing for .git directory.
	exitCode, err := vm.runGitContainer(ctx, volumeName, []string{"rev-parse", "--git-dir"})
	if err != nil {
		return fmt.Errorf("agent.VolumeManager.CloneRepo: check existing: %w", err)
	}
	if exitCode == 0 {
		// Already cloned, nothing to do.
		return nil
	}

	// Clone into the volume.
	exitCode, err = vm.runGitContainer(ctx, volumeName, []string{"clone", repoURL, "."})
	if err != nil {
		return fmt.Errorf("agent.VolumeManager.CloneRepo: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("agent.VolumeManager.CloneRepo: git clone exited with code %d", exitCode)
	}

	return nil
}

// FetchRepo runs git fetch in the volume via a temporary container.
func (vm *VolumeManager) FetchRepo(ctx context.Context, volumeName string) error {
	exitCode, err := vm.runGitContainer(ctx, volumeName, []string{"fetch", "--all", "--prune"})
	if err != nil {
		return fmt.Errorf("agent.VolumeManager.FetchRepo: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("agent.VolumeManager.FetchRepo: git fetch exited with code %d", exitCode)
	}
	return nil
}

// CreateBranch creates an isolated git worktree for the session branch.
// Using git worktree (instead of git checkout) is critical for concurrency safety:
// multiple agents can work on the same repo volume simultaneously without corrupting
// the shared working tree. Each worktree gets its own directory under /repo/.worktrees/.
func (vm *VolumeManager) CreateBranch(ctx context.Context, volumeName, branchName, baseBranch string) error {
	worktreePath := volumeMountDir + "/.worktrees/" + branchName
	exitCode, err := vm.runGitContainer(ctx, volumeName, []string{
		"worktree", "add", "-b", branchName, worktreePath, baseBranch,
	})
	if err != nil {
		return fmt.Errorf("agent.VolumeManager.CreateBranch: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("agent.VolumeManager.CreateBranch: git worktree add exited with code %d", exitCode)
	}
	return nil
}

// RemoveWorktree cleans up a git worktree after a session completes.
func (vm *VolumeManager) RemoveWorktree(ctx context.Context, volumeName, branchName string) error {
	worktreePath := volumeMountDir + "/.worktrees/" + branchName
	exitCode, err := vm.runGitContainer(ctx, volumeName, []string{"worktree", "remove", "--force", worktreePath})
	if err != nil {
		return fmt.Errorf("agent.VolumeManager.RemoveWorktree: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("agent.VolumeManager.RemoveWorktree: git worktree remove exited with code %d", exitCode)
	}
	return nil
}

// RemoveVolume removes a Docker volume.
func (vm *VolumeManager) RemoveVolume(ctx context.Context, volumeName string) error {
	err := vm.client.VolumeRemove(ctx, volumeName, true)
	if err != nil {
		return fmt.Errorf("agent.VolumeManager.RemoveVolume: %w", err)
	}
	return nil
}

// runGitContainer runs a git command inside a temporary container that mounts the volume.
// Returns the container exit code.
func (vm *VolumeManager) runGitContainer(ctx context.Context, volumeName string, gitArgs []string) (int64, error) {
	cfg := &container.Config{
		Image:      gitImage,
		Cmd:        gitArgs,
		WorkingDir: volumeMountDir,
	}

	hostCfg := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: volumeMountDir,
			},
		},
		AutoRemove: true,
	}

	resp, err := vm.client.ContainerCreate(ctx, cfg, hostCfg, &network.NetworkingConfig{}, nil, "")
	if err != nil {
		return -1, fmt.Errorf("create git container: %w", err)
	}

	err = vm.client.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		// Clean up the created container since AutoRemove only applies to running containers.
		_ = vm.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return -1, fmt.Errorf("start git container: %w", err)
	}

	waitCh, errCh := vm.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case result := <-waitCh:
		if result.Error != nil {
			return result.StatusCode, fmt.Errorf("git container error: %s", result.Error.Message)
		}
		return result.StatusCode, nil
	case waitErr := <-errCh:
		return -1, fmt.Errorf("wait git container: %w", waitErr)
	case <-ctx.Done():
		return -1, fmt.Errorf("wait git container: %w", ctx.Err())
	}
}
