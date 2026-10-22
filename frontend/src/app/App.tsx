import { Button } from "@base-ui/react/button";
import { useCallback, useEffect, useState } from "react";
import { CapturePage } from "../features/capture/CapturePage";
import { DiagnosticsPage } from "../features/diagnostics/DiagnosticsPage";
import { HistoryPage } from "../features/history/HistoryPage";
import { InterfacesPage } from "../features/interfaces/InterfacesPage";
import { OverviewPage } from "../features/overview/OverviewPage";
import { SettingsPage } from "../features/settings/SettingsPage";
import { WiFiPage } from "../features/wifi/WiFiPage";
import {
  backend,
  type Capability,
  connected,
  type Interface,
  type Inventory,
  type Job,
  onEvent,
  type Settings,
  type Tool,
} from "../shared/api/bridge";
import { en, pt } from "../shared/i18n/i18n";
import { download } from "../shared/lib/download";
import { Empty } from "../shared/ui/Empty";
import { Field } from "../shared/ui/Field";
import { NavIcon } from "../shared/ui/Icon";

type Page =
  | "dashboard"
  | "interfaces"
  | "diagnostics"
  | "wifi"
  | "packets"
  | "toolkit"
  | "history"
  | "settings";
const pages: Page[] = [
  "dashboard",
  "interfaces",
  "diagnostics",
  "wifi",
  "packets",
  "toolkit",
  "history",
  "settings",
];
export function App() {
  const [page, setPage] = useState<Page>("dashboard"),
    [settings, setSettings] = useState<Settings>({
      language: "en",
      retention_days: 30,
    });
  const t = settings.language === "pt-BR" ? pt : en;
  const labels: Record<string, string> = {
    completed: t.completed,
    failed: t.failed,
    cancelled: t.cancelled,
    queued: t.queued,
    running: t.running,
    available: t.available,
    permission_required: t.restricted,
    location_consent_required: t.locationConsent,
    unsupported: t.unsupported,
    dependency_missing: t.dependencyMissing,
    open_network: t.openNetwork,
    enterprise: t.enterprise,
    no_saved_key: t.noSavedKey,
    no_connection: t.noConnection,
    connected: t.connected,
    open: t.open,
    unreachable: t.unreachable,
  };
  const stateText = (state: string) => labels[state] ?? state;
  const errorLabels: Record<string, string> = {
    permission_required: t.permissionDenied,
    location_consent_required: t.locationConsentError,
    conflict: t.conflictError,
    unauthorized: t.unauthorizedError,
    forbidden: t.forbiddenError,
    unsupported: t.unsupported,
    dependency_missing: t.dependencyMissing,
    no_connection: t.noConnection,
    invalid_input: t.invalidInputError,
    timeout: t.timeoutError,
    not_found: t.notFoundError,
    certificate_invalid: t.certificateError,
    cancelled: t.cancelled,
    failed: t.error,
  };
  const errorText = (value: string) => {
    const code = value.match(/(?:^|\s)([a-z_]+):/)?.[1];
    return code && errorLabels[code] ? errorLabels[code] : value;
  };
  const [interfaces, setInterfaces] = useState<Interface[]>([]),
    [inventory, setInventory] = useState<Inventory>({
      adapters: [],
      counters: [],
    }),
    [capabilities, setCapabilities] = useState<Capability[]>([]),
    [sampleTime, setSampleTime] = useState(""),
    [rates, setRates] = useState<number[][]>([]),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [notice, setNotice] = useState("");
  const [packetCount, setPacketCount] = useState(0);
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
  const execute = useCallback(async (fn: () => Promise<void>) => {
    setError("");
    setNotice("");
    setBusy(true);
    try {
      await fn();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  }, []);
  const refresh = useCallback(async () => {
    const [list, data, supported] = await Promise.all([
      backend().Interfaces(),
      backend().Inventory(),
      backend().Capabilities(),
    ]);
    setInterfaces(list);
    setInventory(data);
    setCapabilities(supported);
  }, []);
  useEffect(() => {
    document.documentElement.lang = settings.language;
  }, [settings.language]);
  useEffect(() => {
    if (!connected()) return;
    execute(async () => {
      await refresh();
      setSettings(await backend().Settings());
    });
  }, [execute, refresh]);
  useEffect(
    () =>
      onEvent<{ timestamp: string; receive_bps: number; send_bps: number }>(
        "metrics",
        (data) => {
          setSampleTime(data.timestamp);
          setRates((values) => [
            ...values.slice(-59),
            [data.receive_bps, data.send_bps],
          ]);
        },
      ),
    [],
  );
  useEffect(() => {
    if (!connected()) return;
    return onEvent<{ job: Job }>("job", (e) => {
      if (e.job.kind === "discovery" || e.job.kind === "nmap") {
        setDiscoveryJob(e.job.state === "running" ? e.job.id : "");
      }
    });
  }, []);
  useEffect(() => {
    setError("");
    setNotice("");
    if (!connected()) return;
    if (page === "toolkit")
      execute(async () => setTools(await backend().Tools()));
  }, [page, execute]);
  const pageProps = { t, busy, execute, stateText, setError, setNotice };
  const nav = (p: Page) => setPage(p);
  return (
    <div className="shell">
      <aside>
        <Button className="brand" onClick={() => nav("dashboard")}>
          <span className="brand-mark">
            k<span>_</span>
          </span>
          <span>
            kali<span className="muted">-tools</span>
            <small>{t.networkWorkspace}</small>
          </span>
        </Button>
        <nav aria-label={t.navigation}>
          {pages.map((p) => (
            <Button
              key={p}
              className={`nav-item ${page === p ? "selected" : ""}`}
              aria-current={page === p ? "page" : undefined}
              onClick={() => nav(p)}
            >
              <span aria-hidden="true">
                <NavIcon page={p} />
              </span>
              {t[p]}
              {p === "packets" && packetCount > 0 && (
                <small>{packetCount}</small>
              )}
            </Button>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <span className="status-dot" /> {t.development}
          <small>0.4.0</small>
        </div>
      </aside>
      <div className="workspace">
        <header>
          <span className="breadcrumb">
            {t.workspace} <span>/</span> <strong>{t[page]}</strong>
          </span>
          <div className="header-actions">
            <span className="local-badge">
              <span className="status-dot" />
              {connected() ? t.connected : t.disconnected}
            </span>
            <select
              aria-label={t.language}
              value={settings.language}
              onChange={(e) =>
                setSettings({
                  ...settings,
                  language: e.target.value as Settings["language"],
                })
              }
            >
              <option value="en">EN</option>
              <option value="pt-BR">PT-BR</option>
            </select>
          </div>
        </header>
        <main>
          <div className="page-heading">
            <div>
              <h1>{page === "dashboard" ? t.subtitle : t[page]}</h1>
              <p className="description">
                {page === "dashboard"
                  ? t.overviewIntro
                  : page === "wifi"
                    ? t.wifiIntro
                    : page === "packets"
                      ? t.captureIntro
                      : page === "history" || page === "settings"
                        ? t.privacy
                        : t.permissions}
              </p>
            </div>
            <Button
              className="secondary"
              disabled={!connected() || busy}
              onClick={() => execute(refresh)}
            >
              <NavIcon page="refresh" /> {t.refresh}
            </Button>
          </div>
          {!connected() && <div className="notice">{t.native}</div>}
          {error && (
            <div role="alert" className="error">
              {t.error}: {errorText(error)}
            </div>
          )}
          {notice && (
            <div role="status" className="notice">
              {notice}
            </div>
          )}
          {page === "dashboard" && (
            <OverviewPage
              {...pageProps}
              interfaces={interfaces}
              capabilities={capabilities}
              rates={rates}
              sampleTime={sampleTime}
              settings={settings}
              nav={nav}
            />
          )}
          {page === "interfaces" && (
            <InterfacesPage
              {...pageProps}
              interfaces={interfaces}
              inventory={inventory}
            />
          )}
          {page === "diagnostics" && (
            <DiagnosticsPage {...pageProps} errorLabels={errorLabels} />
          )}
          {page === "wifi" && <WiFiPage {...pageProps} />}
          {page === "packets" && (
            <CapturePage
              {...pageProps}
              interfaces={interfaces}
              onPacketCount={setPacketCount}
              settings={settings}
            />
          )}
          {page === "toolkit" && (
            <>
              <section className="card form">
                <p className="description">{t.scanIntro}</p>
                <div className="form-grid">
                  <Field label={t.scope}>
                    <input
                      value={cidr}
                      onChange={(e) => setCIDR(e.target.value)}
                    />
                  </Field>
                  <Field label={t.ports}>
                    <input
                      value={ports}
                      onChange={(e) => setPorts(e.target.value)}
                    />
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
                        (tool) =>
                          tool.name === "nmap" && tool.state === "available",
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
                          setInspectionProvider(
                            `${value.scanner} ${value.version}`,
                          );
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
                {inspectionProvider && (
                  <p className="footnote">{inspectionProvider}</p>
                )}
                <div className="table-wrap">
                  <table>
                    <thead>
                      <tr>
                        {[
                          t.target,
                          t.port,
                          t.state,
                          t.service,
                          t.detectionMethod,
                        ].map((label) => (
                          <th key={label}>{label}</th>
                        ))}
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
          )}
          {page === "history" && (
            <HistoryPage {...pageProps} settings={settings} />
          )}
          {page === "settings" && (
            <SettingsPage
              {...pageProps}
              settings={settings}
              setSettings={setSettings}
            />
          )}
        </main>
        <footer>
          <span>
            <span className="status-dot" /> {t.privacy}
          </span>
          <span>GO + WAILS</span>
        </footer>
      </div>
    </div>
  );
}
