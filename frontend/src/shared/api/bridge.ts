import { selectTransport, type Transport } from "./transport";
import type { Backend, Topic, TransportKind } from "./types";

export * from "./types";

let transport: Transport | undefined;
let ready = false;

export async function initBridge(): Promise<void> {
  try {
    transport = selectTransport();
    ready = await transport.init();
  } catch {
    ready = false;
  }
}
export const connected = () => ready;
export const transportKind = (): TransportKind => transport?.kind ?? "wails";
export function backend(): Backend {
  if (!ready || !transport)
    throw new Error("Native bridge unavailable. Open the desktop application.");
  return transport.backend;
}
export function onEvent<T>(name: Topic, fn: (data: T) => void): () => void {
  return transport?.on(name, (value) => fn(value as T)) ?? (() => {});
}
