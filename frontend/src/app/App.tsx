import { Button } from "@base-ui/react/button";
import { useCallback, useEffect, useState } from "react";
import { CapturePage } from "../features/capture/CapturePage";
import { DiagnosticsPage } from "../features/diagnostics/DiagnosticsPage";
import { DiscoveryPage } from "../features/discovery/DiscoveryPage";
import { HistoryPage } from "../features/history/HistoryPage";
import { InterfacesPage } from "../features/interfaces/InterfacesPage";
import { NeighborsTable } from "../features/neighbors/NeighborsTable";
import { OverviewPage } from "../features/overview/OverviewPage";
import { SettingsPage } from "../features/settings/SettingsPage";
import { WiFiPage } from "../features/wifi/WiFiPage";
import {
  backend,
  type Capability,
  connected,
  type Interface,
  type Inventory,
  onEvent,
  type Settings,
  transportKind,
} from "../shared/api/bridge";
import { en, pt } from "../shared/i18n/i18n";
import { NavIcon } from "../shared/ui/Icon";
import { type Page, pages } from "./shell";

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
    closed: t.portClosed,
    filtered: t.portFiltered,
    up: t.hostUp,
    down: t.hostDown,
    unknown: t.hostUnknown,
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
  const pageProps = { t, busy, execute, stateText, setError, setNotice };
  const nav = (p: Page) => {
    if (p !== page) {
      setError("");
      setNotice("");
    }
    setPage(p);
  };
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
          <small>0.7.0-dev.2</small>
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
          {connected() && transportKind() === "http" && (
            <div className="notice">{t.serverMode}</div>
          )}
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
            <>
              <InterfacesPage
                {...pageProps}
                interfaces={interfaces}
                inventory={inventory}
              />
              <NeighborsTable {...pageProps} />
            </>
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
          {page === "toolkit" && <DiscoveryPage {...pageProps} />}
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
