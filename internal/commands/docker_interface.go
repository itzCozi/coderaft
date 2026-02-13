package commands

import (
	"context"
	"time"

	"coderaft/internal/docker"
)

type DockerEngine interface {
	Close() error
	IsDockerAvailableWith() error

	PullImage(ref string) error
	ImageExists(ref string) bool
	GetImageDigestInfo(ref string) (digest string, imageID string, err error)
	CommitContainer(containerName, imageTag string) (string, error)
	SaveImage(imageRef, tarPath string) error
	LoadImage(tarPath string) (string, error)

	CreateIsland(name, image, workspaceHost, workspaceIsland string) (string, error)
	CreateIslandWithConfig(name, image, workspaceHost, workspaceIsland string, projectConfig interface{}) (string, error)
	StartIsland(islandID string) error
	StopIsland(islandName string) error
	RemoveIsland(islandName string) error
	IslandExists(islandName string) (bool, error)
	GetIslandStatus(islandName string) (string, error)
	WaitForIsland(islandName string, timeout time.Duration) error
	ListIslands() ([]docker.IslandInfo, error)

	GetContainerID(islandName string) (string, error)
	GetContainerStats(islandName string) (*docker.ContainerStats, error)
	GetUptime(islandName string) (time.Duration, error)
	GetPortMappings(islandName string) ([]string, error)
	GetMounts(islandName string) ([]string, error)
	GetContainerMeta(islandName string) (env map[string]string, workdir, user, restart string, labels map[string]string, capabilities []string, resources map[string]string, network string)
	IsIslandInitialized(islandName string) bool
	IsContainerIdle(islandName string) (bool, error)

	GetAptSources(islandName string) (snapshotURL string, sources []string, release string)
	GetPipRegistries(islandName string) (indexURL string, extra []string)
	GetNodeRegistries(islandName string) (npmReg, yarnReg, pnpmReg string)
	QueryPackagesParallel(islandName string) (aptList, pipList, npmList, yarnList, pnpmList []string)

	SetupCoderaftOnIsland(islandName, projectName string) error
	SetupCoderaftOnIslandWithUpdate(islandName, projectName string) error
	ExecuteSetupCommandsWithOutput(islandName string, commands []string, showOutput bool) error
	ExecCapture(islandName, command string) (stdout string, stderr string, err error)
	RunDockerCommand(args []string) error
	SDKExecFunc() func(ctx context.Context, containerID string, cmd []string, showOutput bool) (string, string, int, error)
}

var _ DockerEngine = (*docker.Client)(nil)
