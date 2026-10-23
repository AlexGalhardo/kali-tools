import { expect, test, fillDiagnosticTarget, importCapture } from "./support";
import AxeBuilder from "@axe-core/playwright";

test("TLS audit shows matrix, chain and sorted findings", async ({page,connect}, info) => {
 const report = {host:"fixture.test",port:443,server_name:"fixture.test",negotiated_version:"TLS 1.3",cipher_suite:"TLS_AES_128_GCM_SHA256",alpn:"h2",verified:false,verify_error:"unknown_authority",verifier:"go-x509",ocsp_stapled:false,scts:{embedded:0,tls_extension:0},findings_limited:false,lint_engine:"zlint v3.7.2",chain:[{index:0,subject:"CN=fixture.test",issuer:"CN=fixture CA",serial_hex:"2a",not_before:"2026-01-01T00:00:00Z",not_after:"2027-01-01T00:00:00Z",days_remaining:70,sans:["fixture.test"],sans_limited:false,key_algorithm:"ECDSA",key_bits:256,signature_algorithm:"ECDSA-SHA256",is_ca:false,self_signed:false,aia_issuers:[],ocsp_servers:[],crl_distribution_points:[],embedded_scts:0,sha256:"0".repeat(64)}],protocols:[{version:"TLS 1.0",status:"rejected"},{version:"TLS 1.1",status:"rejected"},{version:"TLS 1.2",status:"accepted",cipher:"fixture"},{version:"TLS 1.3",status:"accepted",cipher:"fixture"}],findings:[{source:"zlint",id:"fixture_info",severity:"info",cert_index:0,detail:"Fixture info"},{source:"kali-tools",id:"untrusted_chain",severity:"error",cert_index:0,detail:""},{source:"zlint",id:"fixture_warning",severity:"warn",cert_index:0,detail:"Fixture warning"}]};
 await page.route("**/api/tls/audit",route=>route.fulfill({json:report}));
 await connect(page);
 if(info.project.name==="wails-fake") await page.evaluate(value=>Object.assign(window.go!.wailsui!.Bindings!,{AuditTLS:async()=>value}),report);
 await page.getByRole("button",{name:"Diagnostics",exact:true}).click();
 await page.getByRole("tab",{name:"TLS audit",exact:true}).click();
 await page.getByRole("button",{name:"Audit TLS",exact:true}).click();
 await expect(page.getByRole("heading",{name:"Protocol matrix",exact:true})).toBeVisible();
 await page.getByText("0: CN=fixture.test",{exact:true}).click();
 await expect(page.getByText("CN=fixture CA",{exact:true})).toBeVisible();
 const findings = page.locator("section").filter({has:page.getByRole("heading",{name:"Findings",exact:true})});
 await expect(findings.locator("tbody tr").first()).toContainText("untrusted_chain");
 await expect(findings.locator("tbody tr").nth(1)).toContainText("fixture_warning");
 const audit = await new AxeBuilder({page}).analyze();
 expect(audit.violations.filter(v=>v.impact==="critical"||v.impact==="serious")).toEqual([]);
 await page.screenshot({path:info.outputPath("tls-audit-desktop.png"),fullPage:true});
 await page.setViewportSize({width:390,height:844});
 await expect(page.getByRole("button",{name:"Audit TLS",exact:true})).toBeVisible();
 await page.screenshot({path:info.outputPath("tls-audit-mobile.png"),fullPage:true});
});

