import {expect,test,type Page} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
async function connect(page:Page){await page.addInitScript(()=>{
  const handlers:Record<string,((v:unknown)=>void)[]>={};let settings={language:"en",retention_days:30};let count=100;let cancelDiscovery:(()=>void)|undefined;
  const job={id:"fixture-job",kind:"tcp",state:"completed",created:"2026-10-19T12:00:00Z",summary:"Diagnostic completed"};
  const api={
    Version:async()=>"0.3.0",
    Capabilities:async()=>[{id:"diagnostics",state:"available",detail:"Go diagnostics",provider:"go/net"},{id:"wifi",state:"available",detail:"Windows WLAN",provider:"windows"},{id:"capture_live",state:"permission_required",detail:"Raw IPv4",provider:"windows"}],
    Interfaces:async()=>[{index:1,name:"Fixture Ethernet",mac:"02:00:00:00:00:01",mtu:1500,up:true,loopback:false,addresses:["192.0.2.10/24"]}],
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
  Object.assign(window,{go:{wailsui:{Bindings:api}},runtime:{EventsOn:(name:string,fn:(v:unknown)=>void)=>{handlers[name]??=[];handlers[name].push(fn);return()=>{handlers[name]=handlers[name].filter(f=>f!==fn);};}}});
});await page.goto("/");await expect(page.getByText("Fixture Ethernet")).toBeVisible();}
test("diagnostic job returns real contract result and history",async({page})=>{await connect(page);await page.getByRole("button",{name:"Diagnostics",exact:true}).click();await page.getByRole("button",{name:/Run diagnostic/}).click();await expect(page.locator("pre")).toHaveText("connected");await page.getByRole("button",{name:"History",exact:true}).click();await expect(page.getByRole("cell",{name:"Diagnostic completed",exact:true})).toBeVisible();});
test("Wi-Fi key clears on navigation and blur",async({page})=>{await connect(page);await page.getByRole("button",{name:"Wi-Fi",exact:true}).click();await page.getByRole("button",{name:"Reveal current network key"}).click();await expect(page.getByText("synthetic-test-key")).toBeVisible();await page.evaluate(()=>window.dispatchEvent(new Event("blur")));await expect(page.getByText("synthetic-test-key")).toHaveCount(0);await page.getByRole("button",{name:"Reveal current network key"}).click();await page.getByRole("button",{name:"Overview",exact:true}).click();await expect(page.getByText("synthetic-test-key")).toHaveCount(0);});
test("packet import and display filtering",async({page})=>{await connect(page);await page.getByRole("button",{name:"Packet inspector",exact:true}).click();await page.getByRole("button",{name:"Import PCAP / PCAPNG"}).click();await expect(page.getByRole("cell",{name:"DNS",exact:true})).toBeVisible();await page.getByLabel("Display filter (protocol or address)").fill("TCP");await expect(page.getByRole("cell",{name:"DNS",exact:true})).toHaveCount(0);});
test("PT-BR catalog and preferences",async({page})=>{await connect(page);await page.getByRole("button",{name:"Settings",exact:true}).click();await page.getByLabel("Language",{exact:true}).last().selectOption("pt-BR");await page.getByRole("button",{name:"Salvar preferências"}).click();await expect(page.getByRole("status")).toHaveText("Preferências salvas");});
test("all screens have no serious or critical accessibility violations",async({page})=>{await connect(page);for(const name of ["Overview","Interfaces","Diagnostics","Wi-Fi","Packet inspector","Discovery & tools","History","Settings"]){await page.getByRole("button",{name,exact:true}).click();const results=await new AxeBuilder({page}).analyze();expect(results.violations.filter(v=>v.impact==="serious"||v.impact==="critical"),name).toEqual([]);}});
test("browser without Wails shows explicit limitation",async({page})=>{await page.goto("/");await expect(page.getByText("Native bridge unavailable. Launch the desktop app to access your network.")).toBeVisible();await expect(page.getByRole("button",{name:/Refresh/})).toBeDisabled();});
test("desktop and compact visual evidence",async({page})=>{await page.setViewportSize({width:1320,height:900});await connect(page);await page.screenshot({path:"../.impeccable/review/desktop.png",fullPage:true});await page.setViewportSize({width:960,height:768});await page.getByRole("button",{name:"Diagnostics",exact:true}).click();await page.screenshot({path:"../.impeccable/review/compact.png",fullPage:true});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);});

test("terminal theme stays readable and usable on narrow screens", async ({page}) => {
 await page.setViewportSize({width:390,height:844});
 await connect(page);
 await page.evaluate(() => document.fonts.ready);
 expect(await page.evaluate(() => document.fonts.check('14px "JetBrains Mono"'))).toBe(true);
 await expect(page.locator("html")).toHaveCSS("background-color", "rgb(0, 0, 0)");
 for (const name of ["Overview","Interfaces","Diagnostics","Wi-Fi","Packet inspector","Discovery & tools","History","Settings"]) {
  await page.getByRole("button", {name, exact:true}).click();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), name).toBe(true);
 }
 await page.getByLabel("Language", {exact:true}).first().selectOption("pt-BR");
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
 await page.screenshot({path:"../.impeccable/review/mobile.png", fullPage:true});
});

