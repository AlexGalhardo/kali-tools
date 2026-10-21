import assert from "node:assert/strict";
import {test} from "node:test";
import {semver,checkRelease} from "./release-policy.mjs";
import {fileURLToPath} from "node:url";

test("SemVer accepts release, prerelease and build metadata",()=>{
 for(const version of ["0.3.0","1.0.0-rc.1","1.2.3+build.01","1.2.3-beta.1+sha.abc"])assert.ok(semver.test(version),version);
});
test("SemVer rejects ambiguous and malformed release identifiers",()=>{
 for(const version of ["v0.3.0","01.2.3","1.02.3","1.2.03","1.2","1.2.3-01","1.2.3-alpha..1","1.2.3+","1.2.3\n"])assert.equal(semver.test(version),false,version);
});
test("repository release metadata and changelog agree",()=>{
 assert.match(checkRelease(fileURLToPath(new URL("../",import.meta.url))),semver);
});
