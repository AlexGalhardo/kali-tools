import { readFileSync } from "node:fs";
import { en } from "../src/shared/i18n/i18n";
import { expect, test, importCapture, fillDiagnosticTarget } from "./support";

test.describe.configure({mode:"serial"});
test("token is scrubbed and exchanged for an HttpOnly session @http-only",async({page,connect})=>{
 await connect(page);expect(new URL(page.url()).hash).toBe("");
 const cookies=await page.context().cookies();const session=cookies.find(c=>c.name.startsWith("kali_session_"));
 expect(session).toMatchObject({httpOnly:true,sameSite:"Strict",secure:false});
 expect(await page.evaluate(()=>document.cookie)).not.toContain(session!.value);
});
test("capture export downloads a binary PCAPNG @http-only",async({page,connect})=>{
 await connect(page);await page.getByRole("button",{name:"Packet inspector",exact:true}).click();await importCapture(page);
 await expect(page.getByRole("cell",{name:"DNS",exact:true})).toBeVisible();
 const download=page.waitForEvent("download");await page.getByRole("button",{name:"Save capture file",exact:true}).click();
 const file=await download;expect(file.suggestedFilename()).toBe("capture.pcapng");expect(readFileSync((await file.path())!).subarray(0,4).toString("hex")).toBe("0a0d0d0a");
});
test("capability states come from the server @http-only",async({page,connect})=>{
 await connect(page);const response=await page.request.get("/api/capabilities");expect(response.ok()).toBe(true);
 const capabilities=await response.json();expect(capabilities.length).toBeGreaterThan(0);
 for(const capability of capabilities.filter((c:{id:string})=>["diagnostics","wifi","capture_live"].includes(c.id))){const item=page.locator(".capability-row").filter({hasText:capability.id==="wifi"?"Wi-Fi":capability.id==="diagnostics"?"Diagnostics":"Packet inspector"});await expect(item).toHaveAttribute("title",capability.detail);await expect(item.locator(".pill")).toHaveText(({available:en.available,permission_required:en.restricted,location_consent_required:en.locationConsent,unsupported:en.unsupported} as Record<string,string>)[capability.state]??en.unsupported);}
 await expect(page.getByText(/Served by kali-tools serve/)).toBeVisible();
});
test("served index is the embedded build byte for byte @http-only",async({request})=>{
 const response=await request.get("/");expect(await response.body()).toEqual(readFileSync("dist/index.html"));
});
test("no CSP violations across all pages @http-only",async({page,connect})=>{
 await page.addInitScript(()=>{Object.assign(window,{policyViolations:[]});document.addEventListener("securitypolicyviolation",e=>{(window as unknown as {policyViolations:string[]}).policyViolations.push(e.violatedDirective+":"+e.blockedURI)});});
 await connect(page);
 for(const name of ["Overview","Interfaces","Diagnostics","Wi-Fi","Packet inspector","Discovery & tools","History","Settings"])await page.getByRole("button",{name,exact:true}).click();
 expect(await page.evaluate(()=>(window as unknown as {policyViolations:string[]}).policyViolations)).toEqual([]);
});
test("authenticated SSE delivers terminal job events @http-only",async({page,connect})=>{
 await connect(page);await page.evaluate(()=>{Object.assign(window,{terminalSeen:false});const events=new EventSource("/api/events");events.addEventListener("job",e=>{const value=JSON.parse((e as MessageEvent).data);if(value.job.state==="completed"){Object.assign(window,{terminalSeen:true});events.close();}});window.addEventListener("pagehide",()=>events.close(),{once:true});});
 await page.getByRole("button",{name:"Diagnostics",exact:true}).click();await fillDiagnosticTarget(page);await page.getByRole("button",{name:/Run diagnostic/}).click();
 await expect.poll(()=>page.evaluate(()=>(window as unknown as {terminalSeen:boolean}).terminalSeen)).toBe(true);
});

