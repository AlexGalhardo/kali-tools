import { Button } from "@base-ui/react/button";
import { useEffect, useRef, useState } from "react";
import type { PageProps } from "../../app/shell";
import { backend, connected, type WiFi } from "../../shared/api/bridge";
import { Field } from "../../shared/ui/Field";
import { Stat } from "../../shared/ui/Stat";
export function WiFiPage({ t, busy, execute, stateText }: PageProps) {
  const [wifi, setWifi] = useState<WiFi>(),
    [secret, setSecret] = useState(""),
    [secretState, setSecretState] = useState("");
  const revealEpoch = useRef(0);
  useEffect(() => {
    if (connected())
      void execute(async () => setWifi(await backend().CurrentWiFi()));
  }, [execute]);
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
  return (
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
  );
}
