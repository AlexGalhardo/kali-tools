import { Button } from "@base-ui/react/button";
import { Tabs } from "@base-ui/react/tabs";
import { useEffect, useState } from "react";
import type { PageProps } from "../../app/shell";
import {
  backend,
  connected,
  type DiagnosticResult,
  type Job,
  onEvent,
} from "../../shared/api/bridge";
import { Empty } from "../../shared/ui/Empty";
import { Field } from "../../shared/ui/Field";
import { NavIcon } from "../../shared/ui/Icon";
import { TLSAuditPanel } from "../tls/TLSAuditPanel";
import { HTTPMetadataView } from "./HTTPMetadataView";

type Props = PageProps & { errorLabels: Record<string, string> };
export function DiagnosticsPage({
  t,
  busy,
  execute,
  stateText,
  errorLabels,
}: Props) {
  const [kind, setKind] = useState("tcp"),
    [recordType, setRecordType] = useState(""),
    [target, setTarget] = useState("localhost"),
    [port, setPort] = useState(443),
    [timeout, setTimeoutMS] = useState(3000),
    [redirects, setRedirects] = useState(0),
    [diagnostic, setDiagnostic] = useState<DiagnosticResult>(),
    [activeJob, setActiveJob] = useState("");
  const diagnosticRun = () =>
    execute(async () => {
      const request = {
        kind,
        target,
        port,
        timeout_ms: timeout,
        record_type: kind === "dns" ? recordType : "",
        redirects: kind === "http" ? redirects : 0,
      };
      const j = await backend().StartDiagnostic(request);
      setActiveJob(j.id);
    });
  // Result events stay in memory; history stores only redacted state.
  useEffect(
    () =>
      onEvent<{ job: Job; result?: DiagnosticResult }>("job", (e) => {
        if (e.result) setDiagnostic(e.result);
        if (["completed", "failed", "cancelled"].includes(e.job.state))
          setActiveJob((id) => (id === e.job.id ? "" : id));
      }),
    [],
  );
  return (
    <Tabs.Root defaultValue="diagnostics">
      <Tabs.List className="actions">
        <Tabs.Tab value="diagnostics">{t.diagnostics}</Tabs.Tab>
        <Tabs.Tab value="audit">{t.tlsAudit}</Tabs.Tab>
      </Tabs.List>
      <Tabs.Panel value="diagnostics">
        <section className="card form">
          <div className="form-grid">
            <Field label={t.kind}>
              <select value={kind} onChange={(e) => setKind(e.target.value)}>
                {["tcp", "dns", "http", "tls", "ping", "traceroute"].map(
                  (k) => (
                    <option key={k}>{k}</option>
                  ),
                )}
              </select>
            </Field>
            {kind === "dns" && (
              <Field label={t.dnsRecordType}>
                <select
                  value={recordType}
                  onChange={(e) => setRecordType(e.target.value)}
                >
                  <option value="">{t.dnsAddresses}</option>
                  {["A", "AAAA", "CNAME", "MX", "NS", "TXT", "PTR"].map((r) => (
                    <option key={r}>{r}</option>
                  ))}
                </select>
              </Field>
            )}
            <Field label={t.target}>
              <input
                value={target}
                onChange={(e) => setTarget(e.target.value)}
                placeholder={
                  kind === "http" ? "https://example.com" : "example.com"
                }
              />
            </Field>
            <Field label={t.port}>
              <input
                type="number"
                min="1"
                max="65535"
                value={port}
                onChange={(e) => setPort(Number(e.target.value))}
              />
            </Field>
            <Field label={t.timeout}>
              <input
                type="number"
                min="100"
                max="30000"
                value={timeout}
                onChange={(e) => setTimeoutMS(Number(e.target.value))}
              />
            </Field>
            {kind === "http" && (
              <Field label={t.traceRedirects}>
                <input
                  type="number"
                  min="0"
                  max="5"
                  value={redirects}
                  onChange={(e) => setRedirects(Number(e.target.value))}
                />
              </Field>
            )}
          </div>
          <div className="actions">
            <Button
              disabled={busy || !connected() || Boolean(activeJob)}
              onClick={diagnosticRun}
            >
              {busy ? t.loading : t.run} <NavIcon page="forward" />
            </Button>
            {activeJob && (
              <Button
                className="secondary"
                onClick={() =>
                  execute(async () => {
                    await backend().CancelJob(activeJob);
                  })
                }
              >
                {t.cancel}
              </Button>
            )}
          </div>
        </section>
        <section className="card">
          <div className="card-title">
            <h2>{t.results}</h2>
          </div>
          {diagnostic ? (
            <div className="result">
              <span
                className={`pill ${diagnostic.status === "completed" ? "green" : ""}`}
              >
                {stateText(diagnostic.status)}
              </span>{" "}
              <span>{diagnostic.elapsed_ms.toFixed(2)} ms</span>
              <pre>{diagnostic.values.join("\n")}</pre>
              {diagnostic.http && (
                <HTTPMetadataView http={diagnostic.http} t={t} />
              )}
              {diagnostic.error_code && (
                <p role="alert">
                  {errorLabels[diagnostic.error_code] ?? t.error}
                </p>
              )}
            </div>
          ) : (
            <Empty text={activeJob ? t.loading : t.empty} />
          )}
        </section>
      </Tabs.Panel>
      <Tabs.Panel value="audit">
        <TLSAuditPanel t={t} busy={busy} execute={execute} />
      </Tabs.Panel>
    </Tabs.Root>
  );
}
