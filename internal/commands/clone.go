package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"coderaft/internal/config"
	"coderaft/internal/ui"
)

var (
	cloneForce        bool
	cloneTemplate     string
	cloneNoSetup      bool
	cloneBranch       string
	cloneDepth        int
	cloneName         string
	cloneSparse       bool
	cloneNoSubmodules bool
	cloneSingleBranch bool
)

var cloneCmd = &cobra.Command{
	Use:   "clone <repo>",
	Short: "Clone a repository and create a ready-to-code island",
	Long: `Clone a Git repository and automatically set up a coderaft island.

This command:
  1. Clones the repository to your coderaft workspace
  2. Auto-detects the project's tech stack (Python, Node.js, Go, Rust, Java, Ruby, PHP, etc.)
  3. Creates an isolated Docker island with the right tools
  4. Runs setup commands → ready to code in seconds

Supported URL formats:
  - Full HTTPS URL: https://github.com/user/repo
  - Full SSH URL: git@github.com:user/repo.git
  - Shorthand: user/repo (assumes GitHub)
  - With host: github.com/user/repo
  - GitLab/Bitbucket: https://gitlab.com/user/repo
  - Browser URLs: https://github.com/user/repo/tree/main (branch auto-detected)
  - PR/Issue URLs: https://github.com/user/repo/pull/123

Features:
  - Automatic submodule initialization
  - Branch detection from browser URLs
  - Sparse checkout support for large repositories
  - Retry logic for network issues

Examples:
  coderaft clone user/repo                          # GitHub shorthand
  coderaft clone https://github.com/user/repo
  coderaft clone git@github.com:user/repo.git
  coderaft clone github.com/user/repo
  coderaft clone https://github.com/user/repo/tree/develop    # Auto-detects branch
  coderaft clone https://github.com/user/repo --template nodejs
  coderaft clone https://github.com/user/repo --branch develop
  coderaft clone https://github.com/user/repo --depth 1       # Shallow clone
  coderaft clone user/repo --name my-project
  coderaft clone user/repo --sparse                 # Sparse checkout (large repos)
  coderaft clone user/repo --single-branch          # Clone only one branch
  coderaft clone user/repo --no-submodules          # Skip submodule init`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoInput := args[0]
		startTime := time.Now()

		// Check if git is available
		if _, err := exec.LookPath("git"); err != nil {
			return fmt.Errorf("git is not installed or not in PATH. Please install git first")
		}

		// Extract branch from URL before normalization (if user pasted browser URL like /tree/main)
		urlBranch := extractBranchFromURL(repoInput)

		// Normalize the repository URL (supports shorthand like user/repo)
		repoURL, err := normalizeRepoURL(repoInput)
		if err != nil {
			return fmt.Errorf("invalid repository: %w", err)
		}

		// Use branch from URL if not explicitly specified via --branch flag
		effectiveBranch := cloneBranch
		if effectiveBranch == "" && urlBranch != "" {
			effectiveBranch = urlBranch
			ui.Status("detected branch from URL: %s", effectiveBranch)
		}

		// Extract project name from repo URL or use --name flag
		var projectName string
		if cloneName != "" {
			projectName = cloneName
		} else {
			projectName, err = extractProjectName(repoURL)
			if err != nil {
				return fmt.Errorf("failed to parse repository URL: %w", err)
			}
		}

		if err := validateProjectName(projectName); err != nil {
			return fmt.Errorf("invalid project name '%s': %w", projectName, err)
		}

		cfg, err := configManager.Load()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		if _, exists := cfg.GetProject(projectName); exists && !cloneForce {
			return fmt.Errorf("project '%s' already exists. Use --force to overwrite", projectName)
		}

		workspacePath, err := getWorkspacePath(projectName)
		if err != nil {
			return err
		}

		// Check if directory exists
		if _, err := os.Stat(workspacePath); err == nil {
			if !cloneForce {
				return fmt.Errorf("directory '%s' already exists. Use --force to overwrite", workspacePath)
			}
			ui.Status("removing existing directory '%s'...", workspacePath)
			if err := os.RemoveAll(workspacePath); err != nil {
				return fmt.Errorf("failed to remove existing directory: %w", err)
			}
		}

		// Step 1: Clone the repository
		ui.Step(1, 4, "cloning repository")
		if err := gitClone(repoURL, workspacePath, effectiveBranch); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}

		// Step 2: Detect project stack
		ui.Step(2, 4, "detecting project stack")
		detectedTemplate := cloneTemplate
		if detectedTemplate == "" {
			detectedTemplate = detectProjectStack(workspacePath)
			if detectedTemplate != "" {
				ui.Status("detected stack: %s", detectedTemplate)
			} else {
				ui.Status("no specific stack detected, using base environment")
			}
		} else {
			ui.Status("using specified template: %s", detectedTemplate)
		}

		// Detect monorepo structure
		monorepoInfo := detectMonorepo(workspacePath)
		if monorepoInfo.IsMonorepo {
			ui.Status("detected monorepo: %s", monorepoInfo.Type)
			if len(monorepoInfo.WorkspaceDirs) > 0 {
				ui.Status("workspace directories: %s", strings.Join(monorepoInfo.WorkspaceDirs, ", "))
			}
		}

		// Load or create project config
		var projectConfig *config.ProjectConfig

		// Check for existing coderaft.json in cloned repo
		if existingConfig, err := configManager.LoadProjectConfig(workspacePath); err == nil && existingConfig != nil {
			ui.Info("found existing coderaft.json in repository")
			projectConfig = existingConfig
			// Override name to match our project name
			projectConfig.Name = projectName
		} else if detectedTemplate != "" {
			// Create config from detected/specified template
			projectConfig, err = configManager.CreateProjectConfigFromTemplate(detectedTemplate, projectName)
			if err != nil {
				ui.Warning("failed to create config from template: %v", err)
				projectConfig = configManager.GetDefaultProjectConfig(projectName)
			}

			// Add auto-detected setup commands based on project files
			additionalCommands := detectSetupCommands(workspacePath, detectedTemplate)
			if len(additionalCommands) > 0 {
				projectConfig.SetupCommands = append(projectConfig.SetupCommands, additionalCommands...)
			}
		} else {
			projectConfig = configManager.GetDefaultProjectConfig(projectName)
		}

		// Save the generated config
		if err := configManager.SaveProjectConfig(workspacePath, projectConfig); err != nil {
			ui.Warning("failed to save coderaft.json: %v", err)
		} else {
			ui.Status("generated coderaft.json")
		}

		if cloneNoSetup {
			ui.Success("repository cloned to '%s'", workspacePath)
			ui.Detail("workspace", workspacePath)
			ui.Info("run 'coderaft up' in the project directory to start the island")
			return nil
		}

		// Step 3: Create and start the island
		ui.Step(3, 4, "creating island")

		IslandName := fmt.Sprintf("coderaft_%s", projectName)
		baseImage := cfg.GetEffectiveBaseImage(&config.Project{
			Name:      projectName,
			BaseImage: "buildpack-deps:bookworm",
		}, projectConfig)

		workspaceIsland := "/island"
		if projectConfig != nil && projectConfig.WorkingDir != "" {
			workspaceIsland = projectConfig.WorkingDir
		}

		// Pull the image
		if err := dockerClient.PullImage(baseImage); err != nil {
			return fmt.Errorf("failed to pull base image: %w", err)
		}

		// Handle force flag - remove existing island
		if cloneForce {
			exists, err := dockerClient.IslandExists(IslandName)
			if err != nil {
				return fmt.Errorf("failed to check island existence: %w", err)
			}
			if exists {
				ui.Status("removing existing island '%s'...", IslandName)
				dockerClient.StopIsland(IslandName)
				if err := dockerClient.RemoveIsland(IslandName); err != nil {
					return fmt.Errorf("failed to remove existing island: %w", err)
				}
			}
		}

		// Build config map
		var configMap map[string]interface{}
		if projectConfig != nil {
			configData, err := json.Marshal(projectConfig)
			if err != nil {
				return fmt.Errorf("failed to marshal project config: %w", err)
			}
			if err := json.Unmarshal(configData, &configMap); err != nil {
				return fmt.Errorf("failed to convert project config: %w", err)
			}
		}

		if cfg.Settings != nil && cfg.Settings.AutoStopOnExit {
			if configMap == nil {
				configMap = map[string]interface{}{}
			}
			if _, ok := configMap["restart"]; !ok {
				configMap["restart"] = "no"
			}
		}

		// Use optimized setup
		optimizedSetup := NewOptimizedSetup(dockerClient, configManager)
		if err := optimizedSetup.FastUp(projectConfig, projectName, IslandName, baseImage, workspacePath, workspaceIsland, configMap); err != nil {
			return fmt.Errorf("failed to start island: %w", err)
		}

		// Step 4: Finalize
		ui.Step(4, 4, "finalizing setup")

		// Save project to global config
		project := &config.Project{
			Name:          projectName,
			IslandName:    IslandName,
			BaseImage:     baseImage,
			WorkspacePath: workspacePath,
			Status:        "running",
		}
		cfg.MergeProjectConfig(project, projectConfig)
		cfg.AddProject(project)
		if err := configManager.Save(cfg); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		// Generate lock file
		_ = WriteLockFileForIsland(IslandName, projectName, workspacePath, baseImage, "")

		elapsed := time.Since(startTime).Round(time.Second)
		ui.Blank()
		ui.Success("ready to code in %s!", elapsed)
		ui.Detail("project", projectName)
		ui.Detail("workspace", workspacePath)
		ui.Detail("island", IslandName)
		if detectedTemplate != "" {
			ui.Detail("stack", detectedTemplate)
		}
		if monorepoInfo.IsMonorepo {
			ui.Detail("monorepo", monorepoInfo.Type)
		}

		ui.Blank()
		ui.Info("Next steps:")
		ui.Info("  coderaft shell %s       # open interactive shell", projectName)
		ui.Info("  coderaft run %s <cmd>   # run a command", projectName)

		// Show monorepo-specific hints
		if hint := getMonorepoSetupHint(monorepoInfo); hint != "" {
			ui.Blank()
			ui.Info("Monorepo tip: %s", hint)
		}

		return nil
	},
}

