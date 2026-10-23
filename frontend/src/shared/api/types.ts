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
  dns_ms?: number | null;
  connect_ms?: number | null;
  tls_ms?: number | null;
  ttfb_ms?: number | null;
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
export type LocalScope = { interface: string; cidr: string };
export type LocalDiscoveryRequest = {
  scope: LocalScope;
  protocols: string[];
  duration_seconds: number;
  service_types?: string[];
  ssdp_target?: string;
};
export type LocalResponder = {
  protocol: string;
  address: string;
  port?: number;
  name: string;
  host?: string;
  service_type?: string;
  txt?: string[];
  server?: string;
  location?: string;
};
export type LocalDiscoveryResult = {
  responders: LocalResponder[];
  limited: boolean;
  out_of_scope: number;
  malformed: number;
  errors: string[];
};
export type SNMPRequest = {
  scope: LocalScope;
  target: string;
  port: number;
  owned: boolean;
  version: string;
  community?: string;
  v3?: {
    user: string;
    auth_protocol: string;
    auth_pass?: string;
    priv_protocol?: string;
    priv_pass?: string;
  };
  timeout_ms: number;
  interfaces: boolean;
};
export type SNMPInterface = {
  index: number;
  descr: string;
  oper_status: string;
  speed_bps: number;
  mac: string;
};
export type SNMPResult = {
  target: string;
  system: Record<string, string> | null;
  interfaces: SNMPInterface[] | null;
  limited: boolean;
};
export type Backend = {
  AuditTLS(r: TLSAuditRequest): Promise<TLSAuditReport>;
  InspectDNS(r: DNSInspectRequest): Promise<DNSInspectReport>;
  DiscoverLocal(r: LocalDiscoveryRequest): Promise<LocalDiscoveryResult>;
  QuerySNMP(r: SNMPRequest): Promise<SNMPResult>;
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

export type DNSInspectRequest = {
  name: string;
  type: string;
  resolvers?: string[];
  doh?: string[];
  dot?: string[];
  checks: string[];
  trust_anchors?: string[];
  timeout_ms: number;
};
export type DNSResolverAnswer = {
  resolver: string;
  transport: string;
  status: string;
  rcode?: string;
  ad: boolean;
  answers: string[];
  min_ttl?: number | null;
  elapsed_ms: number;
};
export type DNSInspectReport = {
  name: string;
  type: string;
  compare?: {
    results: DNSResolverAnswer[];
    agree: boolean;
    groups: string[][];
  } | null;
  soa_ns?: {
    zone: string;
    servers: {
      ns: string;
      address: string;
      status: string;
      serial?: number | null;
      ns_set: string[];
      aa: boolean;
    }[];
    serials_agree: boolean;
    ns_sets_agree: boolean;
    verdict: string;
  } | null;
  dnssec?: {
    links: {
      zone: string;
      ds_matches: boolean;
      key_tag: number;
      algorithm: string;
      sig_valid: boolean;
      status: string;
      detail?: string;
    }[];
    answer_sig: boolean;
    verdict: string;
    resolver: string;
  } | null;
  encrypted?: {
    endpoint: string;
    protocol: string;
    status: string;
    http_code?: number;
    rcode?: string;
    elapsed_ms: number;
  }[];
  queries: number;
  limited: boolean;
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
