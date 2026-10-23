import { Button } from "@base-ui/react/button";
import { useEffect, useState } from "react";
import type { PageProps } from "../../app/shell";
import {
  backend,
  connected,
  type Interface,
  type SNMPResult,
} from "../../shared/api/bridge";
import { Field } from "../../shared/ui/Field";

const systemKeys = [
  "sysDescr",
  "sysObjectID",
  "sysUpTime",
  "sysContact",
  "sysName",
  "sysLocation",
];

export function SNMPPanel({
  t,
  busy,
  execute,
}: Pick<PageProps, "t" | "busy" | "execute">) {
  const [interfaces, setInterfaces] = useState<Interface[]>([]);
  const [iface, setIface] = useState("");
  const [cidr, setCIDR] = useState("");
  const [target, setTarget] = useState("");
  const port = 161;
  const [version, setVersion] = useState("v2c");
  const [community, setCommunity] = useState("");
  const [user, setUser] = useState("");
  const [authPass, setAuthPass] = useState("");
  const [privPass, setPrivPass] = useState("");
  const [owned, setOwned] = useState(false);
  const [walk, setWalk] = useState(false);
  const [result, setResult] = useState<SNMPResult>();
  useEffect(() => {
    if (!connected()) return;
    void execute(async () => {
      const rows = await backend().Interfaces();
      setInterfaces(rows);
      setIface(
        (current) =>
          current ||
          rows.find((item) => item.up && !item.loopback)?.name ||
          rows[0]?.name ||
          "",
      );
    });
    return () => {
      setCommunity("");
      setAuthPass("");
      setPrivPass("");
    };
  }, [execute]);
  const secretReady =
    version === "v3" ? user !== "" && authPass !== "" : community !== "";
  const clearSecrets = () => {
    setCommunity("");
    setAuthPass("");
    setPrivPass("");
  };
  return (
    <div className="local-discovery">
      <form
        className="card form"
        aria-labelledby="snmp-title"
        onSubmit={(event) => {
          event.preventDefault();
          const body = {
            scope: { interface: iface, cidr },
            target,
            port,
            owned,
            version,
            timeout_ms: 2000,
            interfaces: walk,
            community: version === "v2c" ? community : undefined,
            v3:
              version === "v3"
                ? {
                    user,
                    auth_protocol: "SHA256",
                    auth_pass: authPass,
                    priv_protocol: privPass ? "AES" : undefined,
                    priv_pass: privPass || undefined,
                  }
                : undefined,
          };
          clearSecrets();
          void execute(async () => {
            setResult(await backend().QuerySNMP(body));
          });
        }}
      >
        <h2 id="snmp-title">{t.snmp}</h2>
        <div className="form-grid">
          <Field label={t.scopeInterface}>
            <select
              value={iface}
              onChange={(event) => setIface(event.target.value)}
              required
            >
              {interfaces.map((item) => (
                <option key={item.name} value={item.name}>
                  {item.name}
                </option>
              ))}
            </select>
          </Field>
          <Field label={t.scopeCidr}>
            <input
              required
              value={cidr}
              onChange={(event) => setCIDR(event.target.value)}
              placeholder="192.0.2.0/24"
            />
          </Field>
          <Field label={t.snmpTarget}>
            <input
              required
              value={target}
              onChange={(event) => setTarget(event.target.value)}
            />
          </Field>
          <Field label={t.snmpVersion}>
            <select
              value={version}
              onChange={(event) => setVersion(event.target.value)}
            >
              <option value="v2c">v2c</option>
              <option value="v3">v3</option>
            </select>
          </Field>
          {version === "v2c" ? (
            <Field label={t.community}>
              <input
                type="password"
                autoComplete="off"
                value={community}
                onChange={(event) => setCommunity(event.target.value)}
              />
            </Field>
          ) : (
            <Field label={t.v3User}>
              <input
                value={user}
                onChange={(event) => setUser(event.target.value)}
                maxLength={64}
              />
            </Field>
          )}
          {version === "v3" && (
            <Field label={t.authPass}>
              <input
                type="password"
                autoComplete="off"
                value={authPass}
                onChange={(event) => setAuthPass(event.target.value)}
              />
            </Field>
          )}
          {version === "v3" && (
            <Field label={t.privPass}>
              <input
                type="password"
                autoComplete="off"
                value={privPass}
                onChange={(event) => setPrivPass(event.target.value)}
              />
            </Field>
          )}
        </div>
        <div className="field">
          <label>
            <input
              type="checkbox"
              checked={owned}
              onChange={(event) => setOwned(event.target.checked)}
            />{" "}
            {t.ownedConfirm}
          </label>
          <label>
            <input
              type="checkbox"
              checked={walk}
              onChange={(event) => setWalk(event.target.checked)}
            />{" "}
            {t.snmpInterfaces}
          </label>
        </div>
        <div className="actions">
          <Button
            type="submit"
            disabled={
              !connected() ||
              busy ||
              !owned ||
              !secretReady ||
              !iface ||
              !cidr ||
              !target
            }
          >
            {t.snmp}
          </Button>
        </div>
      </form>
      {result && (
        <section className="card">
          <h2>{t.snmpSystem}</h2>
          <table>
            <tbody>
              {systemKeys.map((key) => (
                <tr key={key}>
                  <th scope="row">{key}</th>
                  <td>{result.system?.[key]}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <h2>{t.snmpInterfaces}</h2>
          <table>
            <thead>
              <tr>
                <th>Index</th>
                <th>Descr</th>
                <th>Status</th>
                <th>MAC</th>
              </tr>
            </thead>
            <tbody>
              {(result.interfaces ?? []).map((row) => (
                <tr key={row.index}>
                  <td>{row.index}</td>
                  <td>{row.descr}</td>
                  <td>{row.oper_status}</td>
                  <td>{row.mac}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}
    </div>
  );
}
