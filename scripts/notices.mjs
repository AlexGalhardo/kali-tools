import {spawnSync} from "node:child_process";
import {readFileSync,writeFileSync,readdirSync,mkdirSync,statSync,existsSync} from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=fileURLToPath(new URL("../",import.meta.url));
const output=path.join(root,"build/notices");
mkdirSync(output,{recursive:true});
function command(bin,args){const r=spawnSync(bin,args,{cwd:root,encoding:"utf8",timeout:120000,maxBuffer:8<<20});if(r.error)throw r.error;if(r.status!==0)throw Error(r.stderr||`${bin} failed`);return r.stdout.trim();}
function licenses(directory){
 const found=[];
 for(const entry of readdirSync(directory,{withFileTypes:true})){
  if(entry.isFile() && /^(licen[sc]e|copying|copyright|notice)([.\-_]|$)/i.test(entry.name)){
   const file=path.join(directory,entry.name);
   if(statSync(file).size>1<<20)throw Error(`Oversized notice: ${entry.name}`);
   found.push({name:entry.name,text:readFileSync(file,"utf8")});
  }
 }
 return found.sort((a,b)=>a.name.localeCompare(b.name));
}
const sections=[];
const inventory={go:[],frontend:[],vendored:[]};
function append(label,directory){const found=licenses(directory);if(!found.length)throw Error(`Missing license text for ${label}`);sections.push(`\n${"=".repeat(72)}\n${label}\n${"=".repeat(72)}\n`+found.map(x=>`${x.name}\n\n${x.text}`).join("\n\n"));}
append("Go standard library",command("go",["env","GOROOT"]));
const moduleCache=command("go",["env","GOMODCACHE"]);
const escapeModule=(value)=>value.replace(/[A-Z]/g,c=>"!"+c.toLowerCase());
const modules=command("go",["list","-m","-f","{{.Path}}\t{{.Version}}\t{{.Dir}}","all"]).split(/\r?\n/);
for(const line of modules){const [module,version,listedDirectory]=line.split("\t");if(!version)continue;const directory=listedDirectory||path.join(moduleCache,escapeModule(module)+"@"+escapeModule(version));if(!existsSync(directory))throw Error(`Download ${module} with go mod download all before packaging`);inventory.go.push({module,version});append(`${module} ${version}`,directory);}
const lock=JSON.parse(readFileSync(path.join(root,"frontend/package-lock.json"),"utf8"));
for(const [location,pkg] of Object.entries(lock.packages)){
 if(!location || pkg.dev)continue;
 const directory=path.join(root,"frontend",location);
 const metadata=JSON.parse(readFileSync(path.join(directory,"package.json"),"utf8"));
 inventory.frontend.push({name:metadata.name,version:metadata.version,license:metadata.license??pkg.license??"See notices"});
 append(`${metadata.name} ${metadata.version}`,directory);
}
for(const skill of ["impeccable","ponytail"]){const directory=path.join(root,".agents/skills",skill);inventory.vendored.push({name:skill,source:".agents/skills/sources.json"});append(`Vendored development skill: ${skill}`,directory);}
const karpathy=path.join(root,"docs/licenses/karpathy-guidelines.txt");
const fontLicense=path.join(root,"docs/licenses/jetbrains-mono.txt");
sections.push("\nBundled font: JetBrains Mono 2.304 (OFL-1.1)\n"+readFileSync(fontLicense,"utf8"));
inventory.vendored.push({name:"JetBrains Mono",version:"2.304",license:"OFL-1.1",source:"https://github.com/JetBrains/JetBrainsMono/tree/v2.304"});
if(existsSync(karpathy)){sections.push("\nVendored development skill: karpathy-guidelines\n"+readFileSync(karpathy,"utf8"));inventory.vendored.push({name:"karpathy-guidelines",source:".agents/skills/sources.json"});}
writeFileSync(path.join(output,"THIRD_PARTY_NOTICES.txt"),"kali-tools third-party notices\n\nProject code is MIT licensed. Dependencies retain their own licenses.\nNmap and TShark are not included.\n"+sections.join("\n"));
writeFileSync(path.join(output,"dependencies.json"),JSON.stringify(inventory,null,2)+"\n");
console.log(`Collected notices: ${inventory.go.length} Go modules, ${inventory.frontend.length} frontend packages, ${inventory.vendored.length} vendored skills`);
