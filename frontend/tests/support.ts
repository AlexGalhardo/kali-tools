import { expect, test as base, type Page } from "@playwright/test";
export { expect };
export const test = base.extend<{ connect: (page: Page) => Promise<void> }>({
 connect: async ({}, use, testInfo) => {
  await use(async (page: Page) => {
   if (testInfo.project.name === "http") {
    const launch = new URL(process.env.KT_URL!);
    await page.request.put(new URL("/api/settings",launch).href,{headers:{Authorization:`Bearer ${launch.hash.slice(7)}`},data:{language:"en",retention_days:30}});
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
    StartDiagnostic:async(r:{record_type?:string})=>{document.documentElement.dataset.dnsRecord=r.record_type;const failure=document.documentElement.dataset.diagnosticFailure;setTimeout(()=>handlers.job?.forEach(fn=>fn({schema_version:1,job:{...job,state:failure?"failed":"completed"},result:{kind:"tcp",target:"localhost",status:failure?"failed":"completed",error_code:failure,elapsed_ms:1,values:failure?[]:["connected"],provider:"fixture"}})),100);return {...job,state:"queued"};},CancelJob:async(id:string)=>{if(id==="fixture-discovery")cancelDiscovery?.();return true;},
    History:async()=>[job],ClearHistory:async()=>{},CurrentWiFi:async()=>({profile:"Fixture",ssid:"Fixture Network",bssid:"02:00:00:00:00:02",signal:80,authentication:7,state:"connected"}),RevealWiFi:async()=>({state:"available",value:"synthetic-test-key"}),
    Tools:async()=>[{name:"nmap",state:"dependency_missing",version:"",path:""}],Discover:async()=>{
      if(document.documentElement.dataset.cancelDiscovery){
        const running={...job,id:"fixture-discovery",kind:"discovery",state:"running"};
        handlers.job?.forEach(fn=>fn({schema_version:1,job:running}));
        return new Promise<{host:string;port:number;state:string}[]>(resolve=>{cancelDiscovery=()=>{handlers.job?.forEach(fn=>fn({schema_version:1,job:{...running,state:"cancelled"}}));resolve([]);};});
      }
      return [{host:"192.0.2.1",port:80,state:"open"}];
    },
    SaveCapture:async(format:string)=>{document.documentElement.dataset.savedCapture=format;return true;},ImportCapture:async()=>({packets:[{number:1,timestamp:"2026-10-19T12:00:00Z",length:64,source:"192.0.2.1",destination:"192.0.2.2",protocol:"DNS",info:"example.test"}],limited:false,format:"pcap"}),StartCapture:async()=>{},StopCapture:async()=>{}
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
