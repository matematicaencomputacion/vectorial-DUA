/**
 * Verificación Playwright local (evidencia PR 6.2).
 * Uso:
 *   npx --yes playwright install chromium
 *   node cmd/master-web/verify/playwright-check.mjs
 *
 * Usa Chrome del sistema (channel: chrome). Requiere master-web en AVLP_WEB_URL
 * (default http://127.0.0.1:8080). Para router caído: no levantar el router.
 */
import { chromium } from "playwright";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.join(__dirname, "out");
const baseURL = process.env.AVLP_WEB_URL || "http://127.0.0.1:8080";

fs.mkdirSync(outDir, { recursive: true });

function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

const browser = await chromium.launch({ headless: true, channel: "chrome" });
const page = await browser.newPage();

// 1) Página fresca: ask-box NO visible
await page.goto(baseURL, { waitUntil: "networkidle" });
const askBox = page.locator("#ask-box");
await assert(await askBox.count() === 1, "ask-box debe existir en el DOM");
await assert(!(await askBox.isVisible()), "ask-box no debe ser visible en estado inicial");
const askVisible = await askBox.evaluate((el) => {
  const s = getComputedStyle(el);
  return s.display !== "none" && s.visibility !== "hidden" && el.getClientRects().length > 0;
});
assert(!askVisible, "ask-box computado sigue visible (¿[hidden] pisado?)");
await page.screenshot({ path: path.join(outDir, "01-fresh-ask-box-hidden.png"), fullPage: true });
console.log("OK: ask-box oculto en página fresca →", path.join(outDir, "01-fresh-ask-box-hidden.png"));

// 2) Router caído: mensaje amable, sin dial tcp
await page.fill("#doubt", "prueba de conexión");
await page.click("#btn-query");
await page.waitForTimeout(1500);
const statusText = (await page.locator("#status").innerText()).trim();
assert(statusText.length > 0, "debe haber mensaje de estado");
assert(!/dial tcp|transport:|connection error|50051/i.test(statusText), `mensaje técnico filtrado: ${statusText}`);
assert(/tutor|conectar|instante|momento/i.test(statusText), `esperaba tono contenedor, got: ${statusText}`);
await page.screenshot({ path: path.join(outDir, "02-router-down-friendly.png"), fullPage: true });
console.log("OK: error amable sin dial tcp →", path.join(outDir, "02-router-down-friendly.png"));
console.log("status:", statusText);

await browser.close();
console.log("verify OK");
