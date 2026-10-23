export type Interface = {
  index: number;
  name: string;
  mac: string;
  mtu: number;
  up: boolean;
  loopback: boolean;
  addresses: string[];
};
// States: available, dependency_missing, permission_required, location_consent_required, unsupported, failed.
export type Capability = {
  id: string;
  state: string;
  detail: string;
  provider: string;
};
export type Counter = { id: string; received: number; sent: number };
export type Neighbor = {
  address: string;
  mac: string;
  family: string;
  interface_index: number;
  interface_name: string;
  state: string;
  router: boolean;
};
export type NeighborTable = {
  neighbors: Neighbor[];
  limited: boolean;
  provider: string;
};
export type Inventory = {
  adapters: {
    index: number;
    name: string;
    gateways: string[];
    dns: string[];
  }[];
  counters: Counter[];
};
export type DiagnosticRequest = {
  record_type?: string;
  kind: string;
  target: string;
  port: number;
  timeout_ms: number;
  redirects?: number;
};
export type HTTPPhases = {
  dns_ms: number | null;
  connect_ms: number | null;
  tls_ms: number | null;
  ttfb_ms: number | null;
  total_ms: number;
};
export type SecurityHeader = {
  name: string;
  present: boolean;
  value?: string;
  notes?: string[];
};
export type RedirectTarget = {
  location?: string;
  valid: boolean;
  self_loop: boolean;
  cross_host: boolean;
  downgrade: boolean;
  notes?: string[];
};
export type RedirectHop = { url: string; status_code?: number; status: string };
export type HTTPMetadata = {
  status_code?: number;
  final_url?: string;
  phases: HTTPPhases;
  security_headers: SecurityHeader[];
  redirect?: RedirectTarget;
  chain?: RedirectHop[];
  chain_status?: string;
};
export type DiagnosticResult = {
  error_code?: string;
  kind: string;
  target: string;
  status: string;
  elapsed_ms: number;
  values: string[];
  provider: string;
  http?: HTTPMetadata;
};
export type Job = {
  id: string;
  kind: string;
  state: string;
  created: string;
  finished?: string;
  summary: string;
};
export type Settings = { language: string; retention_days: number };
export type Packet = {
  number: number;
  timestamp: string;
  length: number;
  source: string;
  destination: string;
  protocol: string;
  info: string;
};
export type Capture = { packets: Packet[]; limited: boolean; format: string };
export type WiFi = {
  channel?: number;
  profile: string;
  ssid: string;
  bssid: string;
  signal: number;
  authentication: number;
  state: string;
};
export type Tool = {
  name: string;
  state: string;
  version: string;
  path: string;
};
export type ServiceInspection = {
  scanner: string;
  version: string;
  hosts: {
    addresses: { address: string; type: string }[];
    ports: {
      port: number;
      state: { state: string };
      service: {
        name: string;
        product: string;
        version: string;
        method: string;
      };
    }[];
  }[];
};
export type Backend = {
  AuditTLS(r: TLSAuditRequest): Promise<TLSAuditReport>;
  Version(): Promise<string>;
  Capabilities(): Promise<Capability[]>;
  CaptureBackends(): Promise<Capability[]>;
  OpenNpcapDownload(): Promise<void>;
  Interfaces(): Promise<Interface[]>;
  Neighbors(family: string): Promise<NeighborTable>;
  Inventory(): Promise<Inventory>;
  Diagnose(r: DiagnosticRequest): Promise<DiagnosticResult>;
  StartDiagnostic(r: DiagnosticRequest): Promise<Job>;
  CancelJob(id: string): Promise<boolean>;
  History(): Promise<Job[]>;
  ClearHistory(): Promise<void>;
  Settings(): Promise<Settings>;
  SaveSettings(v: Settings): Promise<void>;
  CurrentWiFi(): Promise<WiFi>;
  RevealWiFi(): Promise<{ state: string; value: string }>;
  OpenLocationSettings(): Promise<void>;
  Tools(): Promise<Tool[]>;
  Discover(
    cidr: string,
    ports: number[],
  ): Promise<{ host: string; port: number; state: string }[]>;
  InspectServices(cidr: string, ports: number[]): Promise<ServiceInspection>;
  ImportCapture(): Promise<Capture>;
  SaveCapture(format: string): Promise<boolean>;
  StartCapture(r: {
    backend?: string;
    address: string;
    duration_seconds: number;
    packet_limit: number;
  }): Promise<void>;
  StopCapture(): Promise<void>;
};

export type TLSAuditRequest = {
  host: string;
  port: number;
  server_name: string;
  timeout_ms: number;
  lint: boolean;
};
export type TLSAuditCertificate = {
  index: number;
  subject: string;
  issuer: string;
  serial_hex: string;
  not_before: string;
  not_after: string;
  days_remaining: number;
  sans: string[];
  sans_limited: boolean;
  key_algorithm: string;
  key_bits: number;
  signature_algorithm: string;
  is_ca: boolean;
  self_signed: boolean;
  aia_issuers: string[];
  ocsp_servers: string[];
  crl_distribution_points: string[];
  embedded_scts: number;
  sha256: string;
};
export type TLSAuditReport = {
  host: string;
  port: number;
  server_name: string;
  negotiated_version: string;
  cipher_suite: string;
  alpn: string;
  verified: boolean;
  verify_error?: string;
  verifier: string;
  ocsp_stapled: boolean;
  scts: { embedded: number; tls_extension: number };
  chain: TLSAuditCertificate[];
  protocols: { version: string; status: string; cipher?: string }[];
  findings: {
    source: string;
    id: string;
    severity: string;
    cert_index: number;
    detail?: string;
  }[];
  findings_limited: boolean;
  lint_engine?: string;
};
declare global {
  interface Window {
    go?: { wailsui?: { Bindings?: Backend } };
    runtime?: {
      EventsOn(name: string, callback: (data: unknown) => void): () => void;
    };
  }
}

export type Topic = "job" | "metrics" | "capture";
export type TransportKind = "wails" | "http";
