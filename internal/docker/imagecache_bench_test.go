package docker

import (
	"testing"
)

func BenchmarkFingerprint(b *testing.B) {
	cfg := &BuildImageConfig{
		BaseImage: "ubuntu:22.04",
		SetupCommands: []string{
			"apt update -y",
			"DEBIAN_FRONTEND=noninteractive apt install -y python3 python3-pip git curl wget",
			"pip3 install flask django fastapi",
			"npm install -g typescript",
		},
		Environment: map[string]string{
			"PYTHONPATH":       "/workspace",
			"PYTHONUNBUFFERED": "1",
			"NODE_ENV":         "development",
		},
		WorkingDir:  "/workspace",
		ProjectName: "bench-project",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.Fingerprint()
	}
}

func BenchmarkGenerateDockerfile(b *testing.B) {
	ic := NewImageCache()
	cfg := &BuildImageConfig{
		BaseImage: "ubuntu:22.04",
		SetupCommands: []string{
			"apt update -y",
			"DEBIAN_FRONTEND=noninteractive apt install -y python3 python3-pip python3-venv python3-dev build-essential git curl wget ca-certificates",
			"pip3 install --upgrade pip setuptools wheel",
			"pip3 install flask django fastapi uvicorn gunicorn",
			"curl -fsSL https://deb.nodesource.com/setup_18.x | bash -",
			"apt install -y nodejs",
			"npm install -g npm@latest typescript ts-node",
			"apt-get clean && rm -rf /var/lib/apt/lists/*",
		},
		Environment: map[string]string{
			"PYTHONPATH":       "/workspace",
			"PYTHONUNBUFFERED": "1",
			"NODE_ENV":         "development",
			"PATH":             "/usr/local/go/bin:$PATH",
		},
		Labels: map[string]string{
			"devbox.project": "bench",
			"devbox.version": "1.0",
		},
		WorkingDir:  "/workspace",
		ProjectName: "bench-project",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ic.GenerateDockerfile(cfg)
	}
}

func BenchmarkFingerprintCacheHit(b *testing.B) {

	cfg := &BuildImageConfig{
		BaseImage: "ubuntu:22.04",
		SetupCommands: []string{
			"apt update -y",
			"DEBIAN_FRONTEND=noninteractive apt install -y python3 python3-pip git",
			"pip3 install flask",
		},
		Environment: map[string]string{"PYTHONPATH": "/workspace"},
		WorkingDir:  "/workspace",
		ProjectName: "test",
	}

	cachedFP := cfg.Fingerprint()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fp := cfg.Fingerprint()
		_ = fp == cachedFP
	}
}

func BenchmarkExtractAptPackages(b *testing.B) {
	cmd := "DEBIAN_FRONTEND=noninteractive apt install -y --no-install-recommends python3 python3-pip python3-venv python3-dev build-essential git curl wget ca-certificates gnupg"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractAptPackages(cmd)
	}
}
