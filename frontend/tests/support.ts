import { expect, test as base, type Page } from "@playwright/test";
export { expect };
export const test = base.extend<{ connect: (page: Page) => Promise<void> }>({
 connect: async ({}, use, testInfo) => {
  await use(async (page: Page) => {
   if (testInfo.project.name === "http") {
    const launch = new URL(process.env.KT_URL!);
    await page.request.put(new URL("/api/settings",launch).href,{headers:{Authorization:`Bearer ${launch.hash.slice(7)}`},data:{language:"en",retention_days:30}});
    await page.request.post(new URL("/api/legal/accept",launch).href,{headers:{Authorization:`Bearer ${launch.hash.slice(7)}`}});
    await page.request.delete(new URL("/api/history",launch).href,{headers:{Authorization:`Bearer ${launch.hash.slice(7)}`}});
    await page.goto(launch.href);
    await expect(page.locator("header .local-badge")).toHaveText("Connected");
    await expect(page.getByRole("button", {name:/Refresh/})).toBeEnabled();
    return;
   }
await page.addInitScript(()=>{
  const handlers:Record<string,((v:unknown)=>void)[]>={};let settings={language:"en",retention_days:30};let count=100;let cancelDiscovery:(()=>void)|undefined;
  const job={id:"fixture-job",kind:"tcp",state:"completed",created:"2026-10-19T12:00:00Z",summary:"Diagnostic completed"};
  const api={
    Version:async()=>"0.5.0",
    Capabilities:async()=>[{id:"diagnostics",state:"available",detail:"Go diagnostics",provider:"go/net"},{id:"wifi",state:"available",detail:"Windows WLAN",provider:"windows"},{id:"capture_live",state:"permission_required",detail:"Raw IPv4",provider:"windows"}],
    CaptureBackends:async()=>[{id:"capture_live",state:"available",detail:"Synthetic raw IPv4 capture",provider:"fake"},{id:"capture_pktmon",state:"permission_required",detail:"Administrator required",provider:"pktmon"},{id:"capture_npcap",state:"dependency_missing",detail:"Install Npcap yourself",provider:"npcap"}],
    OpenNpcapDownload:async()=>{},
    Interfaces:async()=>[{index:1,name:"Fixture Ethernet",mac:"02:00:00:00:00:01",mtu:1500,up:true,loopback:false,addresses:["192.0.2.10/24"]}],
    Neighbors:async()=>({neighbors:[{address:"192.0.2.1",mac:"02:00:00:00:00:01",family:"ipv4",interface_index:1,interface_name:"Fixture Ethernet",state:"reachable",router:true}],limited:false,provider:"synthetic passive cache"}),
    Inventory:async()=>({adapters:[{index:1,name:"Fixture Ethernet",gateways:["192.0.2.1"],dns:["192.0.2.53"]}],counters:[{id:"1",received:count+=100,sent:count}]}),
    Settings:async()=>settings,SaveSettings:async(v:typeof settings)=>{settings=v;},
    LegalNotice:async()=>({version:"1",accepted:true}),AcceptLegalNotice:async()=>{},
    SaveDiagnosticBundle:async()=>{document.documentElement.dataset.diagnosticBundle="1";return true;},
    StartDiagnostic:async(r:{record_type?:string;kind?:string;redirects?:number;target?:string})=>{document.documentElement.dataset.dnsRecord=r.record_type??"";document.documentElement.dataset.redirects=String(r.redirects??"");const failure=document.documentElement.dataset.diagnosticFailure;const http=r.kind==="http"?{status_code:302,phases:{dns_ms:null,connect_ms:1.2,tls_ms:null,ttfb_ms:2.5,total_ms:4},security_headers:[{name:"Strict-Transport-Security",present:false},{name:"Content-Security-Policy",present:true,value:"default-src 'none'"},{name:"Content-Security-Policy-Report-Only",present:false},{name:"X-Content-Type-Options",present:true,value:"nosniff"},{name:"X-Frame-Options",present:false},{name:"Referrer-Policy",present:false},{name:"Permissions-Policy",present:false},{name:"Cross-Origin-Opener-Policy",present:false},{name:"Cross-Origin-Embedder-Policy",present:false},{name:"Cross-Origin-Resource-Policy",present:false}],redirect:{location:"http://192.0.2.1/next",valid:true,self_loop:false,cross_host:true,downgrade:false},chain_status:"not_requested"}:undefined;setTimeout(()=>handlers.job?.forEach(fn=>fn({schema_version:1,job:{...job,state:failure?"failed":"completed"},result:{kind:r.kind??"tcp",target:r.target??"localhost",status:failure?"failed":"completed",error_code:failure,elapsed_ms:1,values:failure?[]:r.kind==="http"?["302 Found","HTTP/1.1"]:["connected"],provider:"fixture",http}})),100);return {...job,state:"queued"};},CancelJob:async(id:string)=>{if(id==="fixture-discovery")cancelDiscovery?.();return true;},
    History:async()=>[job],ClearHistory:async()=>{},CurrentWiFi:async()=>({profile:"Fixture",ssid:"Fixture Network",bssid:"02:00:00:00:00:02",signal:80,authentication:7,state:"connected"}),RevealWiFi:async()=>({state:"available",value:"synthetic-test-key"}),
    Tools:async()=>[{name:"tshark",state:"dependency_missing",version:"",path:""}],Discover:async()=>{
      if(document.documentElement.dataset.cancelDiscovery){
        const running={...job,id:"fixture-discovery",kind:"discovery",state:"running"};
        handlers.job?.forEach(fn=>fn({schema_version:1,job:running}));
        return new Promise<{results:{host:string;port:number;state:string;rtt_ms:number;host_state:string}[];truncated:boolean}>(resolve=>{cancelDiscovery=()=>{handlers.job?.forEach(fn=>fn({schema_version:1,job:{...running,state:"cancelled"}}));resolve({results:[],truncated:false});};});
      }
      return {results:[{host:"192.0.2.1",port:80,state:"open",rtt_ms:1.2,host_state:"up"}],truncated:false};
    },
    SaveCapture:async(format:string)=>{document.documentElement.dataset.savedCapture=format;return true;},ImportCapture:async()=>({packets:[{number:1,timestamp:"2026-10-19T12:00:00Z",length:64,source:"192.0.2.1",destination:"192.0.2.2",protocol:"DNS",info:"example.test"}],limited:false,format:"pcap"}),StartCapture:async()=>{},StopCapture:async()=>{},
    DiscoverLocal:async()=>({responders:[],limited:false,out_of_scope:0,malformed:0,errors:[]}),
    QuerySNMP:async()=>({target:"192.0.2.1",system:{sysName:"kali-tools-lab"},interfaces:[],limited:false})
  };
  window.addEventListener("fixture-capture", event=>handlers.capture?.forEach(fn=>fn((event as CustomEvent).detail)));
  Object.assign(window,{go:{wailsui:{Bindings:api}},runtime:{EventsOn:(name:string,fn:(v:unknown)=>void)=>{handlers[name]??=[];handlers[name].push(fn);return()=>{handlers[name]=handlers[name].filter(f=>f!==fn);};}}});
});
   await page.goto("/"); await expect(page.getByText("Fixture Ethernet")).toBeVisible();
  });
 }
});
export async function fillDiagnosticTarget(page: Page) {
 if (test.info().project.name !== "http") return;
 await page.getByLabel("Target",{exact:true}).fill("127.0.0.1");
 await page.getByLabel("Port",{exact:true}).fill(process.env.KT_FIXTURE_TCP!.split(":").at(-1)!);
}
export async function importCapture(page: Page) {
 if (test.info().project.name !== "http") { await page.getByRole("button",{name:"Import PCAP / PCAPNG"}).click(); return; }
 const chooser = page.waitForEvent("filechooser");
 await page.getByRole("button",{name:"Import PCAP / PCAPNG"}).click();
 await (await chooser).setFiles(process.env.KT_FIXTURE_PCAP!);
}
