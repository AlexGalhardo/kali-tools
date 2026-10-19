import {spawnSync} from "node:child_process";
import {fileURLToPath} from "node:url";
import {mkdirSync} from "node:fs";
import path from "node:path";
const root=fileURLToPath(new URL("../",import.meta.url));
const [platform,action="setup"]=process.argv.slice(2);
const supported={windows:"win32",linux:"linux",darwin:"darwin"};
const actions=["setup","doctor","test","build","dev","help"];
if(!Object.hasOwn(supported,platform)||!actions.includes(action)){console.error("Usage: setup-<windows|linux|darwin>.sh [setup|doctor|test|build|dev|help]");process.exit(2);}
if(action==="help"){console.log("setup: install pinned project tools and frontend dependencies\ndoctor: inspect native prerequisites\ntest: Go lint/vet/unit/race/vulnerability/fuzz checks and frontend workflows\nbuild: build CLI and native desktop\ndev: open the Wails development app\nDocker is separate: use setup-<os>-docker.sh.");process.exit(0);}
if(process.platform!==supported[platform]){console.error(`This script targets ${platform}; current host is ${process.platform}. Use the matching script.`);process.exit(2);}
const bin=path.join(root,".tools","bin");mkdirSync(bin,{recursive:true});process.env.PATH=bin+path.delimiter+process.env.PATH;
function run(command,args,cwd=root){const result=spawnSync(command,args,{cwd,stdio:"inherit",shell:process.platform==="win32"&&command==="npm"});if(result.error){console.error(`${command}: ${result.error.message}. See README prerequisites.`);process.exit(1);}if(result.status!==0)process.exit(result.status??1);}
const tags=platform==="linux"?"desktop,webkit2_41":"desktop";
if(action==="setup"){
  run("go",["version"]);run("node",["--version"]);run("npm",["--version"]);
  const previous=process.env.GOBIN;process.env.GOBIN=bin;
  for(const tool of ["github.com/wailsapp/wails/v2/cmd/wails@v2.17.0","github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0","golang.org/x/vuln/cmd/govulncheck@v1.9.0"])run("go",["install",tool]);
  if(previous===undefined)delete process.env.GOBIN;else process.env.GOBIN=previous;
  run("go",["mod","download"]);run("npm",["ci"],path.join(root,"frontend"));run("npm",["exec","--","playwright","install","chromium"],path.join(root,"frontend"));run("wails",["doctor"]);console.log("Setup complete. Next: run the same script with test, build or dev.");
}
if(action==="doctor"){run("go",["version"]);run("node",["--version"]);run("wails",["doctor"]);}
if(action==="test"){for(const task of ["scripts","go","race","fuzz","web","workflows"])run("node",["scripts/task.mjs",task]);run("node",["scripts/markdown.mjs"]);}
if(action==="build"){run("go",["build","-trimpath","-o",`build/bin/kali-tools${platform==="windows"?".exe":""}`,"./cmd/kali-tools"]);run("wails",["build","-tags",tags]);}
if(action==="dev")run("wails",["dev","-tags",tags]);
