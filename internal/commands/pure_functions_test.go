package commands

import (
	"sort"
	"strings"
	"testing"
)





func TestComputeLockChecksum_Deterministic(t *testing.T) {
	lf := &lockFile{
		BaseImage: lockImage{Name: "ubuntu:22.04", Digest: "sha256:abc123"},
		Container: lockContainer{
			WorkingDir:   "/workspace",
			User:         "root",
			Restart:      "unless-stopped",
			Network:      "bridge",
			Gpus:         "all",
			Ports:        []string{"8080:80"},
			Volumes:      []string{"/host:/island"},
			Capabilities: []string{"SYS_PTRACE"},
			Environment:  map[string]string{"GO111MODULE": "on"},
			Labels:       map[string]string{"app": "coderaft"},
			Resources:    map[string]string{"memory": "2g"},
		},
		SetupScript: []string{"apt update -y"},
		Packages: lockPackages{
			Apt: []string{"git=1:2.39.2-1"},
			Pip: []string{"flask==2.3.0"},
		},
		Registries: lockRegistries{
			PipIndexURL: "https://pypi.org/simple",
			NpmRegistry: "https://registry.npmjs.org",
		},
		AptSources: lockAptSources{
			SnapshotURL:   "https://snapshot.debian.org",
			SourcesLists:  []string{"deb http://deb.debian.org/debian bookworm main"},
			PinnedRelease: "bookworm",
		},
	}

	cs1 := computeLockChecksum(lf)
	cs2 := computeLockChecksum(lf)

	if cs1 != cs2 {
		t.Fatalf("expected identical checksums, got %s vs %s", cs1, cs2)
	}
	if !strings.HasPrefix(cs1, "sha256:") {
		t.Fatalf("expected sha256: prefix, got %s", cs1)
	}
	if len(cs1) != 7+64 { 
		t.Fatalf("unexpected checksum length: %d", len(cs1))
	}
}

func TestComputeLockChecksum_ChangesOnDifferentInput(t *testing.T) {
	base := lockFile{
		BaseImage: lockImage{Name: "ubuntu:22.04", Digest: "sha256:abc"},
		Packages:  lockPackages{Apt: []string{"git=1:2.39.2-1"}},
	}
	csBase := computeLockChecksum(&base)

	
	altered := base
	altered.BaseImage.Name = "debian:bookworm"
	csAltered := computeLockChecksum(&altered)
	if csBase == csAltered {
		t.Fatal("changing base_image.name should change checksum")
	}

	
	altered2 := base
	altered2.Packages.Apt = []string{"git=1:2.40.0-1"}
	if csBase == computeLockChecksum(&altered2) {
		t.Fatal("changing a package version should change checksum")
	}

	
	altered3 := base
	altered3.Container.Gpus = "all"
	if csBase == computeLockChecksum(&altered3) {
		t.Fatal("adding GPU setting should change checksum")
	}

	
	altered4 := base
	altered4.SetupScript = []string{"echo hello"}
	if csBase == computeLockChecksum(&altered4) {
		t.Fatal("adding setup commands should change checksum")
	}
}

func TestComputeLockChecksum_MapOrderInsensitive(t *testing.T) {
	lf1 := &lockFile{
		BaseImage: lockImage{Name: "ubuntu:22.04"},
		Container: lockContainer{
			Environment: map[string]string{"A": "1", "B": "2", "C": "3"},
		},
	}
	lf2 := &lockFile{
		BaseImage: lockImage{Name: "ubuntu:22.04"},
		Container: lockContainer{
			Environment: map[string]string{"C": "3", "A": "1", "B": "2"},
		},
	}
	if computeLockChecksum(lf1) != computeLockChecksum(lf2) {
		t.Fatal("environment map order should not affect checksum")
	}
}





