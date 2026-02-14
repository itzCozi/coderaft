package commands

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"coderaft/internal/security"
	"coderaft/internal/ui"
)

var exportOutput string

var exportCmd = &cobra.Command{
	Use:   "export <project>",
	Short: "Export a self-contained archive of the island (image + config + lock)",
	Long: `Create a portable .tar.gz archive containing:

  - The Docker image snapshot of the running island
  - The project's coderaft.json configuration
  - The coderaft.lock.json (if present)

The archive can be transferred to another machine and restored with
'coderaft restore <project> <archive.tar.gz>'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runExport(args[0])
	},
}

func runExport(projectName string) error {
	cfg, err := configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	proj, ok := cfg.GetProject(projectName)
	if !ok {
		return fmt.Errorf("project '%s' not found", projectName)
	}

	exists, err := dockerClient.IslandExists(proj.IslandName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("island '%s' not found; run 'coderaft up %s' first", proj.IslandName, projectName)
	}

	outPath := exportOutput
	if outPath == "" {
		outPath = filepath.Join(proj.WorkspacePath, fmt.Sprintf("%s-export-%s.tar.gz", projectName, time.Now().Format("20060102-150405")))
	}

	ui.Status("snapping island state...")
	imageTag := fmt.Sprintf("coderaft-export/%s:export-%d", projectName, time.Now().Unix())
	imgID, err := dockerClient.CommitContainer(proj.IslandName, imageTag)
	if err != nil {
		return fmt.Errorf("failed to commit island: %w", err)
	}
	ui.Detail("image", imgID)

	tmpDir, err := os.MkdirTemp("", "coderaft-export-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	imageTar := filepath.Join(tmpDir, "image.tar")
	ui.Status("saving image...")
	if err := dockerClient.SaveImage(imageTag, imageTar); err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	ui.Status("building export archive...")
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}

	var writeErr error
	defer func() {
		if err := outFile.Close(); err != nil && writeErr == nil {
			ui.Warning("failed to close output file: %v", err)
		}
	}()

	gw := gzip.NewWriter(outFile)
	defer func() {
		if err := gw.Close(); err != nil && writeErr == nil {
			ui.Warning("failed to close gzip writer: %v", err)
		}
	}()

	tw := tar.NewWriter(gw)
	defer func() {
		if err := tw.Close(); err != nil && writeErr == nil {
			ui.Warning("failed to close tar writer: %v", err)
		}
	}()

	if err := addFileToTar(tw, imageTar, "image.tar"); err != nil {
		return fmt.Errorf("failed to add image to archive: %w", err)
	}

	configPath := filepath.Join(proj.WorkspacePath, "coderaft.json")
	if _, err := os.Stat(configPath); err == nil {
		if err := addFileToTar(tw, configPath, "coderaft.json"); err != nil {
			return fmt.Errorf("failed to add config to archive: %w", err)
		}
	}

	lockPath := filepath.Join(proj.WorkspacePath, "coderaft.lock.json")
	if _, err := os.Stat(lockPath); err == nil {
		if err := addFileToTar(tw, lockPath, "coderaft.lock.json"); err != nil {
			return fmt.Errorf("failed to add lock file to archive: %w", err)
		}
	}

	manifest := map[string]interface{}{
		"version":     1,
		"project":     projectName,
		"image_tag":   imageTag,
		"island_name": proj.IslandName,
		"exported_at": time.Now().UTC().Format(time.RFC3339),
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := addBytesToTar(tw, manifestData, "manifest.json"); err != nil {
		return fmt.Errorf("failed to add manifest to archive: %w", err)
	}

	_ = dockerClient.RunDockerCommand([]string{"rmi", imageTag})

	ui.Success("exported to %s", security.SanitizePathForError(outPath))
	return nil
}

func addFileToTar(tw *tar.Writer, srcPath, nameInArchive string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name:    nameInArchive,
		Size:    info.Size(),
		Mode:    0644,
		ModTime: info.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

func addBytesToTar(tw *tar.Writer, data []byte, nameInArchive string) error {
	hdr := &tar.Header{
		Name:    nameInArchive,
		Size:    int64(len(data)),
		Mode:    0644,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output path for the archive (default: <workspace>/<project>-export-<timestamp>.tar.gz)")
	rootCmd.AddCommand(exportCmd)
}
