import { Button } from "@base-ui/react/button";
import { useEffect, useState } from "react";
import type { PageProps } from "../../app/shell";
import {
  backend,
  connected,
  type Job,
  onEvent,
  type Tool,
} from "../../shared/api/bridge";
import { download } from "../../shared/lib/download";
import { Empty } from "../../shared/ui/Empty";
import { Field } from "../../shared/ui/Field";
export function DiscoveryPage({
  t,
  busy,
  execute,
  stateText,
  setError,
}: PageProps) {
  const [cidr, setCIDR] = useState("192.168.1.0/24"),
    [ports, setPorts] = useState("80,443"),
    [discoveryPage, setDiscoveryPage] = useState(0),
    [discoveryJob, setDiscoveryJob] = useState(""),
    [inspectionProvider, setInspectionProvider] = useState(""),
    [discovery, setDiscovery] = useState<
      {
        host: string;
        port: number;
        state: string;
        service?: string;
        method?: string;
      }[]
    >([]),
    [tools, setTools] = useState<Tool[]>([]);
  useEffect(() => {
    if (!connected()) return;
    void execute(async () => setTools(await backend().Tools()));
    return onEvent<{ job: Job }>("job", (e) => {
      if (e.job.kind === "discovery" || e.job.kind === "nmap") {
        setDiscoveryJob(e.job.state === "running" ? e.job.id : "");
      }
    });
  }, [execute]);
  return (
    <>
      <section className="card form">
        <p className="description">{t.scanIntro}</p>
        <div className="form-grid">
          <Field label={t.scope}>
            <input value={cidr} onChange={(e) => setCIDR(e.target.value)} />
          </Field>
          <Field label={t.ports}>
            <input value={ports} onChange={(e) => setPorts(e.target.value)} />
          </Field>
        </div>
        <div className="actions">
          <Button
            disabled={!connected() || busy}
            onClick={() =>
              execute(async () => {
                setDiscoveryPage(0);
                setDiscovery([]);
                setInspectionProvider("");
                try {
                  setDiscovery(
                    await backend().Discover(
                      cidr,
                      ports.split(",").map(Number),
                    ),
                  );
                } finally {
                  setDiscoveryJob("");
                }
              })
            }
          >
            {busy ? t.loading : t.discover}
          </Button>
          <Button
            className="secondary"
            disabled={
              !connected() ||
              busy ||
              !tools.some(
                (tool) => tool.name === "nmap" && tool.state === "available",
              )
            }
            onClick={() =>
              execute(async () => {
                setDiscoveryPage(0);
                setDiscovery([]);
                setInspectionProvider("");
                try {
                  const value = await backend().InspectServices(
                    cidr,
                    ports.split(",").map(Number),
                  );
                  setInspectionProvider(`${value.scanner} ${value.version}`);
                  setDiscovery(
                    value.hosts.flatMap((host) =>
                      host.ports.map((port) => ({
                        host:
                          host.addresses.find((a) => a.type === "ipv4")
                            ?.address ?? "",
                        port: port.port,
                        state: port.state.state,
                        service: [
                          port.service.name,
                          port.service.product,
                          port.service.version,
                        ]
                          .filter(Boolean)
                          .join(" "),
                        method: port.service.method,
                      })),
                    ),
                  );
                } finally {
                  setDiscoveryJob("");
                }
              })
            }
          >
            {t.inspectServices}
          </Button>
          <Button
            className="secondary"
            disabled={!discoveryJob}
            onClick={() => {
              backend()
                .CancelJob(discoveryJob)
                .catch((e) => setError(String(e)));
            }}
          >
            {t.cancel}
          </Button>
          <Button
            className="secondary"
            disabled={discovery.length === 0}
            onClick={() =>
              download(
                inspectionProvider
                  ? { provider: inspectionProvider, results: discovery }
                  : discovery,
              )
            }
          >
            {t.download}
          </Button>
        </div>
        {inspectionProvider && <p className="footnote">{inspectionProvider}</p>}
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                {[t.target, t.port, t.state, t.service, t.detectionMethod].map(
                  (label) => (
                    <th key={label}>{label}</th>
                  ),
                )}
              </tr>
            </thead>
            <tbody>
              {discovery
                .slice(discoveryPage * 100, discoveryPage * 100 + 100)
                .map((r) => (
                  <tr key={`${r.host}:${r.port}`}>
                    <td className="mono">{r.host}</td>
                    <td>{r.port}</td>
                    <td>{stateText(r.state)}</td>
                    <td>{r.service || "?"}</td>
                    <td>{r.method || "?"}</td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
        {discovery.length === 0 && <Empty text={t.empty} />}
        <div className="pagination">
          <Button
            className="secondary"
            disabled={discoveryPage === 0}
            onClick={() => setDiscoveryPage((p) => p - 1)}
          >
            {t.previous}
          </Button>
          <span>
            {t.page} {discoveryPage + 1} /{" "}
            {Math.max(1, Math.ceil(discovery.length / 100))}
          </span>
          <Button
            className="secondary"
            disabled={(discoveryPage + 1) * 100 >= discovery.length}
            onClick={() => setDiscoveryPage((p) => p + 1)}
          >
            {t.more}
          </Button>
        </div>
      </section>
      <section className="card">
        <div className="card-title">
          <h2>{t.capabilities}</h2>
        </div>
        <p className="footnote">{t.toolsIntro}</p>
        {tools.map((tool) => (
          <div className="capability-row" key={tool.name}>
            <div>
              <strong>{tool.name}</strong>
              <small>{tool.version || "—"}</small>
            </div>
            <span className="pill">{stateText(tool.state)}</span>
          </div>
        ))}
      </section>
    </>
  );
}