test("Wi-Fi location consent shows settings link and never retries automatically @wails-only", async ({page,connect}, testInfo) => {
 await connect(page);
 await page.evaluate(() => {
  let calls = 0;
  const api = window.go!.wailsui!.Bindings!;
  api.CurrentWiFi = async () => {
   document.documentElement.dataset.wifiCalls = String(++calls);
   if (calls === 1) throw new Error("location_consent_required: Windows location access is off");
   return {profile:"Fixture",ssid:"Fixture Network",bssid:"02:00:00:00:00:02",signal:80,authentication:7,state:"connected"};
  };
  Object.assign(api, {OpenLocationSettings:async () => {document.documentElement.dataset.settingsOpened = "1";}});
 });
 await page.getByRole("button",{name:"Wi-Fi",exact:true}).click();
 await expect(page.getByRole("alert")).toContainText("Windows location access is off");
 await expect(page.locator("code")).toHaveText("ms-settings:privacy-location");
 await page.screenshot({path:testInfo.outputPath("wifi-consent-desktop.png")});
 await page.setViewportSize({width:390,height:844});
 await page.getByLabel("Language").selectOption("pt-BR");
 await expect(page.getByRole("alert")).toContainText("O acesso à localização do Windows está desativado");
 await page.screenshot({path:testInfo.outputPath("wifi-consent-compact-pt.png"),fullPage:true});
 expect((await new AxeBuilder({page}).analyze()).violations.filter(v=>v.impact==="serious"||v.impact==="critical")).toEqual([]);
 await page.getByRole("combobox").selectOption("en");
 await page.setViewportSize({width:1280,height:720});
 await page.getByRole("button",{name:"Open location settings",exact:true}).click();
 await expect(page.locator("html")).toHaveAttribute("data-settings-opened","1");
 await expect(page.locator("html")).toHaveAttribute("data-wifi-calls","1");
 await page.getByRole("button",{name:"Check again",exact:true}).click();
 await expect(page.getByRole("heading",{name:"Fixture Network",exact:true})).toBeVisible();
 await expect(page.locator("html")).toHaveAttribute("data-wifi-calls","2");
 await expect(page.getByRole("alert")).toHaveCount(0);
});

test("consent denial during reveal clears an already visible key @wails-only", async ({page,connect}) => {
 await connect(page);
 await page.getByRole("button",{name:"Wi-Fi",exact:true}).click();
 await page.getByRole("button",{name:"Reveal current network key",exact:true}).click();
 await expect(page.getByText("synthetic-test-key")).toBeVisible();
 await page.evaluate(() => { window.go!.wailsui!.Bindings!.RevealWiFi = async () => { throw new Error("location_consent_required: denied"); }; });
 await page.getByRole("button",{name:"Reveal current network key",exact:true}).click();
 await expect(page.getByRole("alert")).toContainText("Windows location access is off");
 await expect(page.getByText("synthetic-test-key")).toHaveCount(0);
});

test("HTTP transport bounds tracked jobs and closes its event source @wails-only", async ({page,connect}) => {
 await connect(page);
 const results = await page.evaluate(async () => {
  const modulePath = "/src/shared/api/transport.ts";
  const {HttpTransport} = await import(modulePath);
  const originalFetch = window.fetch;
  const OriginalSource = window.EventSource;
  const results: {count: number; ids: string[]; closed: boolean}[] = [];
  try {
   for (const count of [64, 65]) {
    let source: FixtureSource | undefined;
    class FixtureSource extends EventTarget {
     closed = false;
     constructor() { super(); source = this; }
     close() { this.closed = true; }
    }
    window.EventSource = FixtureSource as unknown as typeof EventSource;
    let next = 0;
    const ids: string[] = [];
    window.fetch = (async (path: string) => {
     const id = path === "/api/diagnostics" ? String(++next).padStart(32, "0") : path.split("/").at(-1)!;
     if (path.startsWith("/api/jobs/")) ids.push(id);
     return Response.json({id, state: "running"});
    }) as typeof fetch;
    const transport = new HttpTransport();
    const unsubscribe = transport.on("job", () => {});
    for (let i = 0; i < count; i++) await transport.backend.StartDiagnostic({kind: "tcp", target: "127.0.0.1", port: 9, timeout_ms: 100});
    source!.dispatchEvent(new Event("resync"));
    for (let attempt = 0; attempt < 100 && ids.length < 64; attempt++) await new Promise(resolve => setTimeout(resolve, 1));
    unsubscribe();
    results.push({count, ids, closed: source!.closed});
   }
  } finally {
   window.fetch = originalFetch;
   window.EventSource = OriginalSource;
  }
  return results;
 });
 for (const {count, ids, closed} of results) {
  expect(ids).toEqual(Array.from({length: 64}, (_, i) => String(i + (count === 64 ? 1 : 2)).padStart(32, "0")));
  expect(closed).toBe(true);
 }
});

