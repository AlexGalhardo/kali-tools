import { Button } from "@base-ui/react/button";
import type { Dispatch, SetStateAction } from "react";
import type { PageProps } from "../../app/shell";
import { backend, connected, type Settings } from "../../shared/api/bridge";
import { Field } from "../../shared/ui/Field";

type Props = PageProps & {
  settings: Settings;
  setSettings: Dispatch<SetStateAction<Settings>>;
};
export function SettingsPage({
  t,
  busy,
  execute,
  setNotice,
  settings,
  setSettings,
}: Props) {
  return (
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
        <Button
          disabled={!connected() || busy}
          onClick={() =>
            execute(async () => {
              const saved = await backend().SaveDiagnosticBundle();
              if (saved) setNotice(t.diagnosticSaved);
            })
          }
        >
          {t.diagnosticBundle}
        </Button>
      </div>
    </section>
  );
}
