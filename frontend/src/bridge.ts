import * as generated from "../wailsjs/go/main/App";
export type Interface = {
  index: number;
  name: string;
  mac: string;
  mtu: number;
  up: boolean;
  loopback: boolean;
  addresses: string[];
};
export type Counter = { id: string; received: number; sent: number };
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
  kind: string;
  target: string;
  port: number;
  timeout_ms: number;
};
export type DiagnosticResult = {
  kind: string;
  target: string;
  status: string;
  elapsed_ms: number;
  values: string[];
  provider: string;
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
type Backend = {
  Version(): Promise<string>;
  Interfaces(): Promise<Interface[]>;
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
  Tools(): Promise<Tool[]>;
  Discover(
    cidr: string,
    ports: number[],
  ): Promise<{ host: string; port: number; state: string }[]>;
  ImportCapture(): Promise<Capture>;
  StartCapture(r: {
    address: string;
    duration_seconds: number;
    packet_limit: number;
  }): Promise<void>;
  StopCapture(): Promise<void>;
};
declare global {
  interface Window {
    go?: { main: { App: Backend } };
    runtime?: {
      EventsOn(name: string, callback: (data: unknown) => void): () => void;
    };
  }
}
export const connected = () => Boolean(window.go?.main?.App);
export function backend(): Backend {
  const value = window.go?.main?.App;
  if (!value)
    throw new Error("Native bridge unavailable. Open the desktop application.");
  return generated;
}
export function onEvent<T>(name: string, fn: (data: T) => void) {
  return (
    window.runtime?.EventsOn(name, (value) => fn(value as T)) ?? (() => {})
  );
}