test("overview uses provider capability states", async ({page}) => {
 await connect(page);
 await page.evaluate(() => {
  if (window.go) window.go.wailsui!.Bindings!.Capabilities = async () => [{id:"wifi",state:"unsupported",detail:"Provider unavailable",provider:"fixture"}];
 });
 await page.getByRole("button", {name:/Refresh/}).click();
 await expect(page.locator(".capability-row").filter({hasText:"Wi-Fi"})).toContainText("Not available");
 await page.getByLabel("Language", {exact:true}).selectOption("pt-BR");
 await page.evaluate(() => {
  if (window.go) window.go.wailsui!.Bindings!.Capabilities = async () => [{id:"wifi",state:"dependency_missing",detail:"Dependency missing",provider:"fixture"}];
 });
 await page.getByRole("button", {name:/Atualizar/}).click();
 await expect(page.locator(".capability-row").filter({hasText:"Wi-Fi"})).toContainText("Depend\u00eancia ausente");
});

test("limited imports disclose omitted packets", async ({page}) => {
 await connect(page);
 await page.evaluate(() => {
  if (window.go) window.go.wailsui!.Bindings!.ImportCapture = async () => ({packets:[],limited:true,format:"pcap"});
 });
 await page.getByRole("button", {name:"Packet inspector",exact:true}).click();
 await page.getByRole("button", {name:"Import PCAP / PCAPNG"}).click();
 await expect(page.getByRole("status")).toContainText("10,000 packet limit");
});

test("discovery results stay bounded and paginate", async ({page}) => {
 await connect(page);
 await page.evaluate(() => {
  if (window.go) window.go.wailsui!.Bindings!.Discover = async () => Array.from({length:205}, (_,i) => ({host:`192.0.2.${Math.floor(i/32)+1}`,port:i+1,state:"open"}));
 });
 await page.getByRole("button",{name:"Discovery & tools",exact:true}).click();
 await page.getByRole("button",{name:"Discover",exact:true}).click();
 await expect(page.locator("tbody tr")).toHaveCount(100);
 await page.getByRole("button",{name:"Next",exact:true}).click();
 await expect(page.locator("tbody tr")).toHaveCount(100);
 await page.getByRole("button",{name:"Next",exact:true}).click();
 await expect(page.locator("tbody tr")).toHaveCount(5);
 await expect(page.getByRole("button",{name:"Next",exact:true})).toBeDisabled();
});

test("running discovery cancels through its service job ID", async ({page}) => {
 await connect(page);
 await page.evaluate(()=>{document.documentElement.dataset.cancelDiscovery="true";});
 await page.getByRole("button",{name:"Discovery & tools",exact:true}).click();
 await expect(page.getByRole("button",{name:"Cancel",exact:true})).toBeDisabled();
 await page.getByRole("button",{name:"Discover",exact:true}).click();
 await expect(page.getByRole("button",{name:"Cancel",exact:true})).toBeEnabled();
 await page.getByRole("button",{name:"Cancel",exact:true}).click();
 await expect(page.getByRole("button",{name:"Discover",exact:true})).toBeEnabled();
 await expect(page.getByRole("button",{name:"Cancel",exact:true})).toBeDisabled();
 await page.screenshot({path:"../.impeccable/review/discovery.png",fullPage:true});
});

test("capture exports use the selected native format and report cancellation or failure", async ({page}) => {
 await connect(page);
 await page.getByRole("button",{name:"Packet inspector",exact:true}).click();
 const save=page.getByRole("button",{name:"Save capture file",exact:true});
 await expect(save).toBeDisabled();
 await page.getByRole("button",{name:"Import PCAP / PCAPNG"}).click();
 await save.click();
 await expect(page.getByRole("status")).toHaveText("Capture file saved");
 expect(await page.evaluate(()=>document.documentElement.dataset.savedCapture)).toBe("pcapng");
 await page.getByLabel("Capture export format").selectOption("pcap");
 await save.click();
 expect(await page.evaluate(()=>document.documentElement.dataset.savedCapture)).toBe("pcap");
 await page.evaluate(()=>{if(window.go)window.go.wailsui!.Bindings!.SaveCapture=async()=>false;});
 await save.click();
 await expect(page.getByText("Capture file saved",{exact:true})).toHaveCount(0);
 await page.evaluate(()=>{if(window.go)window.go.wailsui!.Bindings!.SaveCapture=async()=>{throw new Error("Fixture disk full");};});
 await save.click();
 await expect(page.getByRole("alert")).toContainText("Fixture disk full");
 await expect(save).toBeEnabled();
 await page.screenshot({path:"../.impeccable/review/capture-export.png",fullPage:true});
});

