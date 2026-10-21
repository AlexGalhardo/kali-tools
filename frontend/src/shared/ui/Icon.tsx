const paths: Record<string, string> = {
  refresh:
    "M20 7v5h-5 M4 17v-5h5 M5 8a8 8 0 0 1 13-3l2 3 M19 16a8 8 0 0 1-13 3l-2-3",
  open: "M6 18 18 6 M6 6h12v12",
  forward: "M4 12h16 M14 6l6 6-6 6",
  dashboard: "M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z",
  interfaces: "M3 7h17m-4-4 4 4-4 4 M21 17H4m4-4-4 4 4 4",
  diagnostics: "M2 12h5l3-8 4 16 3-8h5",
  wifi: "M2 8a16 16 0 0 1 20 0 M5 12a11 11 0 0 1 14 0 M8 16a6 6 0 0 1 8 0 M12 20h.01",
  packets: "M5 3h14v18H5z M8 7h8 M8 11h8 M8 15h5",
  toolkit: "M3 8V3h5 M16 3h5v5 M21 16v5h-5 M8 21H3v-5 M8 12h8 M12 8v8",
  history: "M3 12a9 9 0 1 0 3-7 M3 3v5h5 M12 7v5l3 2",
  settings:
    "M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M12 2v3 M12 19v3 M2 12h3 M19 12h3 M5 5l2 2 M17 17l2 2 M19 5l-2 2 M7 17l-2 2",
};
export function NavIcon({ page }: { page: string }) {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      width="20"
      height="20"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d={paths[page] ?? paths.dashboard} />
    </svg>
  );
}
