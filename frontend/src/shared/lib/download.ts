export function download(value: unknown) {
  const blob = new Blob(
    [JSON.stringify({ schema_version: 1, data: value }, null, 2)],
    { type: "application/json" },
  );
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "hacker-tools-export.json";
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
