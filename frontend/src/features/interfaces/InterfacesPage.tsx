import type { PageProps } from "../../app/shell";
import type { Interface, Inventory } from "../../shared/api/bridge";

type Props = PageProps & { interfaces: Interface[]; inventory: Inventory };
export function InterfacesPage({ t, interfaces, inventory }: Props) {
  return (
    <section className="card table-wrap">
      <table>
        <thead>
          <tr>
            {[t.interface, t.address, "MAC", t.status, t.gateway, t.dns].map(
              (s) => (
                <th key={s}>{s}</th>
              ),
            )}
          </tr>
        </thead>
        <tbody>
          {interfaces.map((i) => {
            const a = inventory.adapters.find((a) => a.index === i.index);
            return (
              <tr key={i.index}>
                <td>
                  <strong>{i.name}</strong>
                  <small>
                    MTU {i.mtu} · #{i.index}
                  </small>
                </td>
                <td className="mono">
                  {i.addresses.map((a) => (
                    <div key={a}>{a}</div>
                  ))}
                </td>
                <td className="mono">{i.mac || "—"}</td>
                <td>
                  <span className={`pill ${i.up ? "green" : ""}`}>
                    {i.up ? t.connected : t.disconnected}
                  </span>
                </td>
                <td className="mono">{a?.gateways?.join(", ") || "—"}</td>
                <td className="mono">{a?.dns?.join(", ") || "—"}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </section>
  );
}
