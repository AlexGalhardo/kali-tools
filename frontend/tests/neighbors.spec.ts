import AxeBuilder from "@axe-core/playwright";
import {expect,test} from "./support";

test("neighbors table paginates and filters by family",async({page,connect},testInfo)=>{
 if(testInfo.project.name==="http"){
  await page.route("**/api/neighbors?*",async route=>{
   const family=new URL(route.request().url()).searchParams.get("family")??"all";
   const neighbors=Array.from({length:family==="ipv6"?1:205},(_,i)=>({address:family==="ipv6"?"2001:db8::1":`192.0.2.${i+1}`,mac:"02:00:00:00:00:01",family:family==="ipv6"?"ipv6":"ipv4",interface_index:1,interface_name:"Fixture Ethernet",state:"reachable",router:i===0}));
   await route.fulfill({contentType:"application/json",body:JSON.stringify({neighbors,limited:true,provider:"synthetic passive cache"})});
  });
 }
 await connect(page);
 if(testInfo.project.name!=="http")await page.evaluate(()=>{
  window.go!.wailsui!.Bindings!.Neighbors=async family=>{
   document.documentElement.dataset.neighborFamily=family;
   return {neighbors:Array.from({length:family==="ipv6"?1:205},(_,i)=>({address:family==="ipv6"?"2001:db8::1":`192.0.2.${i+1}`,mac:"02:00:00:00:00:01",family:family==="ipv6"?"ipv6":"ipv4",interface_index:1,interface_name:"Fixture Ethernet",state:"reachable",router:i===0})),limited:true,provider:"synthetic passive cache"};
  };
 });
 await page.getByRole("button",{name:"Interfaces",exact:true}).click();
 const region=page.getByRole("region",{name:"Neighbors",exact:true});
 await expect(region.getByText("Page 1 / 3",{exact:true})).toBeVisible();
 await expect(region.getByRole("row")).toHaveCount(101);
 await expect(region.getByText("Only the first 4,096 usable neighbors are shown.",{exact:true})).toBeVisible();
 await region.getByRole("button",{name:"Next",exact:true}).click();
 await expect(region.getByText("Page 2 / 3",{exact:true})).toBeVisible();
 await region.getByRole("button",{name:"Next",exact:true}).click();
 await expect(region.getByRole("row")).toHaveCount(6);
 await region.getByLabel("Address family",{exact:true}).selectOption("ipv6");
 await expect(region.getByRole("cell",{name:"2001:db8::1",exact:true})).toBeVisible();
 await expect(region.getByText("Page 1 / 1",{exact:true})).toBeVisible();
 if(testInfo.project.name!=="http")await expect(page.locator("html")).toHaveAttribute("data-neighbor-family","ipv6");
 await region.getByRole("button",{name:"Refresh",exact:true}).click();
 await expect(region.getByText("synthetic passive cache",{exact:true})).toBeVisible();
 await page.screenshot({path:testInfo.outputPath("neighbors-desktop.png"),fullPage:true});
 await page.setViewportSize({width:390,height:844});
 await page.getByLabel("Language",{exact:true}).first().selectOption("pt-BR");
 await expect(page.getByRole("heading",{name:"Vizinhos",exact:true})).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 expect((await new AxeBuilder({page}).analyze()).violations.filter(v=>v.impact==="serious"||v.impact==="critical")).toEqual([]);
 await page.screenshot({path:testInfo.outputPath("neighbors-compact-pt.png"),fullPage:true});
});
