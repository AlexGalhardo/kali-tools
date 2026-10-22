import { Button } from "@base-ui/react/button";
import type { PageProps } from "../../app/shell";
import type { Capability, Interface, Settings } from "../../shared/api/bridge";
import { Empty } from "../../shared/ui/Empty";
import { NavIcon } from "../../shared/ui/Icon";
import { Stat } from "../../shared/ui/Stat";

type Props = PageProps & {
  interfaces: Interface[];
  capabilities: Capability[];
  rates: number[][];
  sampleTime: string;
  settings: Settings;
  nav: (page: "interfaces") => void;
};
export function OverviewPage({
  t,
  interfaces,
  capabilities,
  rates,
  sampleTime,
  settings,
  nav,
}: Props) {
  const latest = rates.at(-1) ?? [0, 0];
  const max = Math.max(1, ...rates.flat());
  return (
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
                stroke="var(--border)"
                strokeDasharray="3 6"
              />
            ))}
            {[0, 1].map((series) => (
              <polyline
                key={series}
                fill="none"
                stroke={
                  series === 0 ? "var(--accent)" : "var(--graph-secondary)"
                }
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
              ? new Date(sampleTime).toLocaleTimeString(settings.language)
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
              aria-label={t.viewInterfaces}
              onClick={() => nav("interfaces")}
            >
              <NavIcon page="open" />
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
  );
}
