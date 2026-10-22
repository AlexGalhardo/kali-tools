import { Button } from "@base-ui/react/button";
import { useEffect, useState } from "react";
import type { PageProps } from "../../app/shell";
import {
  backend,
  connected,
  type NeighborTable,
} from "../../shared/api/bridge";
import { Empty } from "../../shared/ui/Empty";
import { Field } from "../../shared/ui/Field";

export function NeighborsTable({
  t,
  busy,
  setError,
}: Pick<PageProps, "t" | "busy" | "setError">) {
  const [query, setQuery] = useState({ family: "all" });
  const [page, setPage] = useState(0);
  const [loading, setLoading] = useState(false);
  const [table, setTable] = useState<NeighborTable>({
    neighbors: [],
    limited: false,
    provider: "",
  });
  useEffect(() => {
    if (!connected()) return;
    let active = true;
    setLoading(true);
    backend()
      .Neighbors(query.family)
      .then((value) => {
        if (active) {
          setTable(value);
          setPage(0);
        }
      })
      .catch((error) => {
        if (active) {
          setTable({ neighbors: [], limited: false, provider: "" });
          setError(String(error));
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [query, setError]);
  const states: Record<string, string> = {
    reachable: t.state_reachable,
    stale: t.state_stale,
    delay: t.state_delay,
    probe: t.state_probe,
    incomplete: t.state_incomplete,
    unreachable: t.state_unreachable,
    permanent: t.state_permanent,
    unknown: t.state_unknown,
  };
  const pages = Math.max(1, Math.ceil(table.neighbors.length / 100));
  const tableLabel = `${t.neighbors}: ${t.address}`;
  return (
    <section className="card form" aria-label={t.neighbors} aria-busy={loading}>
      <div className="card-title">
        <h2>{t.neighbors}</h2>
      </div>
      <p className="description">{t.neighborsIntro}</p>
      <div className="actions">
        <Field label={t.family}>
          <select
            value={query.family}
            disabled={busy || loading}
            onChange={(event) => setQuery({ family: event.target.value })}
          >
            <option value="all">{t.familyAll}</option>
            <option value="ipv4">IPv4</option>
            <option value="ipv6">IPv6</option>
          </select>
        </Field>
        <Button
          className="secondary"
          disabled={!connected() || busy || loading}
          onClick={() => setQuery({ ...query })}
        >
          {loading ? t.loading : t.refresh}
        </Button>
      </div>
      {table.provider && <p className="footnote">{table.provider}</p>}
      {table.limited && (
        <p role="status" className="notice">
          {t.neighborsLimited}
        </p>
      )}
      {/* biome-ignore lint/a11y/noNoninteractiveTabindex: Keyboard users must be able to scroll the neighbor table. */}
      <section className="table-wrap" aria-label={tableLabel} tabIndex={0}>
        <table>
          <thead>
            <tr>
              {[t.interface, t.address, "MAC", t.state, t.router].map(
                (label) => (
                  <th key={label}>{label}</th>
                ),
              )}
            </tr>
          </thead>
          <tbody>
            {table.neighbors.slice(page * 100, (page + 1) * 100).map((row) => (
              <tr key={`${row.interface_index}:${row.address}`}>
                <td>{row.interface_name || `#${row.interface_index}`}</td>
                <td className="mono">{row.address}</td>
                <td className="mono">{row.mac || "—"}</td>
                <td>{states[row.state] ?? t.state_unknown}</td>
                <td>{row.router ? t.router : "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
      {table.neighbors.length === 0 && (
        <Empty text={loading ? t.loading : t.noNeighbors} />
      )}
      <div className="pagination">
        <Button
          className="secondary"
          disabled={page === 0 || loading}
          onClick={() => setPage((value) => value - 1)}
        >
          {t.previous}
        </Button>
        <span>
          {t.page} {page + 1} / {pages}
        </span>
        <Button
          className="secondary"
          disabled={page + 1 >= pages || loading}
          onClick={() => setPage((value) => value + 1)}
        >
          {t.more}
        </Button>
      </div>
    </section>
  );
}
