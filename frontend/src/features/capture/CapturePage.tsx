import { Button } from "@base-ui/react/button";
import { useEffect, useState } from "react";
import type { PageProps } from "../../app/shell";
import {
  backend,
  type Capture,
  connected,
  type Interface,
  onEvent,
  type Packet,
  type Settings,
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
    [captureFormat, setCaptureFormat] = useState("pcapng");
  useEffect(
    () =>
      onEvent<{
        result: { packets: Packet[]; limited?: boolean };
        error: string;
      }>("capture", (e) => {
        setCapturing(false);
        setPackets(e.result.packets ?? []);
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
          <Field label={t.captureAddress}>
            <select
              value={captureAddress}
              onChange={(e) => setCaptureAddress(e.target.value)}
            >
              <option value="">{t.selectCaptureAddress}</option>
              {interfaces
                .filter((i) => i.up && !i.loopback)
                .flatMap((i) =>
                  i.addresses
                    .filter((a) => !a.includes(":"))
                    .map((a) => (
                      <option key={`${i.index}:${a}`} value={a.split("/")[0]}>
                        {i.name} ? {a}
                      </option>
                    )),
                )}
            </select>
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
            disabled={!connected() || capturing || busy || !captureAddress}
            onClick={() =>
              execute(async () => {
                setCapturing(true);
                try {
                  await backend().StartCapture({
                    address: captureAddress,
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
  );
}