test("diagnostic job returns real contract result and history",async({page,connect})=>{await connect(page);await page.getByRole("button",{name:"Diagnostics",exact:true}).click();await fillDiagnosticTarget(page);await page.getByRole("button",{name:/Run diagnostic/}).click();await expect(page.locator("pre")).toHaveText("connected");await page.getByRole("button",{name:"History",exact:true}).click();await expect(page.getByRole("cell",{name:"Diagnostic completed",exact:true})).toBeVisible();});
test("Wi-Fi key clears on navigation and blur",async({page,connect})=>{await connect(page);await page.getByRole("button",{name:"Wi-Fi",exact:true}).click();await page.getByRole("button",{name:"Reveal current network key"}).click();await expect(page.getByText("synthetic-test-key")).toBeVisible();await page.evaluate(()=>window.dispatchEvent(new Event("blur")));await expect(page.getByText("synthetic-test-key")).toHaveCount(0);await page.getByRole("button",{name:"Reveal current network key"}).click();await page.getByRole("button",{name:"Overview",exact:true}).click();await expect(page.getByText("synthetic-test-key")).toHaveCount(0);});
test("packet import and display filtering",async({page,connect})=>{await connect(page);await page.getByRole("button",{name:"Packet inspector",exact:true}).click();await importCapture(page);await expect(page.getByRole("cell",{name:"DNS",exact:true})).toBeVisible();await page.getByLabel("Display filter (protocol or address)").fill("TCP");await expect(page.getByRole("cell",{name:"DNS",exact:true})).toHaveCount(0);});
test("PT-BR catalog and preferences",async({page,connect})=>{await connect(page);await page.getByRole("button",{name:"Settings",exact:true}).click();await page.getByLabel("Language",{exact:true}).last().selectOption("pt-BR");await page.getByRole("button",{name:"Salvar preferências"}).click();await expect(page.getByRole("status")).toHaveText("Preferências salvas");});
test("all screens have no serious or critical accessibility violations",async({page,connect})=>{await connect(page);for(const name of ["Overview","Interfaces","Diagnostics","Wi-Fi","Packet inspector","Discovery & tools","History","Settings"]){await page.getByRole("button",{name,exact:true}).click();const results=await new AxeBuilder({page}).analyze();expect(results.violations.filter(v=>v.impact==="serious"||v.impact==="critical"),name).toEqual([]);}});
test("browser without a backend shows explicit limitation @wails-only",async({page,connect})=>{await page.goto("/");await expect(page.getByText("Not connected. Launch the desktop app, or open the URL printed by kali-tools serve.")).toBeVisible();await expect(page.getByRole("button",{name:/Refresh/})).toBeDisabled();});
test("desktop and compact visual evidence @wails-only",async({page,connect})=>{await page.setViewportSize({width:1320,height:900});await connect(page);await page.screenshot({path:"../.impeccable/review/desktop.png",fullPage:true});await page.setViewportSize({width:960,height:768});await page.getByRole("button",{name:"Diagnostics",exact:true}).click();await page.screenshot({path:"../.impeccable/review/compact.png",fullPage:true});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);});

