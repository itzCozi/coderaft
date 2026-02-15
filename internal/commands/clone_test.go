package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeRepoURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		// Full URLs should pass through unchanged
		{
			name:     "full https URL",
			input:    "https://github.com/user/repo",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "full https URL with .git",
			input:    "https://github.com/user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
		{
			name:     "SSH URL",
			input:    "git@github.com:user/repo.git",
			expected: "git@github.com:user/repo.git",
		},
		{
			name:     "git protocol",
			input:    "git://github.com/user/repo.git",
			expected: "git://github.com/user/repo.git",
		},

		// Shorthand formats
		{
			name:     "GitHub shorthand user/repo",
			input:    "user/repo",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "GitHub shorthand org/repo",
			input:    "facebook/react",
			expected: "https://github.com/facebook/react",
		},
		{
			name:     "GitHub shorthand with .git",
			input:    "user/repo.git",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "shorthand with hyphen",
			input:    "my-org/my-repo",
			expected: "https://github.com/my-org/my-repo",
		},
		{
			name:     "shorthand with underscore",
			input:    "my_org/my_repo",
			expected: "https://github.com/my_org/my_repo",
		},
		{
			name:     "shorthand with numbers",
			input:    "user123/project456",
			expected: "https://github.com/user123/project456",
		},

		// Host prefix formats
		{
			name:     "github.com prefix",
			input:    "github.com/user/repo",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "gitlab.com prefix",
			input:    "gitlab.com/user/repo",
			expected: "https://gitlab.com/user/repo",
		},
		{
			name:     "bitbucket.org prefix",
			input:    "bitbucket.org/team/repo",
			expected: "https://bitbucket.org/team/repo",
		},

		// Protocol shortcuts
		{
			name:     "github: shortcut",
			input:    "github:user/repo",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "gh: shortcut",
			input:    "gh:user/repo",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "gitlab: shortcut",
			input:    "gitlab:user/repo",
			expected: "https://gitlab.com/user/repo",
		},
		{
			name:     "gl: shortcut",
			input:    "gl:user/repo",
			expected: "https://gitlab.com/user/repo",
		},
		{
			name:     "bitbucket: shortcut",
			input:    "bitbucket:team/repo",
			expected: "https://bitbucket.org/team/repo",
		},
		{
			name:     "bb: shortcut",
			input:    "bb:team/repo",
			expected: "https://bitbucket.org/team/repo",
		},

		// Browser URL formats (should strip extra paths)
		{
			name:     "GitHub browser URL with tree/branch",
			input:    "https://github.com/user/repo/tree/main",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "GitHub browser URL with tree/branch/path",
			input:    "https://github.com/user/repo/tree/main/src/components",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "GitHub browser URL with blob/file",
			input:    "https://github.com/user/repo/blob/main/README.md",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "GitHub browser URL with pull request",
			input:    "https://github.com/user/repo/pull/123",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "GitHub browser URL with issues",
			input:    "https://github.com/user/repo/issues/456",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "GitHub browser URL with commit",
			input:    "https://github.com/user/repo/commit/abc123def",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "GitHub browser URL trailing slash",
			input:    "https://github.com/user/repo/",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "GitHub browser URL with .git suffix",
			input:    "https://github.com/user/repo.git/",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "GitLab nested group URL",
			input:    "https://gitlab.com/group/subgroup/repo",
			expected: "https://gitlab.com/group/subgroup/repo",
		},
		{
			name:     "GitLab URL with /-/ path",
			input:    "https://gitlab.com/group/repo/-/tree/main",
			expected: "https://gitlab.com/group/repo",
		},
		{
			name:     "host prefix with trailing slash",
			input:    "github.com/user/repo/",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "host prefix with tree path",
			input:    "github.com/user/repo/tree/develop",
			expected: "https://github.com/user/repo",
		},

		// Error cases
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "single word (no slash)",
			input:   "repo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeRepoURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("normalizeRepoURL(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("normalizeRepoURL(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExtractProjectName(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
		wantErr  bool
	}{
		{
			name:     "https URL",
			repoURL:  "https://github.com/user/myproject",
			expected: "myproject",
		},
		{
			name:     "https URL with .git",
			repoURL:  "https://github.com/user/myproject.git",
			expected: "myproject",
		},
		{
			name:     "SSH URL",
			repoURL:  "git@github.com:user/myproject.git",
			expected: "myproject",
		},
		{
			name:     "SSH URL without .git",
			repoURL:  "git@github.com:user/myproject",
			expected: "myproject",
		},
		{
			name:     "nested path",
			repoURL:  "https://gitlab.com/group/subgroup/project.git",
			expected: "project",
		},
		{
			name:     "bitbucket URL",
			repoURL:  "https://bitbucket.org/team/repo-name.git",
			expected: "repo-name",
		},
		{
			name:     "with hyphens and numbers",
			repoURL:  "https://github.com/user/my-project-123.git",
			expected: "my-project-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractProjectName(tt.repoURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractProjectName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("extractProjectName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCleanRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user/repo.git", "repo"},
		{"user/repo", "repo"},
		{"group/subgroup/project.git", "project"},
		{"simple", "simple"},
		{"repo.git", "repo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanRepoName(tt.input)
			if got != tt.expected {
				t.Errorf("cleanRepoName(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDetectProjectStack(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected string
	}{
		{
			name:     "Python with requirements.txt",
			files:    []string{"requirements.txt"},
			expected: "python",
		},
		{
			name:     "Python with pyproject.toml",
			files:    []string{"pyproject.toml"},
			expected: "python",
		},
		{
			name:     "Python with Pipfile",
			files:    []string{"Pipfile"},
			expected: "python",
		},
		{
			name:     "Node.js with package.json",
			files:    []string{"package.json"},
			expected: "nodejs",
		},
		{
			name:     "Node.js with yarn.lock",
			files:    []string{"yarn.lock"},
			expected: "nodejs",
		},
		{
			name:     "Go with go.mod",
			files:    []string{"go.mod"},
			expected: "go",
		},
		{
			name:     "Rust with Cargo.toml",
			files:    []string{"Cargo.toml"},
			expected: "rust",
		},
		{
			name:     "Web project (index.html without package.json prefers nodejs if package.json exists)",
			files:    []string{"index.html", "package.json"},
			expected: "nodejs", // package.json triggers nodejs before web check
		},
		{
			name:     "No stack detected",
			files:    []string{"README.md", "LICENSE"},
			expected: "",
		},
		{
			name:     "Empty directory",
			files:    []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "coderaft-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create test files
			for _, f := range tt.files {
				filePath := filepath.Join(tmpDir, f)
				if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
					t.Fatalf("failed to create test file %s: %v", f, err)
				}
			}

			got := detectProjectStack(tmpDir)
			if got != tt.expected {
				t.Errorf("detectProjectStack() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDetectSetupCommands(t *testing.T) {
	tests := []struct {
		name     string
		template string
		files    []string
		wantCmd  string // substring to check for in commands
	}{
		{
			name:     "Python requirements.txt",
			template: "python",
			files:    []string{"requirements.txt"},
			wantCmd:  "pip3 install -r requirements.txt",
		},
		{
			name:     "Python setup.py",
			template: "python",
			files:    []string{"setup.py"},
			wantCmd:  "pip3 install -e .",
		},
		{
			name:     "Python poetry",
			template: "python",
			files:    []string{"pyproject.toml", "poetry.lock"},
			wantCmd:  "poetry install",
		},
		{
			name:     "Node.js npm",
			template: "nodejs",
			files:    []string{"package.json", "package-lock.json"},
			wantCmd:  "npm ci",
		},
		{
			name:     "Node.js yarn",
			template: "nodejs",
			files:    []string{"package.json", "yarn.lock"},
			wantCmd:  "yarn install",
		},
		{
			name:     "Node.js pnpm",
			template: "nodejs",
			files:    []string{"package.json", "pnpm-lock.yaml"},
			wantCmd:  "pnpm install",
		},
		{
			name:     "Go modules",
			template: "go",
			files:    []string{"go.mod"},
			wantCmd:  "go mod download",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "coderaft-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(tmpDir, f), []byte{}, 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
			}

			cmds := detectSetupCommands(tmpDir, tt.template)

			found := false
			for _, cmd := range cmds {
				if contains(cmd, tt.wantCmd) {
					found = true
					break
				}
			}
			if !found && tt.wantCmd != "" {
				t.Errorf("detectSetupCommands() did not contain %q, got %v", tt.wantCmd, cmds)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr)))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDetectProjectStackExtended(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected string
	}{
		// Java projects
		{
			name:     "Java with Maven",
			files:    []string{"pom.xml"},
			expected: "java",
		},
		{
			name:     "Java with Gradle",
			files:    []string{"build.gradle"},
			expected: "java",
		},
		{
			name:     "Java with Gradle Kotlin DSL",
			files:    []string{"build.gradle.kts"},
			expected: "java",
		},
		// Ruby projects
		{
			name:     "Ruby with Gemfile",
			files:    []string{"Gemfile"},
			expected: "ruby",
		},
		{
			name:     "Ruby with ruby-version",
			files:    []string{".ruby-version"},
			expected: "ruby",
		},
		// PHP projects
		{
			name:     "PHP with Composer",
			files:    []string{"composer.json"},
			expected: "php",
		},
		{
			name:     "PHP Laravel",
			files:    []string{"artisan"},
			expected: "php",
		},
		// Rust projects
		{
			name:     "Rust with Cargo.lock",
			files:    []string{"Cargo.lock"},
			expected: "rust",
		},
		// Elixir projects
		{
			name:     "Elixir with mix",
			files:    []string{"mix.exs"},
			expected: "elixir",
		},
		// Scala projects
		{
			name:     "Scala with SBT",
			files:    []string{"build.sbt"},
			expected: "scala",
		},
		// Swift projects
		{
			name:     "Swift Package",
			files:    []string{"Package.swift"},
			expected: "swift",
		},
		// C++ projects
		{
			name:     "C++ with CMake",
			files:    []string{"CMakeLists.txt"},
			expected: "cpp",
		},
		{
			name:     "C++ with meson",
			files:    []string{"meson.build"},
			expected: "cpp",
		},
		// Zig projects
		{
			name:     "Zig project",
			files:    []string{"build.zig"},
			expected: "zig",
		},
		// Haskell projects
		{
			name:     "Haskell with Stack",
			files:    []string{"stack.yaml"},
			expected: "haskell",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "coderaft-stack-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			for _, f := range tt.files {
				filePath := filepath.Join(tmpDir, f)
				if err := os.WriteFile(filePath, []byte{}, 0644); err != nil {
					t.Fatalf("failed to create test file %s: %v", f, err)
				}
			}

			got := detectProjectStack(tmpDir)
			if got != tt.expected {
				t.Errorf("detectProjectStack() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDetectMonorepo(t *testing.T) {
	tests := []struct {
		name         string
		files        []string
		fileContents map[string]string
		wantMonorepo bool
		wantType     string
	}{
		{
			name:         "Turborepo",
			files:        []string{"turbo.json", "package.json"},
			wantMonorepo: true,
			wantType:     "turborepo",
		},
		{
			name:         "Nx workspace",
			files:        []string{"nx.json", "package.json"},
			wantMonorepo: true,
			wantType:     "nx",
		},
		{
			name:         "Lerna",
			files:        []string{"lerna.json", "package.json"},
			wantMonorepo: true,
			wantType:     "lerna",
		},
		{
			name:         "pnpm workspaces",
			files:        []string{"pnpm-workspace.yaml", "package.json"},
			wantMonorepo: true,
			wantType:     "pnpm-workspaces",
		},
		{
			name:  "npm workspaces",
			files: []string{"package.json"},
			fileContents: map[string]string{
				"package.json": `{"name": "monorepo", "workspaces": ["packages/*"]}`,
			},
			wantMonorepo: true,
			wantType:     "npm-workspaces",
		},
		{
			name:  "yarn workspaces",
			files: []string{"package.json", "yarn.lock"},
			fileContents: map[string]string{
				"package.json": `{"name": "monorepo", "workspaces": ["packages/*"]}`,
			},
			wantMonorepo: true,
			wantType:     "yarn-workspaces",
		},
		{
			name:  "Cargo workspace",
			files: []string{"Cargo.toml"},
			fileContents: map[string]string{
				"Cargo.toml": `[workspace]\nmembers = ["crates/*"]`,
			},
			wantMonorepo: true,
			wantType:     "cargo-workspaces",
		},
		{
			name:         "Go workspace",
			files:        []string{"go.work"},
			wantMonorepo: true,
			wantType:     "go-workspaces",
		},
		{
			name:         "Not a monorepo",
			files:        []string{"package.json", "index.js"},
			wantMonorepo: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "coderaft-monorepo-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			for _, f := range tt.files {
				filePath := filepath.Join(tmpDir, f)
				content := ""
				if tt.fileContents != nil {
					content = tt.fileContents[f]
				}
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatalf("failed to create test file %s: %v", f, err)
				}
			}

			info := detectMonorepo(tmpDir)

			if info.IsMonorepo != tt.wantMonorepo {
				t.Errorf("detectMonorepo().IsMonorepo = %v, want %v", info.IsMonorepo, tt.wantMonorepo)
			}

			if tt.wantMonorepo && info.Type != tt.wantType {
				t.Errorf("detectMonorepo().Type = %v, want %v", info.Type, tt.wantType)
			}
		})
	}
}

func TestIsValidRepoPath(t *testing.T) {
	tests := []struct {
		path  string
		valid bool
	}{
		{"user/repo", true},
		{"org/project", true},
		{"my-org/my-repo", true},
		{"user123/repo456", true},
		{"a/b", true},
		{"user", false},
		{"/repo", false},
		{"user/", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isValidRepoPath(tt.path)
			if got != tt.valid {
				t.Errorf("isValidRepoPath(%q) = %v, want %v", tt.path, got, tt.valid)
			}
		})
	}
}

func TestCleanBrowserURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain repo URL unchanged",
			input:    "https://github.com/user/repo",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips tree/branch",
			input:    "https://github.com/user/repo/tree/main",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips tree/branch/path",
			input:    "https://github.com/user/repo/tree/main/src/components",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips blob/file",
			input:    "https://github.com/user/repo/blob/develop/README.md",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips pull request",
			input:    "https://github.com/user/repo/pull/123",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips issues",
			input:    "https://github.com/user/repo/issues/456",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips commit",
			input:    "https://github.com/user/repo/commit/abc123",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips compare",
			input:    "https://github.com/user/repo/compare/main...develop",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips releases",
			input:    "https://github.com/user/repo/releases",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips actions",
			input:    "https://github.com/user/repo/actions",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips wiki",
			input:    "https://github.com/user/repo/wiki",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips .git suffix",
			input:    "https://github.com/user/repo.git",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips trailing slash",
			input:    "https://github.com/user/repo/",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips query params",
			input:    "https://github.com/user/repo?tab=readme",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "strips fragment",
			input:    "https://github.com/user/repo#readme",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "GitLab with /-/ path",
			input:    "https://gitlab.com/user/repo/-/tree/main",
			expected: "https://gitlab.com/user/repo",
		},
		{
			name:     "GitLab nested groups preserved",
			input:    "https://gitlab.com/group/subgroup/repo",
			expected: "https://gitlab.com/group/subgroup/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanBrowserURL(tt.input)
			if got != tt.expected {
				t.Errorf("cleanBrowserURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExtractBranchFromURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no branch in URL",
			input:    "https://github.com/user/repo",
			expected: "",
		},
		{
			name:     "tree/main",
			input:    "https://github.com/user/repo/tree/main",
			expected: "main",
		},
		{
			name:     "tree/develop",
			input:    "https://github.com/user/repo/tree/develop",
			expected: "develop",
		},
		{
			name:     "tree/feature-branch",
			input:    "https://github.com/user/repo/tree/feature-branch",
			expected: "feature-branch",
		},
		{
			name:     "tree/branch with path",
			input:    "https://github.com/user/repo/tree/main/src/components",
			expected: "main",
		},
		{
			name:     "blob/branch/file",
			input:    "https://github.com/user/repo/blob/develop/README.md",
			expected: "develop",
		},
		{
			name:     "commits/branch",
			input:    "https://github.com/user/repo/commits/main",
			expected: "main",
		},
		{
			name:     "pull request (no branch)",
			input:    "https://github.com/user/repo/pull/123",
			expected: "",
		},
		{
			name:     "issues (no branch)",
			input:    "https://github.com/user/repo/issues/456",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBranchFromURL(tt.input)
			if got != tt.expected {
				t.Errorf("extractBranchFromURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
