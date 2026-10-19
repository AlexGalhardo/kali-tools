import {spawnSync} from "node:child_process";
import {readFileSync,writeFileSync,readdirSync,cpSync} from "node:fs";
import {createHash} from "node:crypto";
import path from "node:path";
import {fileURLToPath} from "node:url";
const root=fileURLToPath(new URL("../",import.meta.url));
const directory=path.join(root,"build/release");
const version=JSON.parse(readFileSync(path.join(root,"frontend/package.json"),"utf8")).version;
if(!/^[0-9]+\.[0-9]+\.[0-9]+-[0-9A-Za-z.-]+$/.test(version))throw Error("Only development prereleases are automated");
const tag="v"+version;
const repo="AlexGalhardo/kali-tools";
function run(bin,args){const r=spawnSync(bin,args,{cwd:root,encoding:"utf8",timeout:120000,maxBuffer:2<<20});if(r.error)throw r.error;if(r.status!==0)throw Error(r.stderr||`${bin} failed`);return r.stdout.trim();}
const sha=run("git",["rev-parse","HEAD"]);
run("git",["archive","--format=tar.gz","HEAD","-o",path.join(directory,`kali-tools-${version}-source.tar.gz`)]);
cpSync(path.join(root,"LICENSE.md"),path.join(directory,"LICENSE.md"));
for(const file of ["THIRD_PARTY_NOTICES.txt","dependencies.json"]){cpSync(path.join(root,"build/notices",file),path.join(directory,file));}
const assets=readdirSync(directory).filter(f=>f!=="SHA256SUMS").sort();
for(const target of ["windows-x64.zip","linux-x64.tar.gz","macos-x64.zip","macos-arm64.zip"]){if(!assets.some(f=>f.endsWith(target)))throw Error(`Missing native asset: ${target}`);}
writeFileSync(path.join(directory,"SHA256SUMS"),assets.map(file=>createHash("sha256").update(readFileSync(path.join(directory,file))).digest("hex")+"  "+file).join("\n")+"\n");
const existing=JSON.parse(run("gh",["api",`repos/${repo}/releases?per_page=100`]));
const release=existing.find(r=>r.tag_name===tag);
if(release && !release.draft)throw Error("Refusing to alter a published release");
const notes=path.join(root,"docs/releases",version+".md");
if(release){run("gh",["release","edit",tag,"--repo",repo,"--draft","--prerelease","--target",sha,"--notes-file",notes]);}
else{run("gh",["release","create",tag,"--repo",repo,"--draft","--prerelease","--target",sha,"--title",`kali-tools ${version} ? development preview`,"--notes-file",notes]);}
run("gh",["release","upload",tag,"--repo",repo,"--clobber",...assets.concat("SHA256SUMS").map(file=>path.join(directory,file))]);
console.log(run("gh",["release","view",tag,"--repo",repo,"--json","url,isDraft,targetCommitish"]));
