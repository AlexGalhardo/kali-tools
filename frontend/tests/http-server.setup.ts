import { spawn, execFileSync } from "node:child_process";
import path from "node:path";
import type { FullConfig } from "@playwright/test";

export default async function setup(_config: FullConfig) {
 const port = Number(process.env.PW_HTTP_PORT ?? 4174);
 if (!Number.isInteger(port) || port < 1024 || port > 65535) throw new Error("PW_HTTP_PORT must be an integer from 1024 to 65535");
 const child = spawn("go", ["run", "-tags", "e2e", "./tests/e2e/httpserver", "--assets", "frontend/dist", "--listen", `127.0.0.1:${port}`], {cwd:path.resolve(process.cwd(),".."),windowsHide:true,detached:process.platform!=="win32",stdio:["ignore","pipe","pipe"]});
 const stop = () => {
  if (child.exitCode !== null || !child.pid) return;
  if (process.platform === "win32") {
   try { execFileSync("taskkill",["/PID",String(child.pid),"/T","/F"],{windowsHide:true,stdio:"ignore",timeout:5000}); } catch { child.kill(); }
  } else { try { process.kill(-child.pid,"SIGTERM"); } catch { child.kill("SIGTERM"); } }
 };
 try {
  await new Promise<void>((resolve,reject)=>{
   let output="",errors="";
   const timer=setTimeout(()=>reject(new Error("HTTP fixture startup timed out")),60000);
   const fail=(error:Error)=>{clearTimeout(timer);reject(error);};
   child.once("error",fail);
   child.once("exit",code=>fail(new Error(`HTTP fixture exited ${code}: ${errors}`)));
   child.stderr.on("data",chunk=>{errors=(errors+String(chunk)).slice(-16384);});
   child.stdout.on("data",chunk=>{
    output+=String(chunk);
    if(output.length>65536){fail(new Error("HTTP fixture output limit exceeded"));return;}
    if(!output.includes("KT_READY\n"))return;
    for(const key of ["KT_URL","KT_FIXTURE_TCP","KT_FIXTURE_PCAP"]) {
     const value=output.split("\n").find(line=>line.startsWith(key+"="))?.slice(key.length+1).trim();
     if(!value){fail(new Error(`HTTP fixture missing ${key}`));return;}
     process.env[key]=value;
    }
    clearTimeout(timer);resolve();
   });
  });
 } catch(error) {stop();throw error;}
 return async()=>{stop();};
}
