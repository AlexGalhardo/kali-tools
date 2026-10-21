import type { en } from "../shared/i18n/i18n";
export type Catalog = typeof en;
export type PageProps = {
  t: Catalog;
  busy: boolean;
  execute: (fn: () => Promise<void>) => Promise<void>;
  stateText: (state: string) => string;
  setError: (value: string) => void;
  setNotice: (value: string) => void;
};
