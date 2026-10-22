import {defineConfig} from "@playwright/test";
const port=Number(process.env.PW_PORT??4173);
const httpPort=Number(process.env.PW_HTTP_PORT??4174);
for(const [name,value] of [["PW_PORT",port],["PW_HTTP_PORT",httpPort]] as const)if(!Number.isInteger(value)||value<1024||value>65535)throw Error(`${name} must be an integer from 1024 to 65535`);
const url=`http://127.0.0.1:${port}`;
export default defineConfig({
 testDir:"./tests",fullyParallel:true,retries:0,
 use:{headless:true},
 projects:[
  {name:"wails-fake",use:{baseURL:url},grepInvert:/@http-only/},
  {name:"http",use:{baseURL:`http://127.0.0.1:${httpPort}`},grepInvert:/@wails-only/,fullyParallel:false},
 ],
 workers:1,
 globalSetup:"./tests/http-server.setup.ts",
 webServer:{command:`npm run dev -- --port ${port} --strictPort`,url,reuseExistingServer:false,timeout:30000},reporter:"list"
});
