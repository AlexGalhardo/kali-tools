import { Button } from "@base-ui/react/button";
import type { PageProps } from "../../app/shell";
import { LOCATION_SETTINGS_URI } from "../../shared/api/bridge";

type Props = Pick<PageProps, "t" | "busy"> & {
  canOpen: boolean;
  onOpen: () => void;
  onRetry: () => void;
};

export function LocationConsentNotice({
  t,
  busy,
  canOpen,
  onOpen,
  onRetry,
}: Props) {
  return (
    <>
      <p role="alert">{t.locationConsentRequired}</p>
      <code>{LOCATION_SETTINGS_URI}</code>
      <div className="actions">
        {canOpen && (
          <Button disabled={busy} onClick={onOpen}>
            {t.openLocationSettings}
          </Button>
        )}
        <Button className="secondary" disabled={busy} onClick={onRetry}>
          {t.checkAgain}
        </Button>
      </div>
    </>
  );
}
