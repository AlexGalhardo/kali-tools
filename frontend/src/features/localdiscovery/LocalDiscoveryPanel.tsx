import { Button } from "@base-ui/react/button";
import { useEffect, useState } from "react";
import type { PageProps } from "../../app/shell";
import {
  backend,
  connected,
  type Interface,
  type LocalDiscoveryResult,
} from "../../shared/api/bridge";
import { Field } from "../../shared/ui/Field";

const pageSize = 100;

function labelFor(t: PageProps["t"], value: string): string {
  const text = t[value as keyof PageProps["t"]];
  return typeof text === "string" ? text : value;
}

export function LocalDiscoveryPanel({
  t,
  busy,
  execute,
}: Pick<PageProps, "t" | "busy" | "execute">) {
  const [interfaces, setInterfaces] = useState<Interface[]>([]);
  const [iface, setIface] = useState("");
  const [cidr, setCIDR] = useState("");
  const [mdns, setMDNS] = useState(true);
  const [ssdp, setSSDP] = useState(true);
  const [duration, setDuration] = useState(3);
  const [serviceTypes, setServiceTypes] = useState("");
  const [ssdpTarget, setSSDPTarget] = useState("ssdp:all");
  const [page, setPage] = useState(0);
  const [result, setResult] = useState<LocalDiscoveryResult>();
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
  }, [execute]);
  const protocols = [mdns && "mdns", ssdp && "ssdp"].filter(
    (item): item is string => Boolean(item),
  );
  const rows = result?.responders ?? [];
  const visible = rows.slice(page * pageSize, page * pageSize + pageSize);
  return (
    <div className="local-discovery">
      <form
        className="card form"
        aria-labelledby="local-discovery-title"
        onSubmit={(event) => {
          event.preventDefault();
          void execute(async () => {
            setPage(0);
            setResult(
              await backend().DiscoverLocal({
                scope: { interface: iface, cidr },
                protocols,
                duration_seconds: duration,
                service_types: serviceTypes
                  .split(",")
                  .map((item) => item.trim())
                  .filter(Boolean),
                ssdp_target: ssdpTarget,
              }),
            );
          });
        }}
      >
        <h2 id="local-discovery-title">{t.localDiscovery}</h2>
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
          <Field label={t.durationSeconds}>
            <input
              type="number"
              min={1}
              max={10}
              value={duration}
              onChange={(event) => setDuration(Number(event.target.value))}
            />
          </Field>
          <Field label={t.serviceTypes}>
            <input
              value={serviceTypes}
              onChange={(event) => setServiceTypes(event.target.value)}
            />
          </Field>
          <Field label={t.ssdpTarget}>
            <select
              value={ssdpTarget}
              onChange={(event) => setSSDPTarget(event.target.value)}
            >
              <option value="ssdp:all">ssdp:all</option>
              <option value="upnp:rootdevice">upnp:rootdevice</option>
            </select>
          </Field>
        </div>
        <div className="field">
          <span>{t.protocols}</span>
          <label>
            <input
              type="checkbox"
              checked={mdns}
              onChange={(event) => setMDNS(event.target.checked)}
            />{" "}
            mDNS
          </label>
          <label>
            <input
              type="checkbox"
              checked={ssdp}
              onChange={(event) => setSSDP(event.target.checked)}
            />{" "}
            SSDP
          </label>
        </div>
        <div className="actions">
          <Button
            type="submit"
            disabled={
              !connected() || busy || !iface || !cidr || protocols.length === 0
            }
          >
            {t.localDiscovery}
          </Button>
        </div>
      </form>
      {result && (
        <section className="card" aria-label={t.responders}>
          <p>
            {t.responders}: {rows.length}. {t.outOfScope}: {result.out_of_scope}
            . {t.malformed}: {result.malformed}.
          </p>
          {result.errors.map((code) => (
            <p key={code}>{labelFor(t, code)}</p>
          ))}
          <table>
            <thead>
              <tr>
                <th>{t.protocols}</th>
                <th>{t.address}</th>
                <th>Name</th>
                <th>Type</th>
                <th>Server/TXT</th>
                <th>Location</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((row) => (
                <tr key={`${row.protocol}-${row.address}-${row.name}`}>
                  <td>{row.protocol}</td>
                  <td>{row.address}</td>
                  <td>{row.name}</td>
                  <td>{row.service_type}</td>
                  <td>{row.server || row.txt?.join(" ")}</td>
                  <td>{row.location}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {rows.length > pageSize && (
            <div className="actions">
              <Button
                type="button"
                disabled={page === 0}
                onClick={() => setPage((value) => value - 1)}
              >
                {t.previous}
              </Button>
              <Button
                type="button"
                disabled={(page + 1) * pageSize >= rows.length}
                onClick={() => setPage((value) => value + 1)}
              >
                {t.more}
              </Button>
            </div>
          )}
        </section>
      )}
    </div>
  );
}
