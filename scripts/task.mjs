import {spawnSync} from "node:child_process";
import {readdirSync,readFileSync,existsSync} from "node:fs";
import {fileURLToPath} from "node:url";
import path from "node:path";
const root=fileURLToPath(new URL("../",import.meta.url));
function run(bin,args,cwd=root){const command=process.platform==="win32"&&bin==="npm"?"npm.cmd":bin;const result=spawnSync(command,args,{cwd,stdio:"inherit",shell:process.platform==="win32"&&bin==="npm"});if(result.error)throw result.error;if(result.status!==0)process.exit(result.status??1);}
function goFiles(dir){return readdirSync(dir,{withFileTypes:true}).flatMap(f=>f.isDirectory()?(["node_modules",".git","build"].includes(f.name)?[]:goFiles(path.join(dir,f.name))):f.name.endsWith(".go")?[path.join(dir,f.name)]:[]);}
const task=process.argv[2]??"check";
if(task==="check"||task==="go"){const formatted=spawnSync("gofmt",["-l",...goFiles(root)],{encoding:"utf8"});if(formatted.error)throw formatted.error;if(formatted.status!==0||formatted.stdout.trim()){console.error(formatted.stdout||formatted.stderr);process.exit(1);}run("go",["mod","tidy","-diff"]);run("go",["vet","./..."]);run("go",["test","-timeout","90s","-coverprofile=coverage.out","./..."]);run("golangci-lint",["run","./..."]);run("govulncheck",["./..."]);}
if(task==="check"||task==="web"){run("npm",["run","lint"],path.join(root,"frontend"));run("npm",["run","build"],path.join(root,"frontend"));run("npm",["test"],path.join(root,"frontend"));}
if(task==="race")run("go",["test","-race","-timeout","120s","./..."]);
if(task==="fuzz"){run("go",["test","./internal/capture","-run=^$","-fuzz=FuzzCapture","-fuzztime=10s","-parallel=2"]);run("go",["test","./internal/tools","-run=^$","-fuzz=FuzzNmap","-fuzztime=10s","-parallel=2"]);}
if(task==="build"){run("go",["build","-trimpath","-o",`build/bin/hacker-tools${process.platform==="win32"?".exe":""}`,"./cmd/hacker-tools"]);run("wails",["build","-tags","desktop"]);}
if(task==="dev")run("wails",["dev","-tags","desktop"]);
if(task==="lab")run("docker",["compose","-f","docker/compose.yml","up","--build","--abort-on-container-exit","--exit-code-from","test"]);
if(task==="workflows"){const active=path.join(root,".github/workflows");if(existsSync(active)&&readdirSync(active).some(f=>/\.ya?ml$/.test(f)))throw Error("Workflows must remain dormant");for(const file of readdirSync(path.join(root,"docs/ci"))){if(!file.endsWith(".yml.disabled"))continue;const raw=readFileSync(path.join(root,"docs/ci",file),"utf8");for(const match of raw.matchAll(/uses:\s+([^\s]+)/g)){if(!/@[0-9a-f]{40}$/.test(match[1]))throw Error(`Action must be pinned: ${match[1]}`);}}console.log("Dormant workflow location and immutable action pins verified");}
