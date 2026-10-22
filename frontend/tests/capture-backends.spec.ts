import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "./support";

test("capture backend selector reflects capability states and loss notes @wails-only", async ({page,connect}, testInfo) => {
 await connect(page);
 await page.evaluate(() => {
  const api = window.go!.wailsui!.Bindings!;
  api.CaptureBackends = async () => [
   {id:"capture_live",state:"permission_required",provider:"windows",detail:"Raw capture needs Administrator"},
   {id:"capture_pktmon",state:"available",provider:"pktmon",detail:"All NICs, truncated to 512 bytes"},
   {id:"capture_npcap",state:"dependency_missing",provider:"npcap",detail:"Install Npcap yourself"},
  ];
  api.OpenNpcapDownload = async () => {document.documentElement.dataset.npcapOpened="1";};
  api.StartCapture = async request => {
   document.documentElement.dataset.captureRequest=JSON.stringify(request);
  };
 });
 await page.getByRole("button",{name:"Packet inspector",exact:true}).click();
 const selector=page.getByRole("combobox",{name:"Capture backend",exact:true});
 await expect(selector.locator('option[value="raw_socket"]')).toBeDisabled();
 await expect(selector.locator('option[value="npcap"]')).toBeDisabled();
 await selector.selectOption("pktmon");
 await expect(page.getByText("All NICs, truncated to 512 bytes",{exact:true})).toBeVisible();
 await expect(page.getByRole("combobox",{name:"Local non-loopback IPv4",exact:true})).toHaveCount(0);
 await page.getByRole("button",{name:"Install Npcap (opens npcap.com)",exact:true}).click();
 await expect(page.locator("html")).toHaveAttribute("data-npcap-opened","1");
 await page.getByRole("button",{name:"Start capture",exact:true}).click();
 const request=await page.locator("html").getAttribute("data-capture-request");
 expect(JSON.parse(request!)).toMatchObject({backend:"pktmon",address:""});
 // Deliver the same event shape as the production bridge through the fixture emitter.
 await page.evaluate(() => window.dispatchEvent(new CustomEvent("fixture-capture",{detail:{result:{packets:[],limited:false,dropped:2,limits:["circular_buffer_may_overwrite_oldest","packets_truncated","component_identity_lost","capture_scope_all_nics","duplicates_possible"]},error:""}})));
 await expect(page.getByText("Dropped packets: 2",{exact:true})).toBeVisible();
 await expect(page.getByText("Packets were truncated.",{exact:true})).toBeVisible();
 await expect(page.getByRole("button",{name:"Stop capture",exact:true})).toBeDisabled();
 await page.screenshot({path:testInfo.outputPath("capture-backends-desktop.png"),fullPage:true});
 await page.setViewportSize({width:390,height:844});
 await page.getByLabel("Language",{exact:true}).first().selectOption("pt-BR");
 await expect(page.getByText("Pacotes perdidos: 2",{exact:true})).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 expect((await new AxeBuilder({page}).analyze()).violations.filter(v=>v.impact==="serious"||v.impact==="critical")).toEqual([]);
 await page.screenshot({path:testInfo.outputPath("capture-backends-compact-pt.png"),fullPage:true});
 await page.evaluate(() => window.dispatchEvent(new CustomEvent("fixture-capture",{detail:{result:{packets:[],limited:false,dropped:null,limits:["drop_count_unavailable"]},error:""}})));
 await expect(page.getByText("A contagem de perdas está indisponível.",{exact:true})).toBeVisible();
 await expect(page.getByText(/^Pacotes perdidos:/)).toHaveCount(0);
});

test("browser capture exposes backend states and a download address @http-only", async ({page,connect}) => {
 await page.route("**/api/capture/backends", route=>route.fulfill({contentType:"application/json",body:JSON.stringify([
  {id:"capture_live",state:"available",provider:"fake",detail:"Synthetic raw capture"},
  {id:"capture_pktmon",state:"unsupported",provider:"pktmon",detail:"Not supported"},
  {id:"capture_npcap",state:"dependency_missing",provider:"npcap",detail:"Install Npcap yourself"},
 ])}));
 await connect(page);
 await page.getByRole("button",{name:"Packet inspector",exact:true}).click();
 await expect(page.getByRole("combobox",{name:"Capture backend",exact:true}).locator('option[value="pktmon"]')).toBeDisabled();
 await expect(page.getByRole("link",{name:"https://npcap.com/#download",exact:true})).toHaveAttribute("href","https://npcap.com/#download");
 await expect(page.getByRole("button",{name:"Install Npcap (opens npcap.com)",exact:true})).toHaveCount(0);
});