func TestEscapeBash(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{"it's", "it'\\''s"},
		{`say "hi"`, `say \"hi\"`},
		{"$HOME", `\$HOME`},
		{"`cmd`", "\\`cmd\\`"},
		{`back\slash`, `back\\slash`},
		{"all'at$once`now\"", "all'\\''at\\$once\\`now\\\""},
	}
	for _, tt := range tests {
		got := escapeBash(tt.in)
		if got != tt.want {
			t.Errorf("escapeBash(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}





func TestParseMap_AptSeparator(t *testing.T) {
	lines := []string{"git=1:2.39.2-1", "curl=7.88.1-10", ""}
	got := parseMap(lines, "=")
	if got["git"] != "1:2.39.2-1" {
		t.Errorf("expected git=1:2.39.2-1, got %q", got["git"])
	}
	if got["curl"] != "7.88.1-10" {
		t.Errorf("expected curl=7.88.1-10, got %q", got["curl"])
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}
}

func TestParseMap_PipSeparator(t *testing.T) {
	lines := []string{"Flask==2.3.0", "requests==2.31.0"}
	got := parseMap(lines, "==")
	if got["flask"] != "2.3.0" {
		t.Errorf("expected flask=2.3.0, got %q", got["flask"])
	}
	if got["requests"] != "2.31.0" {
		t.Errorf("expected requests=2.31.0, got %q", got["requests"])
	}
}

func TestParseMap_NpmAtSeparator(t *testing.T) {
	lines := []string{"express@4.18.2", "@types/node@20.4.5"}
	got := parseMap(lines, "@")
	if got["express"] != "4.18.2" {
		t.Errorf("expected express=4.18.2, got %q", got["express"])
	}
	
	if got["@types/node"] != "20.4.5" {
		t.Errorf("expected @types/node=20.4.5, got %v", got)
	}
}

func TestParseMap_EmptyAndWhitespace(t *testing.T) {
	lines := []string{"", "  ", "  git=1.0  "}
	got := parseMap(lines, "=")
	if len(got) != 1 {
		t.Errorf("expected 1 entry, got %d", len(got))
	}
	if got["git"] != "1.0" {
		t.Errorf("expected git=1.0, got %q", got["git"])
	}
}





func TestKeysNotIn(t *testing.T) {
	a := map[string]string{"a": "1", "b": "2", "c": "3"}
	b := map[string]string{"b": "2", "d": "4"}
	got := keysNotIn(a, b)
	sort.Strings(got)
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("expected [a c], got %v", got)
	}
}

func TestKeysNotIn_Empty(t *testing.T) {
	empty := map[string]string{}
	full := map[string]string{"a": "1"}
	if keys := keysNotIn(empty, full); len(keys) != 0 {
		t.Errorf("expected empty, got %v", keys)
	}
	if keys := keysNotIn(full, full); len(keys) != 0 {
		t.Errorf("expected empty, got %v", keys)
	}
}





func TestValidateRegistryURL_Valid(t *testing.T) {
	valid := []string{
		"https://registry.npmjs.org",
		"http://localhost:4873",
		"file:///opt/local-repo",
		"",
	}
	for _, u := range valid {
		if err := validateRegistryURL("test", u); err != nil {
			t.Errorf("validateRegistryURL(%q) = %v, want nil", u, err)
		}
	}
}

func TestValidateRegistryURL_Invalid(t *testing.T) {
	invalid := []string{
		"ftp://mirror.example.com",
		"gopher://old.protocol.test",
		"://missing-scheme",
	}
	for _, u := range invalid {
		if err := validateRegistryURL("test", u); err == nil {
			t.Errorf("validateRegistryURL(%q) = nil, want error", u)
		}
	}
}





func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://example.com/", "https://example.com"},
		{"  HTTPS://Example.COM/path/  ", "https://example.com/path"},
		{"http://localhost", "http://localhost"},
	}
	for _, tt := range tests {
		if got := normalizeURL(tt.in); got != tt.want {
			t.Errorf("normalizeURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}





func TestStringSetEqual(t *testing.T) {
	tests := []struct {
		a, b []string
		want bool
	}{
		{nil, nil, true},
		{[]string{}, nil, true},
		{[]string{"a", "b"}, []string{"b", "a"}, true},
		{[]string{"a", "b"}, []string{"a", "c"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
		{[]string{" a ", " b "}, []string{"b", "a"}, true},
		{[]string{"", "a"}, []string{"a"}, true}, 
	}
	for i, tt := range tests {
		if got := stringSetEqual(tt.a, tt.b); got != tt.want {
			t.Errorf("case %d: stringSetEqual(%v, %v) = %v, want %v", i, tt.a, tt.b, got, tt.want)
		}
	}
}





func TestPackageDiff_NoDrift(t *testing.T) {
	locked := []string{"git=1:2.39.2-1", "curl=7.88.1-10"}
	live := []string{"git=1:2.39.2-1", "curl=7.88.1-10"}
	if drifts := packageDiff("apt", "=", locked, live); len(drifts) != 0 {
		t.Errorf("expected no drift, got %v", drifts)
	}
}

func TestPackageDiff_Added(t *testing.T) {
	locked := []string{"git=1:2.39.2-1"}
	live := []string{"git=1:2.39.2-1", "vim=9.0.1-1"}
	drifts := packageDiff("apt", "=", locked, live)
	if len(drifts) == 0 {
		t.Fatal("expected drift")
	}
	found := false
	for _, d := range drifts {
		if strings.Contains(d, "+ vim") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected added vim entry, got %v", drifts)
	}
}

func TestPackageDiff_Removed(t *testing.T) {
	locked := []string{"git=1:2.39.2-1", "curl=7.88.1-10"}
	live := []string{"git=1:2.39.2-1"}
	drifts := packageDiff("apt", "=", locked, live)
	found := false
	for _, d := range drifts {
		if strings.Contains(d, "- curl") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected removed curl entry, got %v", drifts)
	}
}

func TestPackageDiff_Changed(t *testing.T) {
	locked := []string{"flask==2.3.0"}
	live := []string{"flask==2.4.0"}
	drifts := packageDiff("pip", "==", locked, live)
	found := false
	for _, d := range drifts {
		if strings.Contains(d, "~ flask") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected changed flask entry, got %v", drifts)
	}
}





func TestBuildReconcileActions_NoChanges(t *testing.T) {
	pkgs := lockPackages{
		Apt: []string{"git=1:2.39.2-1"},
	}
	cmds := buildReconcileActions(pkgs, []string{"git=1:2.39.2-1"}, nil, nil, nil, nil)
	if len(cmds) != 0 {
		t.Errorf("expected no commands, got %v", cmds)
	}
}

func TestBuildReconcileActions_InstallMissing(t *testing.T) {
	pkgs := lockPackages{
		Apt: []string{"git=1:2.39.2-1", "curl=7.88.1-10"},
	}
	cmds := buildReconcileActions(pkgs, []string{"git=1:2.39.2-1"}, nil, nil, nil, nil)
	if len(cmds) == 0 {
		t.Fatal("expected install commands")
	}
	hasInstall := false
	for _, c := range cmds {
		if strings.Contains(c, "apt-get install") && strings.Contains(c, "curl") {
			hasInstall = true
		}
	}
	if !hasInstall {
		t.Errorf("expected apt install command for curl, got %v", cmds)
	}
}

func TestBuildReconcileActions_RemoveExtra(t *testing.T) {
	pkgs := lockPackages{
		Apt: []string{"git=1:2.39.2-1"},
	}
	cmds := buildReconcileActions(pkgs, []string{"git=1:2.39.2-1", "vim=9.0.1-1"}, nil, nil, nil, nil)
	hasRemove := false
	for _, c := range cmds {
		if strings.Contains(c, "apt-get remove") && strings.Contains(c, "vim") {
			hasRemove = true
		}
	}
	if !hasRemove {
		t.Errorf("expected apt remove command for vim, got %v", cmds)
	}
}

func TestBuildReconcileActions_PipInstallAndUninstall(t *testing.T) {
	pkgs := lockPackages{
		Pip: []string{"flask==2.3.0"},
	}
	cmds := buildReconcileActions(pkgs, nil, []string{"requests==2.31.0"}, nil, nil, nil)
	hasInstall := false
	hasUninstall := false
	for _, c := range cmds {
		if strings.Contains(c, "pip install flask==2.3.0") {
			hasInstall = true
		}
		if strings.Contains(c, "pip uninstall -y requests") {
			hasUninstall = true
		}
	}
	if !hasInstall {
		t.Error("expected pip install flask")
	}
	if !hasUninstall {
		t.Error("expected pip uninstall requests")
	}
}

func TestBuildReconcileActions_NpmAndYarnAndPnpm(t *testing.T) {
	pkgs := lockPackages{
		Npm:  []string{"express@4.18.2"},
		Yarn: []string{"lodash@4.17.21"},
		Pnpm: []string{"typescript@5.1.6"},
	}
	cmds := buildReconcileActions(pkgs, nil, nil, nil, nil, nil)
	hasNpm := false
	hasYarn := false
	hasPnpm := false
	for _, c := range cmds {
		if strings.Contains(c, "npm i -g express@4.18.2") {
			hasNpm = true
		}
		if strings.Contains(c, "yarn global add lodash@4.17.21") {
			hasYarn = true
		}
		if strings.Contains(c, "pnpm add -g typescript@5.1.6") {
			hasPnpm = true
		}
	}
	if !hasNpm {
		t.Errorf("expected npm install, got %v", cmds)
	}
	if !hasYarn {
		t.Errorf("expected yarn install, got %v", cmds)
	}
	if !hasPnpm {
		t.Errorf("expected pnpm install, got %v", cmds)
	}
}

func TestBuildReconcileActions_VersionUpgrade(t *testing.T) {
	pkgs := lockPackages{
		Pip: []string{"flask==2.4.0"},
	}
	cmds := buildReconcileActions(pkgs, nil, []string{"flask==2.3.0"}, nil, nil, nil)
	hasUpgrade := false
	for _, c := range cmds {
		if strings.Contains(c, "pip install flask==2.4.0") {
			hasUpgrade = true
		}
	}
	if !hasUpgrade {
		t.Errorf("expected pip install for version upgrade, got %v", cmds)
	}
}

func TestBuildReconcileActions_BatchedAptRemove(t *testing.T) {
	pkgs := lockPackages{
		Apt: []string{"git=1:2.39.2-1"},
	}
	cmds := buildReconcileActions(pkgs, []string{"git=1:2.39.2-1", "vim=9.0.1-1", "nano=7.2-1"}, nil, nil, nil, nil)
	
	removeCount := 0
	for _, c := range cmds {
		if strings.Contains(c, "apt-get remove") {
			removeCount++
			if !strings.Contains(c, "vim") || !strings.Contains(c, "nano") {
				t.Errorf("expected both vim and nano in one remove call, got: %s", c)
			}
		}
	}
	if removeCount != 1 {
		t.Errorf("expected 1 batched apt-get remove call, got %d", removeCount)
	}
}
