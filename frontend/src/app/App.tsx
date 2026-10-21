import { Button } from "@base-ui/react/button";
import {
  cloneElement,
  type ReactElement,
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";
import {
  backend,
  type Capability,
  type Capture,
  connected,
  type DiagnosticResult,
  type Interface,
  type Inventory,
  type Job,
  onEvent,
  type Packet,
  type Settings,
  type Tool,
  type WiFi,
} from "../shared/api/bridge";
import { en, pt } from "../shared/i18n/i18n";
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
function download(value: unknown) {
  const blob = new Blob(
    [JSON.stringify({ schema_version: 1, data: value }, null, 2)],
    { type: "application/json" },
  );
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "hacker-tools-export.json";
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
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
  const [kind, setKind] = useState("tcp"),
    [recordType, setRecordType] = useState(""),
    [target, setTarget] = useState("localhost"),
    [port, setPort] = useState(443),
    [timeout, setTimeoutMS] = useState(3000),
    [diagnostic, setDiagnostic] = useState<DiagnosticResult>(),
    [jobs, setJobs] = useState<Job[]>([]),
    [activeJob, setActiveJob] = useState("");
  const [wifi, setWifi] = useState<WiFi>(),
    [secret, setSecret] = useState(""),
    [secretState, setSecretState] = useState(""),
    [packets, setPackets] = useState<Packet[]>([]),
    [packetPage, setPacketPage] = useState(0),
    [filter, setFilter] = useState(""),
    [captureAddress, setCaptureAddress] = useState(""),
    [duration, setDuration] = useState(5),
    [limit, setLimit] = useState(1000),
    [capturing, setCapturing] = useState(false),
    [captureFormat, setCaptureFormat] = useState("pcapng");
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
  const revealEpoch = useRef(0);
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
      setJobs((v) =>
        [e.job, ...v.filter((j) => j.id !== e.job.id)].slice(0, 200),
      );
      if (e.job.kind === "discovery" || e.job.kind === "nmap") {
        setDiscoveryJob(e.job.state === "running" ? e.job.id : "");
      }
      if (["completed", "failed", "cancelled"].includes(e.job.state))
        setActiveJob((v) => (v === e.job.id ? "" : v));
    });
  }, []);
  useEffect(
    () =>
      onEvent<{
        result: { packets: Packet[]; limited?: boolean };
        error: string;
      }>("capture", (e) => {
        setCapturing(false);
        setPackets(e.result.packets ?? []);
        setPacketPage(0);
        if (e.error) setError(e.error);
        if (e.result.limited) setNotice(t.captureMemoryLimited);
      }),
    [t.captureMemoryLimited],
  );
  useEffect(() => {
    revealEpoch.current++;
    setSecret("");
    setSecretState("");
    setError("");
    setNotice("");
    if (!connected()) return;
    if (page === "wifi")
      execute(async () => setWifi(await backend().CurrentWiFi()));
    if (page === "history")
      execute(async () => setJobs(await backend().History()));
    if (page === "toolkit")
      execute(async () => setTools(await backend().Tools()));
  }, [page, execute]);
  useEffect(() => {
    const clear = () => {
      revealEpoch.current++;
      setSecret("");
    };
    window.addEventListener("blur", clear);
    document.addEventListener("visibilitychange", clear);
    return () => {
      clear();
      window.removeEventListener("blur", clear);
      document.removeEventListener("visibilitychange", clear);
    };
  }, []);
  useEffect(() => {
    if (!secret) return;
    const timer = setTimeout(() => setSecret(""), 15000);
    return () => clearTimeout(timer);
  }, [secret]);
  const latest = rates.at(-1) ?? [0, 0];
  const max = Math.max(1, ...rates.flat());
  const filtered = packets.filter((p) =>
    `${p.protocol} ${p.source} ${p.destination}`
      .toLowerCase()
      .includes(filter.toLowerCase()),
  );
  const totalPages = Math.max(1, Math.ceil(filtered.length / 100));
  const diagnosticRun = () =>
    execute(async () => {
      const request = {
        kind,
        target,
        port,
        timeout_ms: timeout,
        record_type: kind === "dns" ? recordType : "",
      };
      const j = await backend().StartDiagnostic(request);
      setActiveJob(j.id);
      setJobs((v) => [j, ...v.filter((v) => v.id !== j.id)]);
    });
  // Result events stay in memory; history stores only redacted state.
  useEffect(
    () =>
      onEvent<{ job: Job; result?: DiagnosticResult }>("job", (e) => {
        if (e.result) setDiagnostic(e.result);
      }),
    [],
  );
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
              {p === "packets" && packets.length > 0 && (
                <small>{packets.length}</small>
              )}
            </Button>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <span className="status-dot" /> {t.development}
          <small>0.3.0</small>
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
              ↻ {t.refresh}
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
            <>
              <div className="stats">
                <Stat
                  label={t.active}
                  value={String(interfaces.filter((i) => i.up).length)}
                  detail={`${interfaces.length} ${t.interfaces.toLowerCase()}`}
                />
                <Stat
                  label={t.received}
                  value={Math.round(latest[0]).toLocaleString()}
                  detail={t.bytes}
                />
                <Stat
                  label={t.sent}
                  value={Math.round(latest[1]).toLocaleString()}
                  detail={t.bytes}
                />
              </div>
              <section className="card traffic">
                <div className="card-title">
                  <h2>
                    {t.received} / {t.sent}
                  </h2>
                  <span className="live-label">
                    <span className="status-dot" />
                    {t.live}
                  </span>
                </div>
                <div
                  className="graph"
                  role="img"
                  aria-label={`${t.received}: ${Math.round(latest[0])} ${t.bytes}; ${t.sent}: ${Math.round(latest[1])} ${t.bytes}`}
                >
                  <svg viewBox="0 0 800 180" preserveAspectRatio="none">
                    <title>
                      {t.received} / {t.sent}
                    </title>
                    {[30, 75, 120, 165].map((y) => (
                      <line
                        key={y}
                        x1="0"
                        x2="800"
                        y1={y}
                        y2={y}
                        stroke="#243140"
                        strokeDasharray="3 6"
                      />
                    ))}
                    {[0, 1].map((series) => (
                      <polyline
                        key={series}
                        fill="none"
                        stroke={series === 0 ? "#54e4ad" : "#7f9bfa"}
                        strokeWidth="2"
                        points={rates
                          .map(
                            (r, i) =>
                              `${(i / 59) * 800},${170 - (r[series] / max) * 150}`,
                          )
                          .join(" ")}
                      />
                    ))}
                  </svg>
                  <span className="graph-unit">
                    {Math.round(max).toLocaleString()} {t.bytes}
                  </span>
                </div>
                <div className="graph-footer">
                  <span>−120 s</span>
                  <span>
                    {sampleTime
                      ? new Date(sampleTime).toLocaleTimeString(
                          settings.language,
                        )
                      : t.empty}
                  </span>
                </div>
              </section>
              <div className="two-column">
                <section className="card">
                  <div className="card-title">
                    <h2>{t.interfaces}</h2>
                    <Button
                      className="text-button"
                      onClick={() => nav("interfaces")}
                    >
                      ↗
                    </Button>
                  </div>
                  {interfaces.slice(0, 4).map((i) => (
                    <div className="interface-row" key={i.index}>
                      <span className="interface-icon">
                        <NavIcon page="interfaces" />
                      </span>
                      <div>
                        <strong>{i.name}</strong>
                        <small>{i.addresses[0] ?? t.none}</small>
                      </div>
                      <span className={`pill ${i.up ? "green" : ""}`}>
                        {i.up ? t.connected : t.disconnected}
                      </span>
                    </div>
                  ))}
                  {interfaces.length === 0 && <Empty text={t.empty} />}
                </section>
                <section className="card">
                  <div className="card-title">
                    <h2>{t.capabilities}</h2>
                  </div>
                  {(
                    [
                      ["diagnostics", t.diagnostics],
                      ["wifi", t.wifi],
                      ["capture_live", t.packets],
                    ] as const
                  ).map(([id, name]) => {
                    const capability = capabilities.find((c) => c.id === id);
                    const labels: Record<string, string> = {
                      available: t.available,
                      permission_required: t.restricted,
                      location_consent_required: t.locationConsent,
                      unsupported: t.unsupported,
                      dependency_missing: t.dependencyMissing,
                      failed: t.error,
                    };
                    return (
                      <div
                        className="capability-row"
                        key={id}
                        title={capability?.detail}
                      >
                        <span>{name}</span>
                        <span
                          className={`pill ${capability?.state === "available" ? "green" : ["permission_required", "location_consent_required"].includes(capability?.state ?? "") ? "amber" : ""}`}
                        >
                          {capability
                            ? (labels[capability.state] ?? t.unsupported)
                            : t.empty}
                        </span>
                      </div>
                    );
                  })}
                  <p className="footnote">{t.permissions}</p>
                </section>
              </div>
            </>
          )}
          {page === "interfaces" && (
            <section className="card table-wrap">
              <table>
                <thead>
                  <tr>
                    {[
                      t.interface,
                      t.address,
                      "MAC",
                      t.status,
                      t.gateway,
                      t.dns,
                    ].map((s) => (
                      <th key={s}>{s}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {interfaces.map((i) => {
                    const a = inventory.adapters.find(
                      (a) => a.index === i.index,
                    );
                    return (
                      <tr key={i.index}>
                        <td>
                          <strong>{i.name}</strong>
                          <small>
                            MTU {i.mtu} · #{i.index}
                          </small>
                        </td>
                        <td className="mono">
                          {i.addresses.map((a) => (
                            <div key={a}>{a}</div>
                          ))}
                        </td>
                        <td className="mono">{i.mac || "—"}</td>
                        <td>
                          <span className={`pill ${i.up ? "green" : ""}`}>
                            {i.up ? t.connected : t.disconnected}
                          </span>
                        </td>
                        <td className="mono">
                          {a?.gateways?.join(", ") || "—"}
                        </td>
                        <td className="mono">{a?.dns?.join(", ") || "—"}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </section>
          )}
          {page === "diagnostics" && (
            <>
              <section className="card form">
                <div className="form-grid">
                  <Field label={t.kind}>
                    <select
                      value={kind}
                      onChange={(e) => setKind(e.target.value)}
                    >
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
                        {["A", "AAAA", "CNAME", "MX", "NS", "TXT", "PTR"].map(
                          (r) => (
                            <option key={r}>{r}</option>
                          ),
                        )}
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
                </div>
                <div className="actions">
                  <Button
                    disabled={busy || !connected() || Boolean(activeJob)}
                    onClick={diagnosticRun}
                  >
                    {busy ? t.loading : t.run} →
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
            </>
          )}
          {page === "wifi" && (
            <section className="card form">
              <div className="card-title">
                <h2>{wifi?.ssid ?? t.wifi}</h2>
                <span className="pill green">
                  {wifi ? stateText(wifi.state) : t.unsupported}
                </span>
              </div>
              <div className="stats wifi-stats">
                <Stat label={t.profile} value={wifi?.profile ?? "—"} />
                <Stat label="BSSID" value={wifi?.bssid ?? "—"} />
                <Stat label={t.signal} value={wifi ? `${wifi.signal}%` : "—"} />
              </div>
              <Field label={t.password}>
                <output className="secret">{secret || "••••••••••••"}</output>
              </Field>
              {secretState && <p role="status">{stateText(secretState)}</p>}
              <div className="actions">
                <Button
                  disabled={!wifi || busy}
                  onClick={() =>
                    execute(async () => {
                      const epoch = ++revealEpoch.current;
                      const s = await backend().RevealWiFi();
                      if (epoch === revealEpoch.current) {
                        setSecret(s.value);
                        setSecretState(s.state);
                      }
                    })
                  }
                >
                  {t.reveal}
                </Button>
                <Button
                  className="secondary"
                  onClick={() => {
                    revealEpoch.current++;
                    setSecret("");
                    setSecretState("");
                  }}
                >
                  {t.hide}
                </Button>
              </div>
            </section>
          )}
          {page === "packets" && (
            <>
              <section className="card form">
                <div className="form-grid">
                  <Field label={t.captureAddress}>
                    <select
                      value={captureAddress}
                      onChange={(e) => setCaptureAddress(e.target.value)}
                    >
                      <option value="">{t.selectCaptureAddress}</option>
                      {interfaces
                        .filter((i) => i.up && !i.loopback)
                        .flatMap((i) =>
                          i.addresses
                            .filter((a) => !a.includes(":"))
                            .map((a) => (
                              <option
                                key={`${i.index}:${a}`}
                                value={a.split("/")[0]}
                              >
                                {i.name} ? {a}
                              </option>
                            )),
                        )}
                    </select>
                  </Field>
                  <Field label={t.duration}>
                    <input
                      type="number"
                      min="1"
                      max="60"
                      value={duration}
                      onChange={(e) => setDuration(Number(e.target.value))}
                    />
                  </Field>
                  <Field label={t.limit}>
                    <input
                      type="number"
                      min="1"
                      max="10000"
                      value={limit}
                      onChange={(e) => setLimit(Number(e.target.value))}
                    />
                  </Field>
                </div>
                <div className="actions">
                  <Button
                    disabled={
                      !connected() || capturing || busy || !captureAddress
                    }
                    onClick={() =>
                      execute(async () => {
                        setCapturing(true);
                        try {
                          await backend().StartCapture({
                            address: captureAddress,
                            duration_seconds: duration,
                            packet_limit: limit,
                          });
                        } catch (e) {
                          setCapturing(false);
                          throw e;
                        }
                      })
                    }
                  >
                    {capturing ? t.capturing : t.start}
                  </Button>
                  <Button
                    className="secondary"
                    disabled={!capturing}
                    onClick={() => execute(() => backend().StopCapture())}
                  >
                    {t.stop}
                  </Button>
                  <Button
                    className="secondary"
                    disabled={!connected() || busy || capturing}
                    onClick={() =>
                      execute(async () => {
                        const value: Capture = await backend().ImportCapture();
                        setPackets(value.packets);
                        if (value.limited) setNotice(t.captureLimited);
                        setPacketPage(0);
                      })
                    }
                  >
                    {t.import}
                  </Button>
                  <Button
                    className="secondary"
                    disabled={packets.length === 0}
                    onClick={() => download(packets)}
                  >
                    {t.download}
                  </Button>
                  <Field label={t.captureFormat}>
                    <select
                      value={captureFormat}
                      onChange={(e) => setCaptureFormat(e.target.value)}
                    >
                      <option value="pcapng">PCAPNG</option>
                      <option value="pcap">PCAP</option>
                    </select>
                  </Field>
                  <Button
                    className="secondary"
                    disabled={
                      !connected() || busy || capturing || packets.length === 0
                    }
                    onClick={() =>
                      execute(async () => {
                        if (await backend().SaveCapture(captureFormat))
                          setNotice(t.captureSaved);
                      })
                    }
                  >
                    {t.saveCapture}
                  </Button>
                </div>
                <p className="footnote">{t.captureSavePrivacy}</p>
              </section>
              <section className="card">
                <p className="footnote">{t.filterIntro}</p>
                <div className="table-toolbar">
                  <Field label={t.filter}>
                    <input
                      value={filter}
                      onChange={(e) => {
                        setFilter(e.target.value);
                        setPacketPage(0);
                      }}
                    />
                  </Field>
                  <span>
                    {filtered.length} {t.packetCount}
                  </span>
                </div>
                <div className="table-wrap">
                  <table>
                    <thead>
                      <tr>
                        {[
                          "#",
                          t.time,
                          t.source,
                          t.destination,
                          t.protocol,
                          t.length,
                          t.info,
                        ].map((s) => (
                          <th key={s}>{s}</th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {filtered
                        .slice(packetPage * 100, packetPage * 100 + 100)
                        .map((p) => (
                          <tr key={p.number}>
                            <td>{p.number}</td>
                            <td>
                              {new Date(p.timestamp).toLocaleTimeString(
                                settings.language,
                              )}
                            </td>
                            <td className="mono">{p.source}</td>
                            <td className="mono">{p.destination}</td>
                            <td>
                              <span className="pill">{p.protocol}</span>
                            </td>
                            <td>{p.length}</td>
                            <td>{p.info}</td>
                          </tr>
                        ))}
                    </tbody>
                  </table>
                </div>
                {filtered.length === 0 && <Empty text={t.empty} />}
                <div className="pagination">
                  <Button
                    className="secondary"
                    disabled={packetPage === 0}
                    onClick={() => setPacketPage((p) => p - 1)}
                  >
                    {t.previous}
                  </Button>
                  <span>
                    {t.page} {packetPage + 1} / {totalPages}
                  </span>
                  <Button
                    className="secondary"
                    disabled={packetPage + 1 >= totalPages}
                    onClick={() => setPacketPage((p) => p + 1)}
                  >
                    {t.more}
                  </Button>
                </div>
              </section>
            </>
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
            <section className="card">
              <div className="card-title">
                <h2>{t.history}</h2>
                <Button
                  className="secondary"
                  disabled={!connected() || busy}
                  onClick={() =>
                    execute(async () => {
                      await backend().ClearHistory();
                      setJobs(await backend().History());
                    })
                  }
                >
                  {t.clear}
                </Button>
              </div>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      {[t.time, t.kind, t.state, t.info].map((v) => (
                        <th key={v}>{v}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {jobs.map((j) => (
                      <tr key={j.id}>
                        <td>
                          {new Date(j.created).toLocaleString(
                            settings.language,
                          )}
                        </td>
                        <td>{j.kind}</td>
                        <td>
                          <span className="pill">{stateText(j.state)}</span>
                        </td>
                        <td>{j.summary}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {jobs.length === 0 && <Empty text={t.empty} />}
            </section>
          )}
          {page === "settings" && (
            <section className="card form">
              <div className="form-grid">
                <Field label={t.language}>
                  <select
                    value={settings.language}
                    onChange={(e) =>
                      setSettings({
                        ...settings,
                        language: e.target.value as Settings["language"],
                      })
                    }
                  >
                    <option value="en">English</option>
                    <option value="pt-BR">Português (Brasil)</option>
                  </select>
                </Field>
                <Field label={t.retention}>
                  <input
                    type="number"
                    min="1"
                    max="365"
                    value={settings.retention_days}
                    onChange={(e) =>
                      setSettings({
                        ...settings,
                        retention_days: Number(e.target.value),
                      })
                    }
                  />
                </Field>
              </div>
              <div className="actions">
                <Button
                  disabled={!connected() || busy}
                  onClick={() =>
                    execute(async () => {
                      await backend().SaveSettings(settings);
                      setNotice(t.saved);
                    })
                  }
                >
                  {t.save}
                </Button>
              </div>
            </section>
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
function Stat({
  label,
  value,
  detail,
}: {
  label: string;
  value: string;
  detail?: string;
}) {
  return (
    <div className="stat">
      <p>{label}</p>
      <strong>{value}</strong>
      {detail && <small>{detail}</small>}
    </div>
  );
}
function Empty({ text }: { text: string }) {
  return <div className="empty">{text}</div>;
}
function Field({
  label,
  children,
}: {
  label: string;
  children: ReactElement<{ id?: string }>;
}) {
  const id = useId();
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {cloneElement(children, { id })}
    </div>
  );
}
