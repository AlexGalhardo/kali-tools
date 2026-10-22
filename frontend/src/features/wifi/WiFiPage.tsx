import { Button } from "@base-ui/react/button";
import { useCallback, useEffect, useRef, useState } from "react";
import type { PageProps } from "../../app/shell";
import {
  backend,
  connected,
  transportKind,
  type WiFi,
} from "../../shared/api/bridge";
import { Field } from "../../shared/ui/Field";
import { Stat } from "../../shared/ui/Stat";
import { LocationConsentNotice } from "./LocationConsentNotice";
export function WiFiPage({ t, busy, execute, stateText }: PageProps) {
  const [wifi, setWifi] = useState<WiFi>(),
    [secret, setSecret] = useState(""),
    [secretState, setSecretState] = useState(""),
    [needsConsent, setNeedsConsent] = useState(false);
  const revealEpoch = useRef(0);
  const requested = useRef(false);
  const consentFailure = useCallback((error: unknown) => {
    if (!String(error).includes("location_consent_required:")) return false;
    revealEpoch.current++;
    setSecret("");
    setSecretState("");
    setWifi(undefined);
    setNeedsConsent(true);
    return true;
  }, []);
  const checkAgain = useCallback(
    () =>
      execute(async () => {
        try {
          setWifi(await backend().CurrentWiFi());
          setNeedsConsent(false);
        } catch (error) {
          if (!consentFailure(error)) throw error;
        }
      }),
    [execute, consentFailure],
  );
  useEffect(() => {
    if (connected() && !requested.current) {
      requested.current = true;
      void checkAgain();
    }
  }, [checkAgain]);
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
  if (needsConsent)
    return (
      <section className="card form">
        <div className="card-title">
          <h2>{t.wifi}</h2>
        </div>
        <LocationConsentNotice
          t={t}
          busy={busy}
          canOpen={transportKind() === "wails"}
          onOpen={() => void execute(() => backend().OpenLocationSettings())}
          onRetry={() => void checkAgain()}
        />
      </section>
    );
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
              try {
                const s = await backend().RevealWiFi();
                if (epoch === revealEpoch.current) {
                  setSecret(s.value);
                  setSecretState(s.state);
                }
              } catch (error) {
                if (!consentFailure(error)) throw error;
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