test("terminal theme stays readable and usable on narrow screens @wails-only", async({page,connect}) => {
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

test("overview uses provider capability states @wails-only", async({page,connect}) => {
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

test("limited imports disclose omitted packets @wails-only", async({page,connect}) => {
 await connect(page);
 await page.evaluate(() => {
  if (window.go) window.go.wailsui!.Bindings!.ImportCapture = async () => ({packets:[],limited:true,format:"pcap"});
 });
 await page.getByRole("button", {name:"Packet inspector",exact:true}).click();
 await page.getByRole("button", {name:"Import PCAP / PCAPNG"}).click();
 await expect(page.getByRole("status")).toContainText("10,000 packet limit");
});

test("discovery results stay bounded and paginate @wails-only", async({page,connect}) => {
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

test("running discovery cancels through its service job ID @wails-only", async({page,connect}) => {
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

test("capture exports use the selected native format and report cancellation or failure @wails-only", async({page,connect}) => {
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

test("DNS record selection reaches the diagnostic service @wails-only", async({page,connect}) => {
 await connect(page);
 await page.getByRole("button",{name:"Diagnostics",exact:true}).click();
 await page.getByRole("combobox",{name:"Operation",exact:true}).selectOption("dns");
 await page.getByRole("combobox",{name:"DNS record type",exact:true}).selectOption("MX");
 await page.getByRole("button",{name:/Run diagnostic/}).click();
 await expect.poll(()=>page.evaluate(()=>document.documentElement.dataset.dnsRecord)).toBe("MX");
});

test("service inspection stays available without optional tools @wails-only", async({page,connect}) => {
 await connect(page);
 await page.getByRole("button",{name:"Discovery & tools",exact:true}).click();
 await expect(page.getByRole("button",{name:"Inspect services",exact:true})).toBeEnabled();
 await page.evaluate(()=>{
  if(!window.go)return;
  window.go.wailsui!.Bindings!.InspectServices=async()=>({scanner:"native",version:"1",hosts:[{addresses:[{address:"192.0.2.1",type:"ipv4"}],ports:[{port:80,state:{state:"open"},service:{name:"http",product:"Synthetic HTTP",version:"1",method:"banner"}}]}]});
 });
 await page.getByRole("button",{name:"Inspect services",exact:true}).click();
 await expect(page.getByRole("cell",{name:"http Synthetic HTTP 1",exact:true})).toBeVisible();
 await expect(page.getByRole("cell",{name:"banner",exact:true})).toBeVisible();
 await expect(page.getByText("native 1",{exact:true})).toBeVisible();
});

test("capture selection handles immediate start failure without getting stuck @wails-only", async({page,connect}) => {
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

test("diagnostic failures have translated reasons @wails-only", async({page,connect}) => {
 await connect(page);
 await page.evaluate(()=>{
  if(window.go)window.go.wailsui!.Bindings!.StartDiagnostic=async()=>{throw new Error("permission_required: Fixture denied");};
 });
 await page.getByLabel("Language",{exact:true}).selectOption("pt-BR");
 await page.getByRole("button",{name:"Diagn\u00f3sticos",exact:true}).click();
 await page.getByRole("button",{name:/Executar diagn/}).click();
 await expect(page.getByRole("alert")).toContainText("Acesso negado");
});

test("asynchronous TLS failure explains certificate verification in both languages @wails-only", async({page,connect}) => {
 await connect(page);
 await page.evaluate(()=>{document.documentElement.dataset.diagnosticFailure="certificate_invalid";});
 await page.getByRole("button",{name:"Diagnostics",exact:true}).click();
 await page.getByRole("button",{name:/Run diagnostic/}).click();
 await expect(page.getByRole("alert")).toContainText("Certificate verification failed");
 await page.getByLabel("Language",{exact:true}).selectOption("pt-BR");
 await expect(page.getByRole("alert")).toContainText("certificado");
 await expect(page.getByRole("alert")).not.toContainText("Certificate verification failed");
});


test("overview renders location consent state @wails-only", async({page,connect}) => {
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

test("HTTP diagnostic shows phases, headers and the redirect target", async ({page, connect}, info) => {
 await connect(page);
 await page.getByRole("button", {name: "Diagnostics", exact: true}).click();
 await page.getByRole("combobox", {name: "Operation", exact: true}).selectOption("http");
 await expect(page.getByLabel("Trace redirects (0-5)", {exact: true})).toBeVisible();
 if (info.project.name === "http") {
  await page.getByLabel("Target", {exact: true}).fill(new URL(page.url()).origin + "/healthz");
 } else {
  await page.getByLabel("Target", {exact: true}).fill("http://127.0.0.1/fixture");
 }
 await page.getByRole("button", {name: /Run diagnostic/}).click();
 await expect(page.getByRole("heading", {name: "Phases", exact: true})).toBeVisible();
 await expect(page.getByRole("heading", {name: "Security headers", exact: true})).toBeVisible();
 await expect(page.getByRole("heading", {name: "Redirect target", exact: true})).toBeVisible();
 if (info.project.name === "http") {
  await expect(page.getByRole("rowheader", {name: "Strict-Transport-Security", exact: true})).toBeVisible();
 } else {
  await expect(page.getByText("default-src 'none'", {exact: true})).toBeVisible();
  await expect(page.getByText("http://192.0.2.1/next", {exact: true})).toBeVisible();
  await expect(page.getByRole("link", {name: "http://192.0.2.1/next"})).toHaveCount(0);
  await expect(page.getByText("Cross-host", {exact: true})).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.dataset.redirects)).toBe("0");
 }
});
