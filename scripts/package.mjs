import {spawnSync} from "node:child_process";
import {readFileSync,writeFileSync,readdirSync,mkdirSync,cpSync,mkdtempSync} from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=fileURLToPath(new URL("../",import.meta.url));
const platform={win32:"windows",linux:"linux",darwin:"macos"}[process.platform];
if(!platform)throw Error("Unsupported packaging platform");
const version=JSON.parse(readFileSync(path.join(root,"frontend/package.json"),"utf8")).version;
if(!/^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$/.test(version))throw Error("Invalid version");
const output=path.join(root,"build/release");mkdirSync(output,{recursive:true});
const stage=mkdtempSync(path.join(root,"build/staging-"));
function run(bin,args){const r=spawnSync(bin,args,{cwd:root,stdio:"inherit",timeout:120000});if(r.error)throw r.error;if(r.status!==0)throw Error(`${bin} failed (${r.status})`);}
run("node",["scripts/notices.mjs"]);
for(const file of ["LICENSE.md","README.md"]){cpSync(path.join(root,file),path.join(stage,file));}
for(const file of ["THIRD_PARTY_NOTICES.txt","dependencies.json"]){cpSync(path.join(root,"build/notices",file),path.join(stage,file));}
const suffix=process.platform==="win32"?".exe":"";
cpSync(path.join(root,"build/bin/kali-tools"+suffix),path.join(stage,"kali-tools"+suffix));
if(process.platform==="darwin"){
 const apps=readdirSync(path.join(root,"build/bin")).filter(x=>x.endsWith(".app"));
 if(apps.length!==1)throw Error("Expected one native app bundle");
 cpSync(path.join(root,"build/bin",apps[0]),path.join(stage,apps[0]),{recursive:true});
}else{cpSync(path.join(root,"build/bin/kali-tools-desktop"+suffix),path.join(stage,"kali-tools-desktop"+suffix));}
const commit=spawnSync("git",["rev-parse","HEAD"],{cwd:root,encoding:"utf8",timeout:10000});if(commit.status!==0)throw Error("Cannot identify source commit");
const status=spawnSync("git",["-c","core.fileMode=false","status","--porcelain"],{cwd:root,encoding:"utf8",timeout:10000});if(status.status!==0)throw Error("Cannot inspect source state");
writeFileSync(path.join(stage,"BUILD.json"),JSON.stringify({version,source_commit:commit.stdout.trim(),source_dirty:Boolean(status.stdout.trim()),platform,architecture:process.arch,signed:false,qualification:"development preview; see PLAN.md"},null,2)+"\n");
const stem=`kali-tools-${version}-${platform}-${process.arch}`;
const archive=path.join(output,stem+(process.platform==="linux"?".tar.gz":".zip"));
if(process.platform==="darwin")run("ditto",["-c","-k",stage,archive]);
else if(process.platform==="win32")run(path.join(process.env.SystemRoot??"C:/Windows","System32","tar.exe"),["-a","-c","-f",archive,"-C",stage,"."]);
else run("tar",["-czf",archive,"-C",stage,"."]);
console.log(archive);
