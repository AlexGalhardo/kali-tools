import * as generated from "../../../wailsjs/go/wailsui/Bindings";
import type { Backend, Topic, TransportKind } from "./types";

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
export function selectTransport(): Transport {
  return new WailsTransport();
}
