package docker

import (
	"context"

	"coderaft/internal/parallel"
	"coderaft/internal/ui"
)

const yarnGlobalListQuery = `node -e "(async()=>{const cp=require('child_process');function sh(c){try{return cp.execSync(c,{stdio:['ignore','pipe','ignore']}).toString()}catch(e){return ''}}const dir=sh('yarn global dir').trim();if(!dir){process.exit(0)}const fs=require('fs'),path=require('path');const pkgLock=path.join(dir,'package.json');let deps={};try{const pkg=JSON.parse(fs.readFileSync(pkgLock,'utf8'));deps=Object.assign({},pkg.dependencies||{},pkg.devDependencies||{})}catch{}Object.keys(deps).forEach(n=>{let v='';try{const pj=JSON.parse(fs.readFileSync(path.join(dir,'node_modules',n,'package.json'),'utf8'));v=pj.version||''}catch{}if(v)console.log(n+'@'+v)});})();" 2>/dev/null || true`

// PackageLists holds all package manager query results
type PackageLists struct {
	Apt      []string
	Apk      []string
	Dnf      []string
	Pacman   []string
	Brew     []string
	Snap     []string
	Pip      []string
	Pipx     []string
	Conda    []string
	Poetry   []string
	Npm      []string
	Yarn     []string
	Pnpm     []string
	Bun      []string
	Cargo    []string
	Go       []string
	Gem      []string
	Composer []string
}

func (c *Client) QueryPackagesParallel(islandName string) (aptList, pipList, npmList, yarnList, pnpmList []string) {
	config := parallel.LoadConfig()
	if !config.EnableParallel {

		return c.queryPackagesSequential(islandName)
	}

	executor := parallel.NewPackageQueryExecutorWithSDK(islandName, c.SDKExecFunc())

	packageLists, err := executor.QueryAllPackages()
	if err != nil {
		ui.Warning("parallel package query failed, falling back to sequential: %v", err)

		return c.queryPackagesSequential(islandName)
	}

	return packageLists["apt"], packageLists["pip"], packageLists["npm"], packageLists["yarn"], packageLists["pnpm"]
}

// QueryAllPackages queries all supported package managers and returns a comprehensive PackageLists struct
func (c *Client) QueryAllPackages(islandName string) *PackageLists {
	config := parallel.LoadConfig()
	if !config.EnableParallel {
		return c.queryAllPackagesSequential(islandName)
	}

	executor := parallel.NewPackageQueryExecutorWithSDK(islandName, c.SDKExecFunc())
	packageLists, err := executor.QueryAllPackagesExtended()
	if err != nil {
		ui.Warning("parallel package query failed, falling back to sequential: %v", err)
		return c.queryAllPackagesSequential(islandName)
	}

	return &PackageLists{
		Apt:      packageLists["apt"],
		Apk:      packageLists["apk"],
		Dnf:      packageLists["dnf"],
		Pacman:   packageLists["pacman"],
		Brew:     packageLists["brew"],
		Snap:     packageLists["snap"],
		Pip:      packageLists["pip"],
		Pipx:     packageLists["pipx"],
		Conda:    packageLists["conda"],
		Poetry:   packageLists["poetry"],
		Npm:      packageLists["npm"],
		Yarn:     packageLists["yarn"],
		Pnpm:     packageLists["pnpm"],
		Bun:      packageLists["bun"],
		Cargo:    packageLists["cargo"],
		Go:       packageLists["go"],
		Gem:      packageLists["gem"],
		Composer: packageLists["composer"],
	}
}

func (c *Client) queryPackagesSequential(islandName string) (aptList, pipList, npmList, yarnList, pnpmList []string) {
	type query struct {
		name    string
		command string
		jsonPkg bool
	}

	queries := []query{
		{"apt", `dpkg-query -W -f='${Package}=${Version}\n' $(apt-mark showmanual 2>/dev/null || true) 2>/dev/null | sort`, false},
		{"pip", "python3 -m pip freeze 2>/dev/null || pip3 freeze 2>/dev/null || true", false},
		{"npm", "npm list -g --depth=0 --json 2>/dev/null || true", true},
		{"yarn", yarnGlobalListQuery, false},
		{"pnpm", "pnpm ls -g --depth=0 --json 2>/dev/null || true", true},
	}

	results := make(map[string][]string)
	ctx := context.Background()

	for _, q := range queries {
		result, err := c.sdk.containerExec(ctx, islandName, []string{"bash", "-c", q.command}, false)
		if err != nil {
			ui.Warning("sequential query for %s failed: %v", q.name, err)
			continue
		}

		if q.jsonPkg {
			results[q.name] = parallel.ParseJSONPackageList(result.Stdout)
		} else {
			results[q.name] = parallel.ParseLineList(result.Stdout)
		}
	}

	return results["apt"], results["pip"], results["npm"], results["yarn"], results["pnpm"]
}

