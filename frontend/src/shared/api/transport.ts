import * as generated from "../../../wailsjs/go/wailsui/Bindings";
import type { Backend, Job, Topic, TransportKind } from "./types";

export interface Transport {
  readonly kind: TransportKind;
  readonly backend: Backend;
  init(): Promise<boolean>;
  on(topic: Topic, fn: (data: unknown) => void): () => void;
}
export class WailsTransport implements Transport {
  readonly kind = "wails";
  readonly backend: Backend = generated;
  async init(): Promise<boolean> {
    return Boolean(window.go?.wailsui?.Bindings);
  }
  on(topic: Topic, fn: (data: unknown) => void): () => void {
    return window.runtime?.EventsOn(topic, fn) ?? (() => {});
  }
}

const maxCaptureBytes = 67108864;
const maxTrackedJobs = 64;
const terminalStates = new Set(["completed", "failed", "cancelled"]);
type Result<K extends keyof Backend> = Awaited<ReturnType<Backend[K]>>;

export class HttpTransport implements Transport {
  readonly kind = "http";
  private source: EventSource | undefined;
  private lastSeq = 0;
  private resyncing = false;
  private readonly jobs = new Set<string>();
  private readonly handlers: Record<Topic, Set<(data: unknown) => void>> = {
    job: new Set(),
    metrics: new Set(),
    capture: new Set(),
  };
  readonly backend: Backend = {
    Version: async () =>
      (await this.request<{ version: string }>("/api/version")).version,
    Capabilities: () =>
      this.request<Result<"Capabilities">>("/api/capabilities"),
    Interfaces: () => this.request<Result<"Interfaces">>("/api/interfaces"),
    Inventory: () => this.request<Result<"Inventory">>("/api/inventory"),
    Tools: () => this.request<Result<"Tools">>("/api/tools"),
    Diagnose: (request) => this.json("POST", "/api/diagnose", request),
    StartDiagnostic: async (request) => {
      const job = await this.json<Job>("POST", "/api/diagnostics", request);
      this.track(job);
      return job;
    },
    CancelJob: async (id) =>
      (
        await this.request<{ cancelled: boolean }>(
          `/api/jobs/${encodeURIComponent(id)}/cancel`,
          { method: "POST" },
        )
      ).cancelled,
    History: () => this.request<Result<"History">>("/api/history"),
    ClearHistory: () => this.request("/api/history", { method: "DELETE" }),
    Settings: () => this.request<Result<"Settings">>("/api/settings"),
    SaveSettings: (value) => this.json("PUT", "/api/settings", value),
    CurrentWiFi: () => this.request<Result<"CurrentWiFi">>("/api/wifi"),
    RevealWiFi: () =>
      this.request<Result<"RevealWiFi">>("/api/wifi/reveal", {
        method: "POST",
      }),
    OpenLocationSettings: async () => {
      throw new Error(
        "unsupported: location settings can only be opened by the desktop app",
      );
    },
    Discover: (cidr, ports) =>
      this.json("POST", "/api/discovery", { cidr, ports }),
    InspectServices: (cidr, ports) =>
      this.json("POST", "/api/inspect", { cidr, ports }),
    StartCapture: (request) => this.json("POST", "/api/capture/start", request),
    StopCapture: () => this.request("/api/capture/stop", { method: "POST" }),
    ImportCapture: () => this.importCapture(),
    SaveCapture: (format) => this.saveCapture(format),
  };

