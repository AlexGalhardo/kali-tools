import type { HTTPMetadata } from "../../shared/api/types";
import type { en } from "../../shared/i18n/i18n";

type Copy = typeof en;

function phase(value: number | null | undefined, empty: string) {
  return value == null ? empty : `${value.toFixed(1)} ms`;
}

export function HTTPMetadataView({ http, t }: { http: HTTPMetadata; t: Copy }) {
  const chain: Record<string, string> = {
    not_requested: t.chainNotRequested,
    complete: t.chainComplete,
    loop: t.chainLoop,
    limit: t.chainLimit,
    invalid_target: t.chainInvalid,
    failed: t.chainFailed,
    followed: t.hopFollowed,
    invalid: t.chainInvalid,
  };
  const notes: Record<string, string> = {
    ignored_over_http: t.noteIgnoredHTTP,
    downgrade: t.flagDowngrade,
    location_truncated: t.chainInvalid,
  };
  const flags = [
    http.redirect?.self_loop ? t.flagSelfLoop : "",
    http.redirect?.cross_host ? t.flagCrossHost : "",
    http.redirect?.downgrade ? t.flagDowngrade : "",
  ].filter(Boolean);
  return (
    <div className="http-meta">
      <h3>{t.phases}</h3>
      <table>
        <tbody>
          {(
            [
              [t.phaseDNS, http.phases.dns_ms],
              [t.phaseConnect, http.phases.connect_ms],
              [t.phaseTLS, http.phases.tls_ms],
              [t.phaseTTFB, http.phases.ttfb_ms],
            ] as const
          ).map(([label, value]) => (
            <tr key={label}>
              <th scope="row">{label}</th>
              <td>{phase(value, t.phaseUnavailable)}</td>
            </tr>
          ))}
          <tr>
            <th scope="row">{t.phaseTotal}</th>
            <td>{http.phases.total_ms.toFixed(1)} ms</td>
          </tr>
        </tbody>
      </table>
      <h3>{t.securityHeaders}</h3>
      <table>
        <tbody>
          {http.security_headers.map((header) => (
            <tr key={header.name}>
              <th scope="row">{header.name}</th>
              <td>{header.present ? t.headerPresent : t.headerMissing}</td>
              <td>
                {header.value ? (
                  <code>{header.value}</code>
                ) : (
                  t.phaseUnavailable
                )}
                {header.notes?.map((note) => (
                  <span key={note}> {notes[note] ?? note}</span>
                ))}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <h3>{t.redirectTarget}</h3>
      <p>
        <code>{http.redirect?.location || t.phaseUnavailable}</code>
      </p>
      {flags.length > 0 && (
        <p>
          {flags.map((flag) => (
            <span key={flag} className="pill">
              {flag}
            </span>
          ))}
        </p>
      )}
      <h3>{t.redirectChain}</h3>
      <p>{chain[http.chain_status ?? ""] ?? http.chain_status}</p>
      {http.chain && http.chain.length > 0 && (
        <table>
          <tbody>
            {http.chain.map((hop) => (
              <tr key={hop.url}>
                <td>
                  <code>{hop.url}</code>
                </td>
                <td>{hop.status_code ?? t.phaseUnavailable}</td>
                <td>{chain[hop.status] ?? hop.status}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
