import {readdirSync,readFileSync} from "node:fs";
import {fileURLToPath} from "node:url";
import {lint} from "../frontend/node_modules/markdownlint/lib/exports-sync.mjs";
const root=fileURLToPath(new URL("../",import.meta.url));
function files(dir,recursive){return readdirSync(dir,{withFileTypes:true}).flatMap(f=>f.isDirectory()?(recursive?files(dir+"/"+f.name,true):[]):f.name.endsWith(".md")?[dir+"/"+f.name]:[]);}
const result=lint({files:[...files(root,false),...files(root+"/docs",true)],config:JSON.parse(readFileSync(new URL("../.markdownlint-cli2.jsonc",import.meta.url),"utf8")).config});
const failures=Object.entries(result).flatMap(([file,issues])=>issues.map(issue=>`${file}:${issue.lineNumber} ${issue.ruleNames[0]} ${issue.ruleDescription}`));if(failures.length){console.error(failures.join("\n"));process.exit(1);}console.log("Markdown checks passed");