func init() {
	cloneCmd.Flags().BoolVarP(&cloneForce, "force", "f", false, "Force clone, overwriting existing project")
	cloneCmd.Flags().StringVarP(&cloneTemplate, "template", "t", "", "Use specific template instead of auto-detection (python, nodejs, go, rust, java, ruby, php, web)")
	cloneCmd.Flags().BoolVar(&cloneNoSetup, "no-setup", false, "Clone only, don't create the island")
	cloneCmd.Flags().StringVarP(&cloneBranch, "branch", "b", "", "Branch to clone")
	cloneCmd.Flags().IntVar(&cloneDepth, "depth", 0, "Create a shallow clone with specified depth")
	cloneCmd.Flags().StringVarP(&cloneName, "name", "n", "", "Override the project name (defaults to repository name)")
	cloneCmd.Flags().BoolVar(&cloneSparse, "sparse", false, "Use sparse checkout (only checkout root files initially) - useful for large repos")
	cloneCmd.Flags().BoolVar(&cloneNoSubmodules, "no-submodules", false, "Don't initialize submodules")
	cloneCmd.Flags().BoolVar(&cloneSingleBranch, "single-branch", false, "Clone only the specified branch (reduces clone size)")
}

// normalizeRepoURL converts various repository formats to a full Git URL
// Supports:
//   - Full HTTPS: https://github.com/user/repo
//   - Full SSH: git@github.com:user/repo.git
//   - Shorthand: user/repo (assumes GitHub)
//   - Host prefix: github.com/user/repo, gitlab.com/user/repo
//   - Protocol prefix: github:user/repo, gh:user/repo
//   - Browser URLs: https://github.com/user/repo/tree/main, /blob/main/file.go
//   - PRs/Issues: https://github.com/user/repo/pull/123
func normalizeRepoURL(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("repository URL cannot be empty")
	}

	// Check for browser-specific patterns BEFORE stripping trailing slashes
	// This catches cases like https://github.com/user/repo.git/
	needsCleaning := needsBrowserURLCleaning(input)

	// Remove trailing slashes
	input = strings.TrimRight(input, "/")

	// Already a full URL (https:// or http://)
	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		// Only clean browser URLs if they have browser-specific paths
		if needsCleaning {
			return cleanBrowserURL(input), nil
		}
		return input, nil
	}

	// SSH URL format (git@host:user/repo.git)
	if strings.HasPrefix(input, "git@") {
		return input, nil
	}

	// Git protocol
	if strings.HasPrefix(input, "git://") {
		return input, nil
	}

	// Handle protocol shortcuts (github:user/repo, gh:user/repo, gitlab:user/repo)
	protocolShortcuts := map[string]string{
		"github:":    "https://github.com/",
		"gh:":        "https://github.com/",
		"gitlab:":    "https://gitlab.com/",
		"gl:":        "https://gitlab.com/",
		"bitbucket:": "https://bitbucket.org/",
		"bb:":        "https://bitbucket.org/",
	}

	for prefix, urlBase := range protocolShortcuts {
		if strings.HasPrefix(input, prefix) {
			path := strings.TrimPrefix(input, prefix)
			path = strings.TrimSuffix(path, ".git")
			if !isValidRepoPath(path) {
				return "", fmt.Errorf("invalid repository path: %s", path)
			}
			return urlBase + path, nil
		}
	}

	// Handle host prefix (github.com/user/repo, gitlab.com/user/repo)
	knownHosts := []string{
		"github.com/",
		"gitlab.com/",
		"bitbucket.org/",
		"codeberg.org/",
		"gitea.com/",
		"sr.ht/",
		"git.sr.ht/",
	}

	for _, host := range knownHosts {
		if strings.HasPrefix(input, host) {
			fullURL := "https://" + input
			if needsBrowserURLCleaning(fullURL) {
				return cleanBrowserURL(fullURL), nil
			}
			return fullURL, nil
		}
	}

	// Handle shorthand format (user/repo or org/repo)
	// More permissive pattern supporting:
	// - Usernames: letters, numbers, hyphens, underscores
	// - Repo names: letters, numbers, hyphens, underscores, dots
	// - Optional .git suffix
	shorthandPattern := regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9_]*\/[a-zA-Z0-9][-a-zA-Z0-9_.]*(?:\.git)?$`)
	if shorthandPattern.MatchString(input) {
		// Default to GitHub for shorthand
		return "https://github.com/" + strings.TrimSuffix(input, ".git"), nil
	}

	// If it looks like a URL but without protocol
	if strings.Contains(input, "/") && strings.Contains(input, ".") {
		// Try adding https:// and see if it parses
		testURL := "https://" + input
		if _, err := url.Parse(testURL); err == nil {
			return cleanBrowserURL(testURL), nil
		}
	}

	return "", fmt.Errorf("unable to parse repository URL '%s'. Use format: user/repo, https://github.com/user/repo, or git@github.com:user/repo.git", input)
}

// needsBrowserURLCleaning checks if a URL contains browser-specific paths that should be cleaned
func needsBrowserURLCleaning(inputURL string) bool {
	// Check for .git/ before any other processing (trailing slash on .git)
	if strings.Contains(inputURL, ".git/") {
		return true
	}

	parsed, err := url.Parse(inputURL)
	if err != nil {
		return false
	}

	// Browser-specific path segments
	browserSegments := []string{
		"/tree/", "/blob/", "/commit/", "/commits/",
		"/pull/", "/pulls", "/issues/", "/issues",
		"/compare/", "/releases", "/tags", "/branches",
		"/actions", "/wiki", "/settings", "/security",
		"/network", "/graphs", "/projects", "/discussions",
		"/stargazers", "/watchers", "/forks", "/archive/",
		"/raw/", "/-/",
	}

	path := parsed.Path
	for _, segment := range browserSegments {
		if strings.Contains(path, segment) {
			return true
		}
	}

	// Also check for query params and fragments that indicate browser usage
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return true
	}

	return false
}

// cleanBrowserURL strips GitHub/GitLab browser-specific paths from URLs
// Also strips .git suffix, trailing slashes, and query params
// Examples:
//   - https://github.com/user/repo/tree/main -> https://github.com/user/repo
//   - https://github.com/user/repo/blob/main/file.go -> https://github.com/user/repo
//   - https://github.com/user/repo/pull/123 -> https://github.com/user/repo
//   - https://github.com/user/repo.git -> https://github.com/user/repo
func cleanBrowserURL(inputURL string) string {
	parsed, err := url.Parse(inputURL)
	if err != nil {
		return inputURL
	}

	// Remove trailing slashes and .git suffix from path
	path := strings.TrimRight(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	// Split path into parts: /user/repo/tree/main/... -> ["", "user", "repo", "tree", "main", ...]
	parts := strings.Split(path, "/")

	// We need at least 3 parts: ["", "user", "repo"]
	if len(parts) < 3 {
		// Not enough parts, just clean trailing slash and .git
		parsed.Path = path
		parsed.RawQuery = ""
		parsed.Fragment = ""
		parsed.RawFragment = ""
		return parsed.String()
	}

	// Browser-specific path segments that indicate we should truncate
	browserSegments := map[string]bool{
		"tree":        true, // /tree/branch
		"blob":        true, // /blob/branch/file
		"commit":      true, // /commit/sha
		"commits":     true, // /commits/branch
		"pull":        true, // /pull/123
		"pulls":       true, // /pulls
		"issues":      true, // /issues/123
		"issue":       true, // /issue/123
		"compare":     true, // /compare/branch1...branch2
		"releases":    true, // /releases
		"tags":        true, // /tags
		"branches":    true, // /branches
		"actions":     true, // /actions
		"wiki":        true, // /wiki
		"settings":    true, // /settings
		"security":    true, // /security
		"network":     true, // /network
		"graphs":      true, // /graphs
		"projects":    true, // /projects
		"discussions": true, // /discussions
		"stargazers":  true, // /stargazers
		"watchers":    true, // /watchers
		"forks":       true, // /forks
		"archive":     true, // /archive/branch.zip
		"raw":         true, // /raw/branch/file (GitLab)
		"-":           true, // GitLab: /-/tree, /-/blob, etc.
	}

	// Check if we have browser segments that need stripping
	hasBrowserSegment := false
	browserSegmentIndex := -1
	for i := 3; i < len(parts); i++ {
		if browserSegments[parts[i]] {
			hasBrowserSegment = true
			browserSegmentIndex = i
			break
		}
	}

	// Build clean path
	var cleanParts []string
	if hasBrowserSegment {
		// Truncate at the browser segment
		cleanParts = parts[:browserSegmentIndex]
	} else if strings.Contains(parsed.Host, "github.com") {
		// For GitHub, always truncate to owner/repo (GitHub doesn't have nested groups)
		cleanParts = []string{"", parts[1], parts[2]}
	} else {
		// For GitLab and others, preserve nested groups/subgroups
		// Keep all path parts since there's no browser segment
		cleanParts = parts
	}

	// Reconstruct URL
	parsed.Path = strings.Join(cleanParts, "/")
	parsed.RawQuery = "" // Remove query parameters
	parsed.Fragment = "" // Remove fragments
	parsed.RawFragment = ""

	return parsed.String()
}

// extractBranchFromURL extracts branch name from GitHub/GitLab browser URLs
// Returns empty string if no branch can be extracted
func extractBranchFromURL(inputURL string) string {
	parsed, err := url.Parse(inputURL)
	if err != nil {
		return ""
	}

	parts := strings.Split(parsed.Path, "/")
	if len(parts) < 5 {
		return ""
	}

	// Look for /tree/branch or /blob/branch patterns
	for i, part := range parts {
		if (part == "tree" || part == "blob" || part == "commits") && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}

// isValidRepoPath checks if a path looks like a valid repo path (owner/repo)
func isValidRepoPath(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return false
	}
	// Check owner and repo are non-empty
	for _, part := range parts[:2] {
		if strings.TrimSpace(part) == "" {
			return false
		}
	}
	return true
}

// extractProjectName extracts the project name from a Git repository URL
func extractProjectName(repoURL string) (string, error) {
	// Handle SSH URLs (git@github.com:user/repo.git)
	if strings.HasPrefix(repoURL, "git@") {
		// git@github.com:user/repo.git -> repo
		parts := strings.Split(repoURL, ":")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid SSH URL format")
		}
		path := parts[1]
		return cleanRepoName(path), nil
	}

	// Handle HTTP(S) URLs
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}

	path := strings.TrimPrefix(parsed.Path, "/")
	return cleanRepoName(path), nil
}

// cleanRepoName extracts a clean project name from a repo path
func cleanRepoName(path string) string {
	// Remove .git suffix
	path = strings.TrimSuffix(path, ".git")
	// Get the last component (repo name)
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

// gitClone clones a repository to the specified path with retry logic and submodule support
func gitClone(repoURL, destPath, branch string) error {
	// Handle sparse checkout separately (requires different git workflow)
	if cloneSparse {
		return gitCloneSparse(repoURL, destPath, branch)
	}

	args := []string{"clone", "--progress"}

	if branch != "" {
		args = append(args, "-b", branch)
	}

	if cloneSingleBranch || cloneDepth > 0 {
		args = append(args, "--single-branch")
	}

	if cloneDepth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", cloneDepth))
	}

	// Handle submodules
	if !cloneNoSubmodules {
		args = append(args, "--recurse-submodules")
		if cloneDepth > 0 {
			args = append(args, "--shallow-submodules")
		}
	}

	args = append(args, repoURL, destPath)

	// Retry logic for transient network errors
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		cmd := exec.Command("git", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		lastErr = cmd.Run()
		if lastErr == nil {
			// Initialize any submodules that weren't cloned (in case --recurse-submodules partially failed)
			if !cloneNoSubmodules {
				if err := initSubmodules(destPath); err != nil {
					ui.Warning("some submodules may not have been initialized: %v", err)
				}
			}
			return nil
		}

		// Clean up partial clone on failure
		os.RemoveAll(destPath)

		// Check if it's a retryable error
		errStr := lastErr.Error()
		isRetryable := strings.Contains(errStr, "Connection reset") ||
			strings.Contains(errStr, "Connection timed out") ||
			strings.Contains(errStr, "Could not resolve host") ||
			strings.Contains(errStr, "SSL") ||
			strings.Contains(errStr, "early EOF") ||
			strings.Contains(errStr, "RPC failed") ||
			strings.Contains(errStr, "fetch-pack")

		if !isRetryable || attempt == maxRetries {
			break
		}

		ui.Warning("clone attempt %d failed, retrying... (%v)", attempt, lastErr)
		time.Sleep(time.Duration(attempt) * 2 * time.Second)
	}

	// Provide helpful error messages
	return formatGitError(lastErr, repoURL, branch)
}

// gitCloneSparse performs a sparse checkout - clones only root files initially
// This is useful for very large repositories
func gitCloneSparse(repoURL, destPath, branch string) error {
	// Step 1: Clone with no checkout
	args := []string{"clone", "--no-checkout", "--filter=blob:none", "--progress"}

	if branch != "" {
		args = append(args, "-b", branch)
	}

	if cloneSingleBranch || cloneDepth > 0 {
		args = append(args, "--single-branch")
	}

	if cloneDepth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", cloneDepth))
	}

	args = append(args, repoURL, destPath)

	ui.Status("sparse clone: fetching repository metadata...")
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return formatGitError(err, repoURL, branch)
	}

	// Step 2: Enable sparse checkout
	ui.Status("sparse clone: enabling sparse checkout...")
	cmd = exec.Command("git", "sparse-checkout", "init", "--cone")
	cmd.Dir = destPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize sparse checkout: %w", err)
	}

	// Step 3: Set sparse checkout to root only (empty set means top-level files only)
	cmd = exec.Command("git", "sparse-checkout", "set")
	cmd.Dir = destPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set sparse checkout: %w", err)
	}

	// Step 4: Checkout
	ui.Status("sparse clone: checking out root files...")
	checkoutArgs := []string{"checkout"}
	if branch != "" {
		checkoutArgs = append(checkoutArgs, branch)
	}
	cmd = exec.Command("git", checkoutArgs...)
	cmd.Dir = destPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout: %w", err)
	}

	ui.Info("sparse checkout complete. Use 'git sparse-checkout add <dir>' to add directories")
	return nil
}

// initSubmodules initializes and updates git submodules
func initSubmodules(repoPath string) error {
	// Check if .gitmodules exists
	gitmodulesPath := filepath.Join(repoPath, ".gitmodules")
	if _, err := os.Stat(gitmodulesPath); os.IsNotExist(err) {
		return nil // No submodules
	}

	ui.Status("initializing submodules...")

	cmd := exec.Command("git", "submodule", "update", "--init", "--recursive")
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// formatGitError provides user-friendly error messages for common git errors
func formatGitError(err error, repoURL, branch string) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Repository not found
	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "Repository not found") {
		return fmt.Errorf("repository not found: %s\n  • Check that the repository exists and is accessible\n  • For private repos, ensure you have proper authentication set up", repoURL)
	}

	// Authentication errors
	if strings.Contains(errStr, "Authentication failed") || strings.Contains(errStr, "Permission denied") {
		return fmt.Errorf("authentication failed for: %s\n  • For HTTPS: check your credentials or use a personal access token\n  • For SSH: ensure your SSH key is added to your Git provider", repoURL)
	}

	// Network errors
	if strings.Contains(errStr, "Could not resolve host") {
		return fmt.Errorf("could not resolve host\n  • Check your internet connection\n  • Verify the repository URL is correct: %s", repoURL)
	}

	// Invalid URL
	if strings.Contains(errStr, "not a git repository") {
		return fmt.Errorf("not a valid git repository: %s", repoURL)
	}

	// Branch not found
	if strings.Contains(errStr, "Remote branch") && strings.Contains(errStr, "not found") {
		branchName := branch
		if branchName == "" {
			branchName = "(default)"
		}
		return fmt.Errorf("branch '%s' not found in repository: %s\n  • Check that the branch exists\n  • Try without specifying a branch to use the default", branchName, repoURL)
	}

	// Large repository hints
	if strings.Contains(errStr, "RPC failed") || strings.Contains(errStr, "fetch-pack") {
		return fmt.Errorf("clone failed (possibly large repository): %s\n  • Try using --depth 1 for a shallow clone\n  • Check your network connection", repoURL)
	}

	return fmt.Errorf("git clone failed: %w", err)
}

// detectProjectStack analyzes project files to determine the tech stack
func detectProjectStack(projectPath string) string {
	// Define detection rules (order matters - more specific first)
	detectionRules := []struct {
		files    []string
		template string
	}{
		// Python projects
		{[]string{"requirements.txt"}, "python"},
		{[]string{"setup.py"}, "python"},
		{[]string{"pyproject.toml"}, "python"},
		{[]string{"Pipfile"}, "python"},
		{[]string{"poetry.lock"}, "python"},
		{[]string{"environment.yml"}, "python"}, // conda

		// Node.js projects
		{[]string{"package.json"}, "nodejs"},
		{[]string{"yarn.lock"}, "nodejs"},
		{[]string{"pnpm-lock.yaml"}, "nodejs"},
		{[]string{"bun.lockb"}, "nodejs"}, // Bun runtime

		// Go projects
		{[]string{"go.mod"}, "go"},
		{[]string{"go.sum"}, "go"},

		// Rust projects
		{[]string{"Cargo.toml"}, "rust"},
		{[]string{"Cargo.lock"}, "rust"},

		// Java projects
		{[]string{"pom.xml"}, "java"},             // Maven
		{[]string{"build.gradle"}, "java"},        // Gradle
		{[]string{"build.gradle.kts"}, "java"},    // Gradle Kotlin DSL
		{[]string{"settings.gradle"}, "java"},     // Gradle multi-project
		{[]string{"settings.gradle.kts"}, "java"}, // Gradle Kotlin multi-project
		{[]string{".mvn"}, "java"},                // Maven wrapper directory

		// Ruby projects
		{[]string{"Gemfile"}, "ruby"},
		{[]string{"Gemfile.lock"}, "ruby"},
		{[]string{".ruby-version"}, "ruby"},
		{[]string{"Rakefile"}, "ruby"},

		// PHP projects
		{[]string{"composer.json"}, "php"},
		{[]string{"composer.lock"}, "php"},
		{[]string{"artisan"}, "php"}, // Laravel

		// .NET / C# projects
		{[]string{"*.csproj"}, "dotnet"},
		{[]string{"*.fsproj"}, "dotnet"},
		{[]string{"*.sln"}, "dotnet"},
		{[]string{"global.json"}, "dotnet"},
		{[]string{"nuget.config"}, "dotnet"},

		// Elixir projects
		{[]string{"mix.exs"}, "elixir"},
		{[]string{"mix.lock"}, "elixir"},

		// Scala projects
		{[]string{"build.sbt"}, "scala"},
		{[]string{"build.sc"}, "scala"}, // Mill build tool

		// Kotlin projects (standalone, not Android)
		{[]string{"build.gradle.kts", "src/main/kotlin"}, "kotlin"},

		// Swift projects
		{[]string{"Package.swift"}, "swift"},

		// C/C++ projects
		{[]string{"CMakeLists.txt"}, "cpp"},
		{[]string{"Makefile", "*.c"}, "cpp"},
		{[]string{"meson.build"}, "cpp"},
		{[]string{"conanfile.txt"}, "cpp"},
		{[]string{"vcpkg.json"}, "cpp"},

		// Zig projects
		{[]string{"build.zig"}, "zig"},

		// Haskell projects
		{[]string{"stack.yaml"}, "haskell"},
		{[]string{"cabal.project"}, "haskell"},
		{[]string{"*.cabal"}, "haskell"},

		// OCaml projects
		{[]string{"dune-project"}, "ocaml"},
		{[]string{"*.opam"}, "ocaml"},

		// Web projects (static sites, multiple indicators)
		{[]string{"index.html"}, "web"},
	}

	for _, rule := range detectionRules {
		allFound := true
		for _, file := range rule.files {
			// Handle glob patterns
			if strings.Contains(file, "*") {
				matches, _ := filepath.Glob(filepath.Join(projectPath, file))
				if len(matches) == 0 {
					allFound = false
					break
				}
			} else if _, err := os.Stat(filepath.Join(projectPath, file)); os.IsNotExist(err) {
				allFound = false
				break
			}
		}
		if allFound {
			return rule.template
		}
	}

	return ""
}

// detectSetupCommands generates additional setup commands based on detected project files
func detectSetupCommands(projectPath, template string) []string {
	var commands []string

	// Helper to check if file exists
	fileExists := func(name string) bool {
		_, err := os.Stat(filepath.Join(projectPath, name))
		return err == nil
	}

	switch template {
	case "python":
		// Check for requirements.txt
		if fileExists("requirements.txt") {
			commands = append(commands, "pip3 install -r requirements.txt")
		}
		// Check for setup.py
		if fileExists("setup.py") {
			commands = append(commands, "pip3 install -e .")
		}
		// Check for pyproject.toml (poetry or pip)
		if fileExists("pyproject.toml") {
			if fileExists("poetry.lock") {
				commands = append(commands, "pip3 install poetry && poetry install")
			} else if fileExists("pdm.lock") {
				commands = append(commands, "pip3 install pdm && pdm install")
			} else {
				commands = append(commands, "pip3 install -e .")
			}
		}
		// Conda environment
		if fileExists("environment.yml") {
			commands = append(commands, "conda env create -f environment.yml || pip3 install -r requirements.txt 2>/dev/null || true")
		}

	case "nodejs":
		// Check for package-lock.json (npm)
		if fileExists("package-lock.json") {
			commands = append(commands, "npm ci")
		} else if fileExists("yarn.lock") {
			commands = append(commands, "npm install -g yarn && yarn install --frozen-lockfile")
		} else if fileExists("pnpm-lock.yaml") {
			commands = append(commands, "npm install -g pnpm && pnpm install --frozen-lockfile")
		} else if fileExists("bun.lockb") {
			commands = append(commands, "curl -fsSL https://bun.sh/install | bash && bun install")
		} else if fileExists("package.json") {
			commands = append(commands, "npm install")
		}

	case "go":
		if fileExists("go.mod") {
			commands = append(commands, "go mod download")
		}

	case "rust":
		if fileExists("Cargo.toml") {
			commands = append(commands, "cargo fetch")
		}

	case "java":
		if fileExists("pom.xml") {
			// Maven project
			if fileExists("mvnw") {
				commands = append(commands, "chmod +x mvnw && ./mvnw dependency:resolve")
			} else {
				commands = append(commands, "mvn dependency:resolve || apt-get update && apt-get install -y maven && mvn dependency:resolve")
			}
		} else if fileExists("build.gradle") || fileExists("build.gradle.kts") {
			// Gradle project
			if fileExists("gradlew") {
				commands = append(commands, "chmod +x gradlew && ./gradlew dependencies --refresh-dependencies")
			} else {
				commands = append(commands, "gradle dependencies || apt-get update && apt-get install -y gradle && gradle dependencies")
			}
		}

	case "ruby":
		if fileExists("Gemfile") {
			commands = append(commands, "gem install bundler && bundle install")
		}

	case "php":
		if fileExists("composer.json") {
			commands = append(commands, "composer install --no-interaction")
		}

	case "dotnet":
		// Find .csproj or .sln files
		if matches, _ := filepath.Glob(filepath.Join(projectPath, "*.sln")); len(matches) > 0 {
			commands = append(commands, "dotnet restore")
		} else if matches, _ := filepath.Glob(filepath.Join(projectPath, "*.csproj")); len(matches) > 0 {
			commands = append(commands, "dotnet restore")
		}

	case "elixir":
		if fileExists("mix.exs") {
			commands = append(commands, "mix local.hex --force && mix local.rebar --force && mix deps.get")
		}

	case "scala":
		if fileExists("build.sbt") {
			commands = append(commands, "sbt update")
		}

	case "swift":
		if fileExists("Package.swift") {
			commands = append(commands, "swift package resolve")
		}

	case "cpp":
		if fileExists("CMakeLists.txt") {
			commands = append(commands, "mkdir -p build && cd build && cmake ..")
		} else if fileExists("conanfile.txt") || fileExists("conanfile.py") {
			commands = append(commands, "conan install . --build=missing")
		} else if fileExists("vcpkg.json") {
			commands = append(commands, "vcpkg install")
		}

	case "haskell":
		if fileExists("stack.yaml") {
			commands = append(commands, "stack setup && stack build --only-dependencies")
		} else if matches, _ := filepath.Glob(filepath.Join(projectPath, "*.cabal")); len(matches) > 0 {
			commands = append(commands, "cabal update && cabal build --only-dependencies")
		}

	case "zig":
		// Zig doesn't have a package manager dependency step by default
		if fileExists("build.zig") {
			commands = append(commands, "zig build --fetch")
		}
	}

	return commands
}

// MonorepoInfo contains information about detected monorepo structure
type MonorepoInfo struct {
	IsMonorepo    bool
	Type          string   // "npm-workspaces", "yarn-workspaces", "pnpm-workspaces", "lerna", "nx", "turborepo", "cargo-workspaces", "go-workspaces"
	WorkspaceDirs []string // Detected workspace directories
}

// detectMonorepo analyzes the project to detect if it's a monorepo
func detectMonorepo(projectPath string) *MonorepoInfo {
	info := &MonorepoInfo{
		IsMonorepo:    false,
		WorkspaceDirs: []string{},
	}

	// Helper to check if file exists
	fileExists := func(name string) bool {
		_, err := os.Stat(filepath.Join(projectPath, name))
		return err == nil
	}

	// Check for Turborepo
	if fileExists("turbo.json") {
		info.IsMonorepo = true
		info.Type = "turborepo"
		info.WorkspaceDirs = findWorkspaceDirectories(projectPath)
		return info
	}

	// Check for Nx
	if fileExists("nx.json") {
		info.IsMonorepo = true
		info.Type = "nx"
		info.WorkspaceDirs = findWorkspaceDirectories(projectPath)
		return info
	}

	// Check for Lerna
	if fileExists("lerna.json") {
		info.IsMonorepo = true
		info.Type = "lerna"
		info.WorkspaceDirs = findWorkspaceDirectories(projectPath)
		return info
	}

	// Check for Rush
	if fileExists("rush.json") {
		info.IsMonorepo = true
		info.Type = "rush"
		info.WorkspaceDirs = findWorkspaceDirectories(projectPath)
		return info
	}

	// Check for pnpm workspaces
	if fileExists("pnpm-workspace.yaml") {
		info.IsMonorepo = true
		info.Type = "pnpm-workspaces"
		info.WorkspaceDirs = findWorkspaceDirectories(projectPath)
		return info
	}

	// Check for npm/yarn workspaces in package.json
	if fileExists("package.json") {
		data, err := os.ReadFile(filepath.Join(projectPath, "package.json"))
		if err == nil && strings.Contains(string(data), `"workspaces"`) {
			info.IsMonorepo = true
			if fileExists("yarn.lock") {
				info.Type = "yarn-workspaces"
			} else {
				info.Type = "npm-workspaces"
			}
			info.WorkspaceDirs = findWorkspaceDirectories(projectPath)
			return info
		}
	}

	// Check for Cargo workspaces
	if fileExists("Cargo.toml") {
		data, err := os.ReadFile(filepath.Join(projectPath, "Cargo.toml"))
		if err == nil && strings.Contains(string(data), "[workspace]") {
			info.IsMonorepo = true
			info.Type = "cargo-workspaces"
			info.WorkspaceDirs = findWorkspaceDirectories(projectPath)
			return info
		}
	}

	// Check for Go workspaces
	if fileExists("go.work") {
		info.IsMonorepo = true
		info.Type = "go-workspaces"
		info.WorkspaceDirs = findWorkspaceDirectories(projectPath)
		return info
	}

	return info
}

// findWorkspaceDirectories finds common workspace directories
func findWorkspaceDirectories(projectPath string) []string {
	commonDirs := []string{"packages", "apps", "libs", "services", "modules", "projects", "crates"}
	var found []string

	for _, dir := range commonDirs {
		dirPath := filepath.Join(projectPath, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			found = append(found, dir)
		}
	}

	return found
}

// getMonorepoSetupHint returns helpful info about detected monorepo
func getMonorepoSetupHint(info *MonorepoInfo) string {
	if !info.IsMonorepo {
		return ""
	}

	hints := map[string]string{
		"turborepo":        "Turborepo detected. Run 'turbo run build' to build all packages.",
		"nx":               "Nx workspace detected. Run 'npx nx run-many --target=build' to build all projects.",
		"lerna":            "Lerna monorepo detected. Run 'npx lerna bootstrap' to link packages.",
		"rush":             "Rush monorepo detected. Run 'rush update' to install dependencies.",
		"pnpm-workspaces":  "pnpm workspace detected. Dependencies are shared across packages.",
		"yarn-workspaces":  "Yarn workspace detected. Dependencies are shared across packages.",
		"npm-workspaces":   "npm workspace detected. Dependencies are shared across packages.",
		"cargo-workspaces": "Cargo workspace detected. Run 'cargo build --workspace' to build all crates.",
		"go-workspaces":    "Go workspace detected. All modules share the same build cache.",
	}

	if hint, ok := hints[info.Type]; ok {
		return hint
	}
	return ""
}
