import { Button } from "@base-ui/react/button";
import { useMemo, useState } from "react";
import type { PageProps } from "../../app/shell";
import {
  backend,
  connected,
  type TLSAuditReport,
} from "../../shared/api/bridge";
import { Field } from "../../shared/ui/Field";

export function TLSAuditPanel({
  t,
  busy,
  execute,
}: Pick<PageProps, "t" | "busy" | "execute">) {
  const [host, setHost] = useState("localhost");
  const [port, setPort] = useState(443);
  const [sni, setSNI] = useState("");
  const [lint, setLint] = useState(true);
  const [report, setReport] = useState<TLSAuditReport>();
  const [page, setPage] = useState(0);
  const findings = useMemo(() => {
    const rank: Record<string, number> = {
      fatal: 0,
      error: 1,
      warn: 2,
      info: 3,
    };
    return [...(report?.findings ?? [])].sort(
      (a, b) =>
        rank[a.severity] - rank[b.severity] ||
        a.cert_index - b.cert_index ||
        a.id.localeCompare(b.id),
    );
  }, [report]);
  const statuses: Record<string, string> = {
    accepted: t.tls_accepted,
    rejected: t.tls_rejected,
    client_unsupported: t.tls_client_unsupported,
    timeout: t.tls_timeout,
    error: t.tls_error,
  };
  const severities: Record<string, string> = {
    fatal: t.severity_fatal,
    error: t.severity_error,
    warn: t.severity_warn,
    info: t.severity_info,
  };
  return (
    <div className="tls-audit">
      <form
        className="card form"
        onSubmit={(e) => {
          e.preventDefault();
          void execute(async () => {
            setReport(undefined);
            setPage(0);
            setReport(
              await backend().AuditTLS({
                host,
                port,
                server_name: sni,
                lint,
                timeout_ms: 5000,
              }),
            );
          });
        }}
      >
        <p className="description">{t.auditIntro}</p>
        <div className="form-grid">
          <Field label={t.target}>
            <input
              required
              maxLength={253}
              value={host}
              onChange={(e) => setHost(e.target.value)}
            />
          </Field>
          <Field label={t.port}>
            <input
              required
              type="number"
              min={1}
              max={65535}
              value={port}
              onChange={(e) => setPort(Number(e.target.value))}
            />
          </Field>
          <Field label={t.serverName}>
            <input
              maxLength={253}
              value={sni}
              onChange={(e) => setSNI(e.target.value)}
            />
          </Field>
          <Field label={t.runLint}>
            <input
              type="checkbox"
              checked={lint}
              onChange={(e) => setLint(e.target.checked)}
            />
          </Field>
        </div>
        <Button
          type="submit"
          className="primary"
          disabled={busy || !connected()}
        >
          {busy ? t.loading : t.auditRun}
        </Button>
      </form>
      {report && (
        <>
          <section className="card">
            <h2>{t.tlsAudit}</h2>
            <dl>
              <dt>{t.negotiated}</dt>
              <dd>{report.negotiated_version}</dd>
              <dt>{t.cipher}</dt>
              <dd>{report.cipher_suite}</dd>
              <dt>{t.verifiedBy}</dt>
              <dd>
                {report.verified ? t.verified : t.unverified} ({report.verifier}
                ) {report.verify_error}
              </dd>
              <dt>{t.ocspStapled}</dt>
              <dd>{String(report.ocsp_stapled)}</dd>
              <dt>{t.scts}</dt>
              <dd>{report.scts.embedded + report.scts.tls_extension}</dd>
              <dt>ALPN</dt>
              <dd>{report.alpn || "—"}</dd>
            </dl>
          </section>
          <section className="card">
            <h2>{t.protocolMatrix}</h2>
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>{t.kind}</th>
                    <th>{t.state}</th>
                    <th>{t.cipher}</th>
                  </tr>
                </thead>
                <tbody>
                  {report.protocols.map((p) => (
                    <tr key={p.version}>
                      <td>{p.version}</td>
                      <td>{statuses[p.status] ?? p.status}</td>
                      <td>{p.cipher || "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
          <section className="card">
            <h2>{t.chain}</h2>
            {report.chain.map((c) => (
              <details key={c.index}>
                <summary>
                  {c.index}: {c.subject}
                </summary>
                <dl>
                  <dt>{t.issuer}</dt>
                  <dd>{c.issuer}</dd>
                  <dt>{t.validFrom}</dt>
                  <dd>{String(c.not_before)}</dd>
                  <dt>{t.validUntil}</dt>
                  <dd>{String(c.not_after)}</dd>
                  <dt>SAN</dt>
                  <dd>
                    {c.sans.join(", ")}
                    {c.sans_limited && "…"}
                  </dd>
                  <dt>{t.cipher}</dt>
                  <dd>
                    {c.key_algorithm} {c.key_bits} / {c.signature_algorithm}
                  </dd>
                  <dt>SHA-256</dt>
                  <dd>{c.sha256}</dd>
                  <dt>AIA / OCSP / CRL</dt>
                  <dd>
                    {[
                      ...c.aia_issuers,
                      ...c.ocsp_servers,
                      ...c.crl_distribution_points,
                    ].join(", ") || "—"}
                  </dd>
                </dl>
              </details>
            ))}
          </section>
          <section className="card">
            <h2>{t.findings}</h2>
            {report.findings_limited && (
              <p role="status">{t.findingsLimited}</p>
            )}
            <p>{report.lint_engine}</p>
            {findings.length === 0 ? (
              <p>{t.auditNoFindings}</p>
            ) : (
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>{t.state}</th>
                      <th>ID</th>
                      <th>{t.chain}</th>
                      <th>{t.auditDetail}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {findings.slice(page * 100, (page + 1) * 100).map((f) => (
                      <tr key={`${f.source}:${f.id}:${f.cert_index}`}>
                        <td>{severities[f.severity] ?? f.severity}</td>
                        <td>
                          {f.source}: {f.id}
                        </td>
                        <td>{f.cert_index}</td>
                        <td>{f.detail}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {findings.length > 100 && (
              <div className="actions">
                <Button disabled={page === 0} onClick={() => setPage(page - 1)}>
                  {t.previous}
                </Button>
                <span>
                  {t.page} {page + 1} / {Math.ceil(findings.length / 100)}
                </span>
                <Button
                  disabled={(page + 1) * 100 >= findings.length}
                  onClick={() => setPage(page + 1)}
                >
                  {t.more}
                </Button>
              </div>
            )}
          </section>
        </>
      )}
    </div>
  );
}