test("DNS record selection reaches the diagnostic service", async ({page}) => {
 await connect(page);
 await page.getByRole("button",{name:"Diagnostics",exact:true}).click();
 await page.getByRole("combobox",{name:"Operation",exact:true}).selectOption("dns");
 await page.getByRole("combobox",{name:"DNS record type",exact:true}).selectOption("MX");
 await page.getByRole("button",{name:/Run diagnostic/}).click();
 await expect.poll(()=>page.evaluate(()=>document.documentElement.dataset.dnsRecord)).toBe("MX");
});

test("Nmap is optional and service inspection preserves observed provider data", async ({page}) => {
 await connect(page);
 await page.getByRole("button",{name:"Discovery & tools",exact:true}).click();
 await expect(page.getByRole("button",{name:"Inspect services with Nmap",exact:true})).toBeDisabled();
 await page.getByRole("button",{name:"Overview",exact:true}).click();
 await page.evaluate(()=>{
  if(!window.go)return;
  window.go.wailsui!.Bindings!.Tools=async()=>[{name:"nmap",state:"available",version:"Nmap version 7.98",path:"fixture"}];
  window.go.wailsui!.Bindings!.InspectServices=async()=>({scanner:"nmap",version:"7.98",hosts:[{addresses:[{address:"192.0.2.1",type:"ipv4"}],ports:[{port:80,state:{state:"open"},service:{name:"http",product:"Synthetic HTTP",version:"1",method:"probed"}}]}]});
 });
 await page.getByRole("button",{name:"Discovery & tools",exact:true}).click();
 await page.getByRole("button",{name:"Inspect services with Nmap",exact:true}).click();
 await expect(page.getByRole("cell",{name:"http Synthetic HTTP 1",exact:true})).toBeVisible();
 await expect(page.getByRole("cell",{name:"probed",exact:true})).toBeVisible();
 await expect(page.getByText("nmap 7.98",{exact:true})).toBeVisible();
});

test("capture selection handles immediate start failure without getting stuck", async ({page}) => {
 await connect(page);
 await page.getByRole("button",{name:"Packet inspector",exact:true}).click();
 const start=page.getByRole("button",{name:"Start capture",exact:true});
 await expect(start).toBeDisabled();
 await page.getByRole("combobox",{name:"Local non-loopback IPv4",exact:true}).selectOption("192.0.2.10");
 await page.evaluate(()=>{if(window.go)window.go.wailsui!.Bindings!.StartCapture=async()=>{throw new Error("permission_required: Fixture denied");};});
 await start.click();
 await expect(page.getByRole("alert")).toContainText("Access denied");
 await expect(start).toBeEnabled();
 await expect(page.getByRole("button",{name:"Stop capture",exact:true})).toBeDisabled();
});

test("diagnostic failures have translated reasons", async ({page}) => {
 await connect(page);
 await page.evaluate(()=>{
  if(window.go)window.go.wailsui!.Bindings!.StartDiagnostic=async()=>{throw new Error("permission_required: Fixture denied");};
 });
 await page.getByLabel("Language",{exact:true}).selectOption("pt-BR");
 await page.getByRole("button",{name:"Diagn\u00f3sticos",exact:true}).click();
 await page.getByRole("button",{name:/Executar diagn/}).click();
 await expect(page.getByRole("alert")).toContainText("Acesso negado");
});

test("asynchronous TLS failure explains certificate verification in both languages", async ({page}) => {
 await connect(page);
 await page.evaluate(()=>{document.documentElement.dataset.diagnosticFailure="certificate_invalid";});
 await page.getByRole("button",{name:"Diagnostics",exact:true}).click();
 await page.getByRole("button",{name:/Run diagnostic/}).click();
 await expect(page.getByRole("alert")).toContainText("Certificate verification failed");
 await page.getByLabel("Language",{exact:true}).selectOption("pt-BR");
 await expect(page.getByRole("alert")).toContainText("certificado");
 await expect(page.getByRole("alert")).not.toContainText("Certificate verification failed");
});


test("overview renders location consent state", async ({page}) => {
 await connect(page);
 await page.evaluate(() => {
  if (window.go) window.go.wailsui!.Bindings!.Capabilities = async () => [{id:"wifi",state:"location_consent_required",detail:"Windows location consent",provider:"fixture"}];
 });
 await page.getByRole("button", {name:/Refresh/}).click();
 const row=page.locator(".capability-row").filter({hasText:"Wi-Fi"});
 await expect(row).toContainText("Location consent required");
 await expect(row.locator(".pill")).toHaveClass(/amber/);
 await page.getByLabel("Language", {exact:true}).selectOption("pt-BR");
 await expect(row).toContainText("Consentimento de localiza\u00e7\u00e3o necess\u00e1rio");
 await page.screenshot({path:"../build/bin/h06-location-consent.png",fullPage:true});
});
