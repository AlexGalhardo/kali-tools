import {defineConfig} from "@playwright/test";
const port=Number(process.env.PW_PORT??4173);
if(!Number.isInteger(port)||port<1024||port>65535)throw Error("PW_PORT must be an integer from 1024 to 65535");
const url=`http://127.0.0.1:${port}`;
export default defineConfig({testDir:"./tests",fullyParallel:true,retries:0,use:{baseURL:url,headless:true},webServer:{command:`npm run dev -- --port ${port} --strictPort`,url,reuseExistingServer:false,timeout:30000},reporter:"list"});
