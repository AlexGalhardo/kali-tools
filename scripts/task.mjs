import {spawnSync} from "node:child_process";
import {chmodSync,readdirSync,readFileSync} from "node:fs";
import {fileURLToPath} from "node:url";
import path from "node:path";
const root=fileURLToPath(new URL("../",import.meta.url));
function run(bin,args,cwd=root){const command=process.platform==="win32"&&bin==="npm"?"npm.cmd":bin;const result=spawnSync(command,args,{cwd,stdio:"inherit",shell:process.platform==="win32"&&bin==="npm"});if(result.error)throw result.error;if(result.status!==0)process.exit(result.status??1);}
function goFiles(dir){return readdirSync(dir,{withFileTypes:true}).flatMap(f=>f.isDirectory()?(["node_modules",".git","build"].includes(f.name)?[]:goFiles(path.join(dir,f.name))):f.name.endsWith(".go")?[path.join(dir,f.name)]:[]);}
const task=process.argv[2]??"check";
if(!["check","go","web","race","fuzz","build","dev","lab","workflows","bridge","bindings","scripts"].includes(task))throw Error(`Unknown task: ${task}`);
if(task==="check"||task==="scripts")run("node",["--test","scripts/lab.test.mjs"]);
if(task==="check"||task==="go"){const formatted=spawnSync("gofmt",["-l",...goFiles(root)],{encoding:"utf8"});if(formatted.error)throw formatted.error;if(formatted.status!==0||formatted.stdout.trim()){console.error(formatted.stdout||formatted.stderr);process.exit(1);}run("go",["mod","tidy","-diff"]);run("go",["vet","./..."]);run("go",["test","-timeout","90s","-coverprofile=coverage.out","./..."]);run("golangci-lint",["run","./..."]);run("govulncheck",["./..."]);}
if(task==="check"||task==="web"){run("npm",["run","lint"],path.join(root,"frontend"));run("npm",["run","build"],path.join(root,"frontend"));run("npm",["test"],path.join(root,"frontend"));}
if(task==="race")run("go",["test","-race","-timeout","120s","./..."]);
if(task==="fuzz"){run("go",["test","./internal/capture","-run=^$","-fuzz=FuzzCapture","-fuzztime=10s","-parallel=2"]);run("go",["test","./internal/tools","-run=^$","-fuzz=FuzzNmap","-fuzztime=10s","-parallel=2"]);}
if(task==="build"){run("go",["build","-trimpath","-o",`build/bin/kali-tools${process.platform==="win32"?".exe":""}`,"./cmd/kali-tools"]);run("wails",["build","-tags","desktop"]);}
if(task==="dev")run("wails",["dev","-tags","desktop"]);
if(task==="lab")run("node",["scripts/lab.mjs"]);
if(task==="workflows"){
 const directory=path.join(root,".github/workflows");
 const files=readdirSync(directory).filter(f=>/\.ya?ml$/.test(f));
 if(files.length===0)throw Error("Authorized CI/CD source is missing");
 for(const file of files){const raw=readFileSync(path.join(directory,file),"utf8");
  if(raw.includes("pull_request_target"))throw Error("Privileged pull-request triggers are not allowed");
  for(const match of raw.matchAll(/uses:\s+([^\s]+)/g)){if(!/@[0-9a-f]{40}$/.test(match[1]))throw Error(`Action must be pinned: ${match[1]}`);}
 }
 console.log("Active workflow source and immutable action pins verified; run actionlint for full static validation");
}

if(task==="bridge"){
  if(process.platform!=="win32")throw Error("Native bridge acceptance currently targets Windows only");
  run("go",["test","-race","-tags","desktop,production,nativesmoke","-run","TestNativeBridge","-timeout","45s","."]);
}
if(task==="bindings"){
  run("wails",["generate","module","-tags","desktop"]);
  // Wails writes generated JavaScript as executable on Unix; keep source modes portable.
  for(const file of ["go/main/App.d.ts","go/main/App.js","go/models.ts","runtime/package.json","runtime/runtime.d.ts","runtime/runtime.js"])chmodSync(path.join(root,"frontend/wailsjs",file),0o644);
  run("git",["diff","--exit-code","--","frontend/wailsjs"]);
}
