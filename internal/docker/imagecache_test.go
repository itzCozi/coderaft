package docker

import (
	"strings"
	"testing"
)

func TestBuildImageConfigFingerprint(t *testing.T) {
	cfg1 := &BuildImageConfig{
		BaseImage:     "ubuntu:22.04",
		SetupCommands: []string{"apt update -y", "apt install -y git"},
		Environment:   map[string]string{"FOO": "bar"},
		WorkingDir:    "/workspace",
		ProjectName:   "test",
	}

	cfg2 := &BuildImageConfig{
		BaseImage:     "ubuntu:22.04",
		SetupCommands: []string{"apt update -y", "apt install -y git"},
		Environment:   map[string]string{"FOO": "bar"},
		WorkingDir:    "/workspace",
		ProjectName:   "test",
	}

	// Same config should produce same fingerprint
	fp1 := cfg1.Fingerprint()
	fp2 := cfg2.Fingerprint()
	if fp1 != fp2 {
		t.Errorf("Same config should produce same fingerprint: %s != %s", fp1, fp2)
	}

	// Fingerprint should be 16 chars hex
	if len(fp1) != 16 {
		t.Errorf("Fingerprint should be 16 chars, got %d: %s", len(fp1), fp1)
	}
}

func TestBuildImageConfigFingerprintDiffers(t *testing.T) {
	cfg1 := &BuildImageConfig{
		BaseImage:     "ubuntu:22.04",
		SetupCommands: []string{"apt update -y"},
		ProjectName:   "test",
	}

	cfg2 := &BuildImageConfig{
		BaseImage:     "ubuntu:24.04",
		SetupCommands: []string{"apt update -y"},
		ProjectName:   "test",
	}

	if cfg1.Fingerprint() == cfg2.Fingerprint() {
		t.Error("Different base images should produce different fingerprints")
	}

	cfg3 := &BuildImageConfig{
		BaseImage:     "ubuntu:22.04",
		SetupCommands: []string{"apt update -y", "apt install -y git"},
		ProjectName:   "test",
	}

	if cfg1.Fingerprint() == cfg3.Fingerprint() {
		t.Error("Different commands should produce different fingerprints")
	}
}

func TestBuildImageConfigFingerprintEnvOrder(t *testing.T) {
	// Environment variable order should not affect fingerprint
	cfg1 := &BuildImageConfig{
		BaseImage:   "ubuntu:22.04",
		Environment: map[string]string{"A": "1", "B": "2", "C": "3"},
		ProjectName: "test",
	}

	cfg2 := &BuildImageConfig{
		BaseImage:   "ubuntu:22.04",
		Environment: map[string]string{"C": "3", "A": "1", "B": "2"},
		ProjectName: "test",
	}

	if cfg1.Fingerprint() != cfg2.Fingerprint() {
		t.Error("Environment variable order should not affect fingerprint")
	}
}

func TestGenerateDockerfile(t *testing.T) {
	ic := NewImageCache()

	cfg := &BuildImageConfig{
		BaseImage: "ubuntu:22.04",
		SetupCommands: []string{
			"apt update -y",
			"DEBIAN_FRONTEND=noninteractive apt install -y python3 python3-pip git",
			"pip3 install flask",
		},
		Environment: map[string]string{
			"PYTHONPATH": "/workspace",
		},
		WorkingDir:  "/workspace",
		ProjectName: "test-project",
	}

	dockerfile := ic.GenerateDockerfile(cfg)

	// Should start with FROM
	if !strings.HasPrefix(dockerfile, "FROM ubuntu:22.04") {
		t.Errorf("Dockerfile should start with FROM, got: %s", dockerfile[:50])
	}

	// Should contain ENV
	if !strings.Contains(dockerfile, "ENV PYTHONPATH=") {
		t.Error("Dockerfile should contain ENV directive")
	}

	// Should have optimized apt layer with --no-install-recommends
	if !strings.Contains(dockerfile, "--no-install-recommends") {
		t.Error("Dockerfile should use --no-install-recommends for apt")
	}

	// Should clean apt cache in same layer
	if !strings.Contains(dockerfile, "rm -rf /var/lib/apt/lists/*") {
		t.Error("Dockerfile should clean apt cache")
	}

	// Should contain pip install as separate command
	if !strings.Contains(dockerfile, "pip3 install flask") {
		t.Error("Dockerfile should contain pip install command")
	}

	// Should have WORKDIR
	if !strings.Contains(dockerfile, "WORKDIR /workspace") {
		t.Error("Dockerfile should contain WORKDIR")
	}

	// Should NOT contain apt full-upgrade (we skip upgrades for cache stability)
	if strings.Contains(dockerfile, "full-upgrade") {
		t.Error("Dockerfile should not contain apt full-upgrade")
	}
}

func TestGenerateDockerfileNoCommands(t *testing.T) {
	ic := NewImageCache()

	cfg := &BuildImageConfig{
		BaseImage:   "ubuntu:22.04",
		ProjectName: "empty",
	}

	dockerfile := ic.GenerateDockerfile(cfg)

	if !strings.HasPrefix(dockerfile, "FROM ubuntu:22.04") {
		t.Error("Dockerfile should start with FROM")
	}

	if !strings.Contains(dockerfile, "WORKDIR /workspace") {
		t.Error("Dockerfile should have default WORKDIR")
	}
}

func TestExtractAptPackages(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			"apt install -y python3 git curl",
			[]string{"python3", "git", "curl"},
		},
		{
			"DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends python3 python3-pip",
			[]string{"python3", "python3-pip"},
		},
		{
			"apt-get install -y git",
			[]string{"git"},
		},
	}

	for _, tt := range tests {
		pkgs := extractAptPackages(tt.input)
		if len(pkgs) != len(tt.expected) {
			t.Errorf("extractAptPackages(%q) = %v, want %v", tt.input, pkgs, tt.expected)
			continue
		}
		for i, pkg := range pkgs {
			if pkg != tt.expected[i] {
				t.Errorf("extractAptPackages(%q)[%d] = %s, want %s", tt.input, i, pkg, tt.expected[i])
			}
		}
	}
}

func TestMountConsistencyFlag(t *testing.T) {
	// Just ensure it doesn't panic and returns a string
	flag := MountConsistencyFlag()
	// On any OS it should be empty string or ":delegated"
	if flag != "" && flag != ":delegated" {
		t.Errorf("Unexpected consistency flag: %s", flag)
	}
}
