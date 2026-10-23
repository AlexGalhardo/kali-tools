import { Button } from "@base-ui/react/button";
import { useState } from "react";
import type { PageProps } from "../../app/shell";
import {
  backend,
  connected,
  type DNSInspectReport,
} from "../../shared/api/bridge";
import { Field } from "../../shared/ui/Field";

const recordTypes = [
  "A",
  "AAAA",
  "CNAME",
  "MX",
  "NS",
  "TXT",
  "SOA",
  "CAA",
  "DS",
  "DNSKEY",
];

function splitList(value: string): string[] {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function dnsLabel(t: PageProps["t"], value: string): string {
  const key = `dns_${value}` as keyof PageProps["t"];
  const label = t[key];
  return typeof label === "string" ? label : value;
}

export function DNSInspectPanel({
  t,
  busy,
  execute,
}: Pick<PageProps, "t" | "busy" | "execute">) {
  const [name, setName] = useState("example.com");
  const [type, setType] = useState("A");
  const [resolvers, setResolvers] = useState("");
  const [doh, setDoH] = useState("");
  const [dot, setDoT] = useState("");
  const [compare, setCompare] = useState(true);
  const [soa, setSoa] = useState(false);
  const [dnssec, setDnssec] = useState(false);
  const [encrypted, setEncrypted] = useState(false);
  const [report, setReport] = useState<DNSInspectReport>();
  const checks = [
    compare && "compare",
    soa && "soa_ns",
    dnssec && "dnssec",
    encrypted && "encrypted",
  ].filter((item): item is string => Boolean(item));
  return (
    <div className="dns-inspect">
      <form
        className="card form"
        onSubmit={(e) => {
          e.preventDefault();
          void execute(async () => {
            setReport(undefined);
            setReport(
              await backend().InspectDNS({
                name,
                type,
                resolvers: splitList(resolvers),
                doh: splitList(doh),
                dot: splitList(dot),
                checks,
                timeout_ms: 2000,
              }),
            );
          });
        }}
      >
        <p className="description">{t.dnsIntro}</p>
        <div className="form-grid">
          <Field label={t.target}>
            <input
              required
              maxLength={253}
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </Field>
          <Field label={t.dnsRecord}>
            <select value={type} onChange={(e) => setType(e.target.value)}>
              {recordTypes.map((item) => (
                <option key={item}>{item}</option>
              ))}
            </select>
          </Field>
          <Field label={t.resolvers}>
            <input
              value={resolvers}
              maxLength={1024}
              onChange={(e) => setResolvers(e.target.value)}
            />
          </Field>
          <Field label={t.doh}>
            <input
              value={doh}
              maxLength={1024}
              onChange={(e) => setDoH(e.target.value)}
            />
          </Field>
          <Field label={t.dot}>
            <input
              value={dot}
              maxLength={1024}
              onChange={(e) => setDoT(e.target.value)}
            />
          </Field>
          <div className="field">
            <span>{t.checks}</span>
            <label>
              <input
                type="checkbox"
                checked={compare}
                onChange={(e) => setCompare(e.target.checked)}
              />{" "}
              {t.checkCompare}
            </label>
            <label>
              <input
                type="checkbox"
                checked={soa}
                onChange={(e) => setSoa(e.target.checked)}
              />{" "}
              {t.checkSoaNs}
            </label>
            <label>
              <input
                type="checkbox"
                checked={dnssec}
                onChange={(e) => setDnssec(e.target.checked)}
              />{" "}
              {t.checkDnssec}
            </label>
            <label>
              <input
                type="checkbox"
                checked={encrypted}
                onChange={(e) => setEncrypted(e.target.checked)}
              />{" "}
              {t.checkEncrypted}
            </label>
          </div>
        </div>
        <Button
          type="submit"
          className="primary"
          disabled={busy || !connected() || checks.length === 0}
        >
          {busy ? t.loading : t.dnsRun}
        </Button>
      </form>
      {report?.compare && (
        <section className="card">
          <h2>{t.checkCompare}</h2>
          <p>
            <span
              className={`pill ${report.compare.agree ? "green" : "amber"}`}
            >
              {report.compare.agree ? t.agree : t.disagree}
            </span>
          </p>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{t.resolvers}</th>
                  <th>{t.state}</th>
                  <th>{t.dnsRecord}</th>
                </tr>
              </thead>
              <tbody>
                {report.compare.results.map((row) => (
                  <tr key={row.resolver}>
                    <td>{row.resolver}</td>
                    <td>{dnsLabel(t, row.status)}</td>
                    <td>{row.answers.join("; ") || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
      {report?.soa_ns && (
        <section className="card">
          <h2>{t.checkSoaNs}</h2>
          <p>
            {t.dnsZone}: {report.soa_ns.zone}{" "}
            <span className="pill">{dnsLabel(t, report.soa_ns.verdict)}</span>
          </p>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{t.dnsNameservers}</th>
                  <th>{t.state}</th>
                  <th>{t.dnsSerial}</th>
                </tr>
              </thead>
              <tbody>
                {report.soa_ns.servers.map((server) => (
                  <tr key={`${server.ns}-${server.address}`}>
                    <td>
                      {server.ns} {server.address}
                    </td>
                    <td>{dnsLabel(t, server.status)}</td>
                    <td>{server.serial ?? "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
      {report?.dnssec && (
        <section className="card">
          <h2>{t.dnsChain}</h2>
          <p className="description">{t.dnsIntro}</p>
          <ol>
            {report.dnssec.links.map((link) => (
              <li key={link.zone}>
                {link.zone}{" "}
                <span
                  className={`pill ${link.status === "secure" ? "green" : "amber"}`}
                >
                  {dnsLabel(t, link.status)}
                </span>
              </li>
            ))}
          </ol>
        </section>
      )}
      {report?.encrypted && report.encrypted.length > 0 && (
        <section className="card">
          <h2>{t.checkEncrypted}</h2>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{t.dnsEndpoint}</th>
                  <th>{t.dnsProtocol}</th>
                  <th>{t.state}</th>
                </tr>
              </thead>
              <tbody>
                {report.encrypted.map((row) => (
                  <tr key={`${row.protocol}-${row.endpoint}`}>
                    <td>{row.endpoint}</td>
                    <td>{row.protocol}</td>
                    <td>{dnsLabel(t, row.status)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </div>
  );
}
