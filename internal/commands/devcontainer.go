package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type devContainer struct {
	Name              string            `json:"name,omitempty"`
	Image             string            `json:"image,omitempty"`
	WorkspaceFolder   string            `json:"workspaceFolder,omitempty"`
	ContainerEnv      map[string]string `json:"containerEnv,omitempty"`
	PostCreateCommand string            `json:"postCreateCommand,omitempty"`
	ForwardPorts      []int             `json:"forwardPorts,omitempty"`
	Mounts            []string          `json:"mounts,omitempty"`
}

var devcontainerCmd = &cobra.Command{
	Use:   "devcontainer",
	Short: "Generate VS Code devcontainer.json from coderaft.json",
	Args:  cobra.NoArgs,
}

var devcontainerGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate .devcontainer/devcontainer.json for the current project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get cwd: %w", err)
		}

		pcfg, err := configManager.LoadProjectConfig(cwd)
		if err != nil {
			return fmt.Errorf("failed to load coderaft project config: %w", err)
		}
		if pcfg == nil {
			return fmt.Errorf("no coderaft project config found in %s (coderaft.json | coderaft.project.json | .coderaft.json)", cwd)
		}

		dc := devContainer{
			Name:            pcfg.Name,
			Image:           firstNonEmpty(pcfg.BaseImage, "ubuntu:latest"),
			WorkspaceFolder: firstNonEmpty(pcfg.WorkingDir, "/island"),
			ContainerEnv:    map[string]string{},
		}

		for k, v := range pcfg.Environment {
			dc.ContainerEnv[k] = v
		}

		for _, p := range pcfg.Ports {
			part := strings.TrimSpace(p)
			if part == "" {
				continue
			}

			if i := strings.Index(part, ":"); i != -1 {
				part = part[i+1:]
			}
			if i := strings.Index(part, "/"); i != -1 {
				part = part[:i]
			}
			if part != "" {
				if portNum, err := strconv.Atoi(part); err == nil {
					dc.ForwardPorts = append(dc.ForwardPorts, portNum)
				}
			}
		}

		dc.Mounts = append(dc.Mounts, "source=${localWorkspaceFolder},target="+dc.WorkspaceFolder+",type=bind,consistency=cached")

		for _, vol := range pcfg.Volumes {
			s := strings.TrimSpace(vol)
			if s == "" || !strings.Contains(s, ":") {
				continue
			}
			parts := strings.SplitN(s, ":", 2)
			host := parts[0]
			target := parts[1]

			if strings.HasPrefix(host, "~") {
				host = "${env:HOME}" + strings.TrimPrefix(host, "~")
			}
			dc.Mounts = append(dc.Mounts, fmt.Sprintf("source=%s,target=%s,type=bind", host, target))
		}

		if len(pcfg.SetupCommands) > 0 {

			dc.PostCreateCommand = strings.Join(pcfg.SetupCommands, " && ")
		}

		outDir := filepath.Join(cwd, ".devcontainer")
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("failed to create .devcontainer dir: %w", err)
		}
		outPath := filepath.Join(outDir, "devcontainer.json")
		data, err := json.MarshalIndent(dc, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal devcontainer.json: %w", err)
		}
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outPath, err)
		}

		fmt.Printf("Wrote %s\n", outPath)
		fmt.Println("Open the folder in VS Code and use 'Reopen in Container' to start a consistent dev island.")
		return nil
	},
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func init() {
	devcontainerCmd.AddCommand(devcontainerGenerateCmd)
}
