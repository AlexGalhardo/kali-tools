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
} from "./bridge";
import { NavIcon } from "./Icon";
import { en, pt } from "./i18n";

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
  const [interfaces, setInterfaces] = useState<Interface[]>([]),
    [inventory, setInventory] = useState<Inventory>({
      adapters: [],
      counters: [],
    }),
    [rates, setRates] = useState<number[][]>([]),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [notice, setNotice] = useState("");
  const [kind, setKind] = useState("tcp"),
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
    [capturing, setCapturing] = useState(false);
  const [cidr, setCIDR] = useState("192.168.1.0/24"),
    [ports, setPorts] = useState("80,443"),
    [discovery, setDiscovery] = useState<
      { host: string; port: number; state: string }[]
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
    const [list, data] = await Promise.all([
      backend().Interfaces(),
      backend().Inventory(),
    ]);
    setInterfaces(list);
    setInventory(data);
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
      onEvent<{ receive_bps: number; send_bps: number }>("metrics", (data) => {
        setRates((values) => [
          ...values.slice(-59),
          [data.receive_bps, data.send_bps],
        ]);
      }),
    [],
  );
  useEffect(() => {
    if (!connected()) return;
    return onEvent<{ job: Job }>("job", (e) => {
      setJobs((v) =>
        [e.job, ...v.filter((j) => j.id !== e.job.id)].slice(0, 200),
      );
      if (["completed", "failed", "cancelled"].includes(e.job.state))
        setActiveJob((v) => (v === e.job.id ? "" : v));
    });
  }, []);
  useEffect(
    () =>
      onEvent<{ result: { packets: Packet[] }; error: string }>(
        "capture",
        (e) => {
          setCapturing(false);
          setPackets(e.result.packets ?? []);
          setPacketPage(0);
          if (e.error) setError(e.error);
        },
      ),
    [],
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
      const request = { kind, target, port, timeout_ms: timeout };
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
            h<span>_</span>
          </span>
          <span>
            hacker<span className="muted">tools</span>
            <small>NETWORK WORKSPACE</small>
          </span>
        </Button>
        <nav aria-label="Main navigation">
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
          <small>0.1.0-alpha.1-dev</small>
        </div>
      </aside>
      <div className="workspace">
        <header>
          <span className="breadcrumb">
            Workspace <span>/</span> <strong>{t[page]}</strong>
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
              {t.error}: {error}
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
                    {new Date().toLocaleTimeString(settings.language)}
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
                  {[t.diagnostics, t.wifi, t.packets].map((name, i) => (
                    <div className="capability-row" key={name}>
                      <span>{name}</span>
                      <span className={`pill ${i === 0 ? "green" : "amber"}`}>
                        {i === 0 ? t.available : t.restricted}
                      </span>
                    </div>
                  ))}
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
                    <span className="pill green">{diagnostic.status}</span>{" "}
                    <span>{diagnostic.elapsed_ms.toFixed(2)} ms</span>
                    <pre>{diagnostic.values.join("\n")}</pre>
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
                  {wifi?.state ?? t.unsupported}
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
              {secretState && <p role="status">{secretState}</p>}
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
                    <input
                      value={captureAddress}
                      onChange={(e) => setCaptureAddress(e.target.value)}
                      placeholder="192.168.1.10"
                    />
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
                    disabled={!connected() || capturing || busy}
                    onClick={() =>
                      execute(async () => {
                        await backend().StartCapture({
                          address: captureAddress,
                          duration_seconds: duration,
                          packet_limit: limit,
                        });
                        setCapturing(true);
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
                </div>
              </section>
              <section className="card">
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
                  <span>{filtered.length} packets</span>
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
                      execute(async () =>
                        setDiscovery(
                          await backend().Discover(
                            cidr,
                            ports.split(",").map(Number),
                          ),
                        ),
                      )
                    }
                  >
                    {busy ? t.loading : t.discover}
                  </Button>
                  <Button
                    className="secondary"
                    disabled={discovery.length === 0}
                    onClick={() => download(discovery)}
                  >
                    {t.download}
                  </Button>
                </div>
                {discovery.length > 0 && (
                  <pre>
                    {discovery
                      .map((r) => `${r.host}:${r.port}  ${r.state}`)
                      .join("\n")}
                  </pre>
                )}
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
                    <span className="pill">{tool.state}</span>
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
                          <span className="pill">{j.state}</span>
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
    <label className="field" htmlFor={id}>
      <span>{label}</span>
      {cloneElement(children, { id })}
    </label>
  );
}
