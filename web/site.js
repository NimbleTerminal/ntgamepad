// ntgamepad - Copyright (c) 2026 Neomantra Corp.
import { BoobaTerminal, parseRendererFromURL } from './booba/booba.js';

const status = document.querySelector('#status');
function failed(error) {
  status.dataset.error = '';
  status.textContent = `Could not run the demo: ${error.message}. Try Restart.`;
  console.error(error);
}
async function main() {
  const go = new Go();
  const response = await fetch('demos/pad.wasm');
  if (!response.ok) throw new Error(`Download failed (${response.status})`);
  const { instance } = await WebAssembly.instantiateStreaming(response, go.importObject);
  const running = go.run(instance);
  running.then(() => { status.textContent = 'Session ended. Choose Restart to run again.'; }, failed);
  const terminal = new BoobaTerminal('terminal', { renderer: parseRendererFromURL() });
  await terminal.init();
  window.addEventListener('pagehide', () => terminal.dispose(), { once: true });
  terminal.connectWasm();
  terminal.focus();
  status.textContent = 'Running locally in your browser. No terminal server required.';
}
main().catch(failed);