func (c *Client) queryAllPackagesSequential(islandName string) *PackageLists {
	type query struct {
		name    string
		command string
		jsonPkg bool
	}

	queries := []query{
		// System package managers
		{"apt", `dpkg-query -W -f='${Package}=${Version}\n' $(apt-mark showmanual 2>/dev/null || true) 2>/dev/null | sort`, false},
		{"apk", `apk info -v 2>/dev/null | sed 's/-\([0-9]\)/=\1/' | sort || true`, false},
		{"dnf", `dnf list installed 2>/dev/null | tail -n +2 | awk '{print $1"="$2}' | sort || true`, false},
		{"pacman", `pacman -Qe 2>/dev/null | awk '{print $1"="$2}' | sort || true`, false},
		{"brew", `brew list --versions 2>/dev/null | awk '{print $1"="$2}' | sort || true`, false},
		{"snap", `snap list 2>/dev/null | tail -n +2 | awk '{print $1"="$2}' | sort || true`, false},

		// Python
		{"pip", "python3 -m pip freeze 2>/dev/null || pip3 freeze 2>/dev/null || true", false},
		{"pipx", `pipx list --json 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin).get('venvs',{}); [print(f'{k}=={v[\"metadata\"][\"main_package\"][\"package_version\"]}') for k,v in d.items()]" 2>/dev/null || true`, false},
		{"conda", `conda list --export 2>/dev/null | grep -v "^#" | sed 's/==/=/' | sort || true`, false},
		{"poetry", `poetry show 2>/dev/null | awk '{print $1"=="$2}' | sort || true`, false},

		// Node.js
		{"npm", "npm list -g --depth=0 --json 2>/dev/null || true", true},
		{"yarn", yarnGlobalListQuery, false},
		{"pnpm", "pnpm ls -g --depth=0 --json 2>/dev/null || true", true},
		{"bun", `bun pm ls -g 2>/dev/null | grep -E "^├|^└" | sed 's/[├└─ ]*//' | sort || true`, false},

		// Language-specific
		{"cargo", `cargo install --list 2>/dev/null | grep -E "^[a-z]" | awk '{print $1"="$2}' | tr -d ':' | sort || true`, false},
		{"go", `ls $(go env GOPATH 2>/dev/null)/bin 2>/dev/null | while read f; do echo "$f"; done | sort || true`, false},
		{"gem", `gem list --local 2>/dev/null | sed 's/ (/=/;s/)//' | sort || true`, false},
		{"composer", `composer global show 2>/dev/null | awk '{print $1"="$2}' | sort || true`, false},
	}

	results := make(map[string][]string)
	ctx := context.Background()

	for _, q := range queries {
		result, err := c.sdk.containerExec(ctx, islandName, []string{"bash", "-c", q.command}, false)
		if err != nil {
			continue
		}

		if q.jsonPkg {
			results[q.name] = parallel.ParseJSONPackageList(result.Stdout)
		} else {
			results[q.name] = parallel.ParseLineList(result.Stdout)
		}
	}

	return &PackageLists{
		Apt:      results["apt"],
		Apk:      results["apk"],
		Dnf:      results["dnf"],
		Pacman:   results["pacman"],
		Brew:     results["brew"],
		Snap:     results["snap"],
		Pip:      results["pip"],
		Pipx:     results["pipx"],
		Conda:    results["conda"],
		Poetry:   results["poetry"],
		Npm:      results["npm"],
		Yarn:     results["yarn"],
		Pnpm:     results["pnpm"],
		Bun:      results["bun"],
		Cargo:    results["cargo"],
		Go:       results["go"],
		Gem:      results["gem"],
		Composer: results["composer"],
	}
}
