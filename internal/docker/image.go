package docker

import (
	"context"
	"fmt"
	"os"

	"coderaft/internal/ui"
)

func (c *Client) PullImage(ref string) error {
	ctx := context.Background()

	exists, err := c.sdk.imageExists(ctx, ref)
	if err == nil && exists {
		return nil
	}

	ui.Status("pulling image %s...", ref)
	if err := c.sdk.pullImage(ctx, ref); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", ref, err)
	}
	return nil
}

func (c *Client) ImageExists(ref string) bool {
	ctx := context.Background()
	exists, err := c.sdk.imageExists(ctx, ref)
	return err == nil && exists
}

func (c *Client) CommitContainer(containerName, imageTag string) (string, error) {
	ctx := context.Background()
	id, err := c.sdk.commitContainer(ctx, containerName, imageTag)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (c *Client) SaveImage(imageRef, tarPath string) error {
	f, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("failed to create tar file: %w", err)
	}
	defer f.Close()

	ctx := context.Background()
	return c.sdk.saveImage(ctx, imageRef, f)
}

func (c *Client) LoadImage(tarPath string) (string, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return "", fmt.Errorf("failed to open tar file: %w", err)
	}
	defer f.Close()

	ctx := context.Background()
	return c.sdk.loadImage(ctx, f)
}

func (c *Client) GetImageDigestInfo(ref string) (string, string, error) {
	ctx := context.Background()

	imgInspect, _, err := c.sdk.cli.ImageInspectWithRaw(ctx, ref)
	if err == nil {
		digest := ""
		if len(imgInspect.RepoDigests) > 0 {
			digest = imgInspect.RepoDigests[0]
		}
		return digest, imgInspect.ID, nil
	}

	containerInspect, err := c.sdk.containerInspect(ctx, ref)
	if err != nil {
		return "", "", fmt.Errorf("inspect failed: %w", err)
	}
	imageID := containerInspect.Image
	if imageID == "" {
		return "", "", nil
	}

	imgInspect, _, err = c.sdk.cli.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		return "", imageID, nil
	}
	digest := ""
	if len(imgInspect.RepoDigests) > 0 {
		digest = imgInspect.RepoDigests[0]
	}
	return digest, imgInspect.ID, nil
}
