import React from "react";
import { createRoot } from "react-dom/client";
import { App } from "./app/App";
import { initBridge } from "./shared/api/bridge";
import "./styles/style.css";

async function main() {
  await initBridge();
  const root = document.getElementById("root");
  if (root)
    createRoot(root).render(
      <React.StrictMode>
        <App />
      </React.StrictMode>,
    );
}
void main();
