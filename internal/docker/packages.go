package docker

import (
	"context"

	"coderaft/internal/parallel"
	"coderaft/internal/ui"
)

const yarnGlobalListQuery = `node -e "(async()=>{const cp=require('child_process');function sh(c){try{return cp.execSync(c,{stdio:['ignore','pipe','ignore']}).toString()}catch(e){return ''}}const dir=sh('yarn global dir').trim();if(!dir){process.exit(0)}const fs=require('fs'),path=require('path');const pkgLock=path.join(dir,'package.json');let deps={};try{const pkg=JSON.parse(fs.readFileSync(pkgLock,'utf8'));deps=Object.assign({},pkg.dependencies||{},pkg.devDependencies||{})}catch{}Object.keys(deps).forEach(n=>{let v='';try{const pj=JSON.parse(fs.readFileSync(path.join(dir,'node_modules',n,'package.json'),'utf8'));v=pj.version||''}catch{}if(v)console.log(n+'@'+v)});})();" 2>/dev/null || true`

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
