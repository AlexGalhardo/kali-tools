import {readFileSync} from "node:fs";
import path from "node:path";
import {fileURLToPath} from "node:url";

// SemVer 2.0.0, including numeric prerelease identifiers without leading zeroes.
export const semver=/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?(?![\s\S])/;

export function checkRelease(root){
 const read=file=>readFileSync(path.join(root,file),"utf8");
 const version=JSON.parse(read("frontend/package.json")).version;
 if(!semver.test(version))throw Error("Release version must follow SemVer 2.0.0");
 const lock=JSON.parse(read("frontend/package-lock.json"));
 const goVersion=read("internal/domain/domain.go").match(/const Version = "([^"]+)"/)?.[1];
 if([lock.version,lock.packages[""].version,goVersion].some(value=>value!==version))throw Error("Release versions are inconsistent");
 if(JSON.parse(read("wails.json")).info.productVersion!==version.split(/[-+]/)[0])throw Error("Wails product version is inconsistent");
 const changelog=read("CHANGELOG.md");
 if(!changelog.includes("## [Unreleased]")||!changelog.split(/\r?\n/).some(line=>line.startsWith(`## [${version}] - `)&&/^## \[[^\]]+\] - \d{4}-\d{2}-\d{2}$/.test(line)))throw Error("Changelog needs Unreleased and a dated release entry");
 const groups=[...changelog.matchAll(/^### (.+)$/gm)].map(match=>match[1].trim());
 if(groups.some(group=>!["Added","Changed","Deprecated","Removed","Fixed","Security"].includes(group)))throw Error("Changelog groups must follow Keep a Changelog 1.1.0");
 if(!read(`docs/releases/${version}.md`).startsWith(`# kali-tools ${version}`))throw Error("Release notes version is inconsistent");
 return version;
}

if(process.argv[1]&&path.resolve(process.argv[1])===fileURLToPath(import.meta.url)){
 console.log(`Release policy passed: ${checkRelease(fileURLToPath(new URL("../",import.meta.url)))}`);
}
