import { Button } from "@base-ui/react/button";
import { useEffect, useState } from "react";
import type { PageProps } from "../../app/shell";
import {
  backend,
  type Capability,
  type Capture,
  connected,
  type Interface,
  onEvent,
  type Packet,
  type Settings,
  transportKind,
} from "../../shared/api/bridge";
import { download } from "../../shared/lib/download";
import { Empty } from "../../shared/ui/Empty";
import { Field } from "../../shared/ui/Field";

type Props = PageProps & {
  interfaces: Interface[];
  settings: Settings;
  onPacketCount: (count: number) => void;
};
export function CapturePage({
  t,
  busy,
  execute,
  setError,
  setNotice,
  interfaces,
  settings,
  onPacketCount,
}: Props) {
  const [packets, setPackets] = useState<Packet[]>([]),
    [packetPage, setPacketPage] = useState(0),
    [filter, setFilter] = useState(""),
    [captureAddress, setCaptureAddress] = useState(""),
    [duration, setDuration] = useState(5),
    [limit, setLimit] = useState(1000),
    [capturing, setCapturing] = useState(false),
    [captureBackend, setCaptureBackend] = useState("raw_socket"),
    [backends, setBackends] = useState<Capability[]>([]),
    [limits, setLimits] = useState<string[]>([]),
    [dropped, setDropped] = useState<number | null>(null),
    [captureFormat, setCaptureFormat] = useState("pcapng");
  useEffect(() => {
    if (!connected()) return;
    let active = true;
    backend()
      .CaptureBackends()
      .then((value) => {
        if (active) setBackends(value);
      })
      .catch((error) => {
        if (active) setError(error);
      });
    return () => {
      active = false;
    };
  }, [setError]);
  const choices = [
    { value: "raw_socket", id: "capture_live", label: t.backendRawSocket },
    { value: "pktmon", id: "capture_pktmon", label: t.backendPktmon },
    { value: "npcap", id: "capture_npcap", label: t.backendNpcap },
    { value: "libpcap", id: "capture_libpcap", label: t.backendLibpcap },
  ];
  const wideAddress =
    captureBackend === "npcap" || captureBackend === "libpcap";
  const selected = backends.find(
    (c) => c.id === choices.find((c) => c.value === captureBackend)?.id,
  );
  const installNpcap =
    backends.find((c) => c.id === "capture_npcap")?.state ===
    "dependency_missing";
  const limitLabels: Record<string, string> = {
    circular_buffer_may_overwrite_oldest: t.limitCircularBuffer,
    packets_truncated: t.limitTruncated,
    component_identity_lost: t.limitComponentIdentity,
    capture_scope_all_nics: t.limitAllNICs,
    duplicates_possible: t.limitDuplicates,
    drop_count_unavailable: t.limitDropCountMissing,
  };
  useEffect(
    () =>
      onEvent<{
        result: {
          packets: Packet[];
          limited?: boolean;
          limits?: string[];
          dropped?: number | null;
        };
        error: string;
      }>("capture", (e) => {
        setCapturing(false);
        setPackets(e.result.packets ?? []);
        setLimits(e.result.limits ?? []);
        setDropped(e.result.dropped ?? null);
        setPacketPage(0);
        if (e.error) setError(e.error);
        if (e.result.limited) setNotice(t.captureMemoryLimited);
      }),
    [t.captureMemoryLimited, setError, setNotice],
  );
  const filtered = packets.filter((p) =>
    `${p.protocol} ${p.source} ${p.destination}`
      .toLowerCase()
      .includes(filter.toLowerCase()),
  );
  const totalPages = Math.max(1, Math.ceil(filtered.length / 100));
  useEffect(
    () => onPacketCount(packets.length),
    [onPacketCount, packets.length],
  );
  return (
    <>
      <section className="card form">
        <div className="form-grid">
          <Field label={t.captureBackend}>
            <select
              value={captureBackend}
              disabled={capturing || busy}
              aria-describedby="capture-backend-detail"
              onChange={(e) => {
                setCaptureBackend(e.target.value);
                setCaptureAddress("");
              }}
            >
              {choices.map((choice) => {
                const status = backends.find((c) => c.id === choice.id);
                return (
                  <option
                    key={choice.value}
                    value={choice.value}
                    disabled={status?.state !== "available"}
                    title={status?.detail}
                  >
                    {choice.label}
                  </option>
                );
              })}
            </select>
          </Field>
          {captureBackend !== "pktmon" && (
            <Field
              label={wideAddress ? t.captureLocalAddress : t.captureAddress}
            >
              <select
                value={captureAddress}
                disabled={capturing || busy}
                onChange={(e) => setCaptureAddress(e.target.value)}
              >
                <option value="">{t.selectCaptureAddress}</option>
                {interfaces
                  .filter((i) => i.up && (wideAddress || !i.loopback))
                  .flatMap((i) =>
                    i.addresses
                      .filter((a) => wideAddress || !a.includes(":"))
                      .map((a) => (
                        <option key={`${i.index}:${a}`} value={a.split("/")[0]}>
                          {i.name} ? {a}
                        </option>
                      )),
                  )}
              </select>
            </Field>
          )}
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
        <p id="capture-backend-detail" className="footnote">
          {selected?.detail ?? t.selectCaptureBackend}
        </p>
        {installNpcap && (
          <p className="footnote">
            {transportKind() === "wails" ? (
              <Button
                className="secondary"
                disabled={busy}
                onClick={() => execute(() => backend().OpenNpcapDownload())}
              >
                {t.installNpcap}
              </Button>
            ) : (
              <a
                href="https://npcap.com/#download"
                target="_blank"
                rel="noreferrer"
              >
                https://npcap.com/#download
              </a>
            )}
          </p>
        )}
        <div className="actions">
          <Button
            disabled={
              !connected() ||
              capturing ||
              busy ||
              selected?.state !== "available" ||
              (captureBackend !== "pktmon" && !captureAddress)
            }
            onClick={() =>
              execute(async () => {
                setCapturing(true);
                setLimits([]);
                setDropped(null);
                try {
                  await backend().StartCapture({
                    backend: captureBackend,
                    address: captureBackend === "pktmon" ? "" : captureAddress,
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
                setLimits([]);
                setDropped(null);
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
            disabled={!connected() || busy || capturing || packets.length === 0}
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
        {dropped !== null && (
          <p>
            {t.droppedPackets}: {dropped}
          </p>
        )}
        {limits.length > 0 && (
          <ul className="footnote">
            {limits.map((code) => (
              <li key={code}>{limitLabels[code] ?? code}</li>
            ))}
          </ul>
        )}
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
        {/* biome-ignore lint/a11y/noNoninteractiveTabindex: Keyboard users must be able to scroll the packet table. */}
        <section className="table-wrap" aria-label={t.packets} tabIndex={0}>
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
        </section>
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
  );
}