  async init(): Promise<boolean> {
    try {
      const token = location.hash.match(/^#token=([A-Za-z0-9_-]{43})$/)?.[1];
      if (token) {
        try {
          await this.json("POST", "/api/session", { token });
        } finally {
          history.replaceState(null, "", location.pathname + location.search);
        }
      }
      await this.request("/api/version");
      return true;
    } catch {
      return false;
    }
  }

  private async response(
    path: string,
    init: RequestInit = {},
  ): Promise<Response> {
    let response: Response;
    try {
      response = await fetch(path, { ...init, credentials: "same-origin" });
    } catch {
      throw new Error("no_connection: kali-tools server is unreachable");
    }
    if (!response.ok) {
      let value: unknown;
      try {
        value = await response.json();
      } catch {
        throw new Error("no_connection: unexpected response from server");
      }
      if (
        typeof value === "object" &&
        value !== null &&
        "error" in value &&
        typeof value.error === "object" &&
        value.error !== null &&
        "code" in value.error &&
        "message" in value.error &&
        typeof value.error.code === "string" &&
        typeof value.error.message === "string"
      ) {
        throw new Error(`${value.error.code}: ${value.error.message}`);
      }
      throw new Error("no_connection: unexpected response from server");
    }
    return response;
  }

  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const response = await this.response(path, init);
    if (
      response.status === 204 ||
      (response.status === 202 && !response.headers.has("Content-Type"))
    )
      return undefined as T;
    if (!response.headers.get("Content-Type")?.includes("application/json"))
      throw new Error("no_connection: unexpected response from server");
    try {
      return (await response.json()) as T;
    } catch {
      throw new Error("no_connection: unexpected response from server");
    }
  }
  private json<T>(method: string, path: string, body: unknown): Promise<T> {
    return this.request(path, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  }

  private importCapture(): Promise<Result<"ImportCapture">> {
    return new Promise((resolve, reject) => {
      const input = document.createElement("input");
      input.type = "file";
      input.accept = ".pcap,.pcapng";
      input.hidden = true;
      const empty = () => {
        input.remove();
        resolve({ packets: [], limited: false, format: "" });
      };
      input.oncancel = empty;
      input.onchange = async () => {
        const file = input.files?.[0];
        if (!file) {
          empty();
          return;
        }
        try {
          if (file.size > maxCaptureBytes)
            throw new Error("invalid_input: capture must be at most 64 MiB");
          resolve(
            await this.request("/api/capture/import", {
              method: "POST",
              headers: { "Content-Type": "application/octet-stream" },
              body: file,
            }),
          );
        } catch (error) {
          reject(error);
        } finally {
          input.remove();
        }
      };
      document.body.append(input);
      input.click();
    });
  }
  private async saveCapture(format: string): Promise<boolean> {
    const response = await this.response(
      `/api/capture/export?format=${encodeURIComponent(format)}`,
    );
    const url = URL.createObjectURL(await response.blob());
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `capture.${format}`;
    document.body.append(anchor);
    anchor.click();
    anchor.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    return true;
  }

  on(topic: Topic, fn: (data: unknown) => void): () => void {
    this.handlers[topic].add(fn);
    if (!this.source) {
      this.source = new EventSource("/api/events");
      for (const name of ["job", "metrics", "capture"] as const) {
        this.source.addEventListener(name, (event) =>
          this.receive(name, event as MessageEvent<string>),
        );
      }
      this.source.addEventListener("resync", () => {
        void this.resync();
      });
    }
    return () => {
      this.handlers[topic].delete(fn);
      if (Object.values(this.handlers).every((values) => values.size === 0)) {
        this.source?.close();
        this.source = undefined;
      }
    };
  }
  private receive(topic: Topic, event: MessageEvent<string>) {
    let value: unknown;
    try {
      value = JSON.parse(event.data);
    } catch {
      return;
    }
    const seq = Number(event.lastEventId);
    const gap = this.lastSeq > 0 && seq > this.lastSeq + 1;
    if (Number.isSafeInteger(seq) && seq > this.lastSeq) this.lastSeq = seq;
    if (
      topic === "job" &&
      typeof value === "object" &&
      value !== null &&
      "job" in value
    )
      this.track(value.job as Job);
    this.dispatch(topic, value);
    if (gap) void this.resync();
  }
  private track(job: Job) {
    if (terminalStates.has(job.state)) {
      this.jobs.delete(job.id);
      return;
    }
    this.jobs.add(job.id);
    if (this.jobs.size > maxTrackedJobs) {
      const oldest = this.jobs.values().next().value;
      if (oldest) this.jobs.delete(oldest);
    }
  }
  private dispatch(topic: Topic, value: unknown) {
    for (const handler of this.handlers[topic]) handler(value);
  }
  private async resync() {
    if (this.resyncing) return;
    this.resyncing = true;
    try {
      for (const id of Array.from(this.jobs)) {
        try {
          const job = await this.request<Job>(
            `/api/jobs/${encodeURIComponent(id)}`,
          );
          this.track(job);
          this.dispatch("job", { schema_version: 1, seq: this.lastSeq, job });
        } catch (error) {
          if (String(error).includes("not_found:")) this.jobs.delete(id);
        }
      }
    } finally {
      this.resyncing = false;
    }
  }
}

export function selectTransport(): Transport {
  return typeof window.go === "object" && window.go !== null
    ? new WailsTransport()
    : new HttpTransport();
}
