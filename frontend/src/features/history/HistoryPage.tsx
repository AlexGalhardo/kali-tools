import { Button } from "@base-ui/react/button";
import { useEffect, useState } from "react";
import type { PageProps } from "../../app/shell";
import {
  backend,
  connected,
  type Job,
  onEvent,
  type Settings,
} from "../../shared/api/bridge";
import { Empty } from "../../shared/ui/Empty";

type Props = PageProps & { settings: Settings };
export function HistoryPage({ t, busy, execute, stateText, settings }: Props) {
  const [jobs, setJobs] = useState<Job[]>([]);
  useEffect(() => {
    if (!connected()) return;
    void execute(async () => setJobs(await backend().History()));
    return onEvent<{ job: Job }>("job", (e) =>
      setJobs((values) =>
        [e.job, ...values.filter((j) => j.id !== e.job.id)].slice(0, 200),
      ),
    );
  }, [execute]);
  return (
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
                <td>{new Date(j.created).toLocaleString(settings.language)}</td>
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
  );
}
