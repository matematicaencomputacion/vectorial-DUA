/**
 * Verificación Playwright local (evidencia Ola 3.b / C1).
 * Uso:
 *   npx --yes playwright install chromium
 *   node cmd/master-web/verify/playwright-check.mjs
 *
 * Usa Chrome del sistema (channel: chrome). Requiere master-web en AVLP_WEB_URL
 * (default http://127.0.0.1:8080) con el router arriba.
 *
 * Modos (AVLP_ONLY): "chips" solo el mapeo de chips, "routerdown" solo el
 * chequeo de router caído (levantar master-web sin router). Sin valor corre todo.
 */
import { chromium } from "playwright";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.join(__dirname, "out");
const baseURL = process.env.AVLP_WEB_URL || "http://127.0.0.1:8080";
const mode = process.env.AVLP_ONLY || "full";

fs.mkdirSync(outDir, { recursive: true });

function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

function shot(page, name) {
  return page.screenshot({ path: path.join(outDir, name), fullPage: true });
}

// Cada chip declara su destino esperado: nodo interactivo propio, seed estático
// o estación en vivo. Los ULID son los seeds de data/nodes/interactive.
const CHIPS = [
  { label: "Ejemplo: .env / diagrama", kind: "static", url: "master://nodes/env-diagram" },
  { label: "Ejemplo: variables y scope", kind: "interactive", ulid: "01ARZ3NDEKTSV4RRFFQ69G5FAV" },
  { label: "Ejemplo: async/await", kind: "interactive", ulid: "01ARZ3NDEKTSV4RRFFQ69G5FB0" },
  { label: "Ejemplo: PostGIS matriz", kind: "interactive", ulid: "01ARZ3NDEKTSV4RRFFQ69G5FB2" },
  { label: "Ejemplo: automóvil", kind: "interactive", ulid: "01ARZ3NDEKTSV4RRFFQ69G5FC0" },
  { label: "Ejemplo: duda novel (live)", kind: "live" },
  { label: "Ejemplo: fuera de tema (honesto)", kind: "live" },
];

const browser = await chromium.launch({ headless: true, channel: "chrome" });
const page = await browser.newPage();

async function runChip(chip, i) {
  await page.goto(baseURL, { waitUntil: "domcontentloaded" });
  const chipBtn = page.locator(".hints button", { hasText: chip.label });
  assert((await chipBtn.count()) === 1, `chip no encontrado: ${chip.label}`);
  await chipBtn.click();

  const queryText = await page.inputValue("#doubt");
  assert(queryText.length > 0, `el chip no rellenó «Tu duda»: ${chip.label}`);

  const waitQuery = page.waitForResponse(
    (r) => r.url().endsWith("/api/query") && r.request().method() === "POST",
    { timeout: 45000 }
  );
  await page.click("#btn-query");
  const route = await (await waitQuery).json();

  const shotName = `chip-${String(i + 1).padStart(2, "0")}-${chip.kind}.png`;
  let dest;
  if (chip.kind === "live") {
    assert(route.matched || route.pending, `sin resultado para ${chip.label}`);
    if (route.matched) {
      assert(
        String(route.matched.resource_url).startsWith("live://stations/"),
        `${chip.label} debía ir a estación en vivo, fue a ${route.matched.resource_url}`
      );
      assert(!route.matched.has_interactive_payload, "una estación live no trae payload interactivo");
      dest = route.matched.resource_url;
      // El chip «fuera de tema» se materializa por primera vez aquí: el contenido
      // honesto vive en la respuesta del miss, no en un re-match posterior.
      if (/honesto/i.test(chip.label)) {
        const content = String(route.matched.live_content || "");
        assert(
          /no encontré material verificado|sin inventar/i.test(content),
          `estación honesta debía decir que no sabe, got: ${content.slice(0, 200)}`
        );
        await page.waitForTimeout(400);
        await shot(page, "flow-05-live-honesto.png");
        console.log("OK estación en vivo honesta → flow-05-live-honesto.png");
      }
    } else {
      dest = `pending ${route.pending.tracking_ulid}`;
    }
    await page.waitForTimeout(600);
  } else {
    assert(route.matched, `${chip.label} no matcheó: ${JSON.stringify(route)}`);
    const m = route.matched;
    dest = m.node_id;
    if (chip.kind === "interactive") {
      assert(
        m.node_id.endsWith(chip.ulid),
        `${chip.label} debía ir a ${chip.ulid}, fue a ${m.node_id} (${m.resource_url})`
      );
      assert(m.has_interactive_payload, `${chip.label} debía traer payload interactivo`);
      // El nodo interactivo habilita «+ Tengo una duda diferente».
      await page.waitForSelector("#ask-box:not([hidden])", { timeout: 15000 });
    } else {
      assert(m.resource_url === chip.url, `${chip.label} debía ir a ${chip.url}, fue a ${m.resource_url}`);
      assert(!m.has_interactive_payload, `${chip.label} no debería traer payload interactivo`);
    }
    assert(m.similarity_score >= 0.55, `${chip.label} bajo umbral: ${m.similarity_score}`);
  }

  await shot(page, shotName);
  const sim = route.matched ? Number(route.matched.similarity_score).toFixed(3) : "—";
  console.log(`OK chip «${chip.label}» → ${chip.kind} ${dest} (sim ${sim}) → ${shotName}`);
}

async function routerDownCheck() {
  await page.goto(baseURL, { waitUntil: "domcontentloaded" });
  await page.fill("#doubt", "prueba de conexión");
  await page.click("#btn-query");
  // El estado arranca en «Consultando el router…»; esperamos el tono de error.
  await page.waitForSelector('#status[data-tone="error"]', { timeout: 20000 });
  const statusText = (await page.locator("#status").innerText()).trim();
  assert(
    !/dial tcp|transport:|connection error|5005\d|rpc error/i.test(statusText),
    `mensaje técnico filtrado al estudiante: ${statusText}`
  );
  assert(/tutor|conectar|instante|momento/i.test(statusText), `esperaba tono contenedor, got: ${statusText}`);
  await shot(page, "flow-06-router-caido.png");
  console.log(`OK router caído → mensaje amable: «${statusText}» → flow-06-router-caido.png`);
}

if (mode === "routerdown") {
  await routerDownCheck();
  await browser.close();
  console.log("verify OK (router caído)");
  process.exit(0);
}

if (mode === "chips") {
  for (let i = 0; i < CHIPS.length; i++) {
    await runChip(CHIPS[i], i);
  }
  await browser.close();
  console.log("verify OK (chips)");
  process.exit(0);
}

// 1) Estado inicial: sin formulario de duda diferente y HTML sin cachear.
const resp = await page.goto(baseURL, { waitUntil: "domcontentloaded" });
const cacheControl = (resp.headers()["cache-control"] || "").toLowerCase();
assert(cacheControl.includes("no-cache"), `index.html debe ir con no-cache, got: ${cacheControl || "(vacío)"}`);
console.log(`OK Cache-Control: ${cacheControl}`);

const askBox = page.locator("#ask-box");
assert((await askBox.count()) === 1, "ask-box debe existir en el DOM");
assert(!(await askBox.isVisible()), "ask-box no debe ser visible sin nodo interactivo");
const askVisible = await askBox.evaluate((el) => {
  const s = getComputedStyle(el);
  return s.display !== "none" && s.visibility !== "hidden" && el.getClientRects().length > 0;
});
assert(!askVisible, "ask-box computado sigue visible (¿[hidden] pisado?)");
await shot(page, "flow-01-inicial.png");
console.log("OK estado inicial sin formulario de duda diferente → flow-01-inicial.png");

// 2) Botón de limpieza en ambos campos: oculto vacío, borra y devuelve el foco.
for (const [field, clear] of [["#doubt", "#doubt-clear"], ["#ask-doubt", "#ask-doubt-clear"]]) {
  if (field === "#ask-doubt") {
    await page.evaluate(() => { document.getElementById("ask-box").hidden = false; });
  }
  const btn = page.locator(clear);
  assert((await btn.count()) === 1, `${clear} debe existir`);
  assert((await btn.getAttribute("aria-label")) === "Borrar el texto", `${clear}: aria-label`);
  assert(!(await btn.isVisible()), `${clear} oculto con el campo vacío`);
  await page.fill(field, "texto dictado que salió torcido");
  assert(await btn.isVisible(), `${clear} visible cuando hay texto`);
  await btn.click();
  assert((await page.inputValue(field)) === "", `${clear} debe vaciar ${field}`);
  assert(!(await btn.isVisible()), `${clear} vuelve a ocultarse`);
  const focused = await page.evaluate(() => document.activeElement && document.activeElement.id);
  assert(focused === field.slice(1), `el foco debe volver a ${field}, está en ${focused}`);
}
await shot(page, "flow-02-boton-limpieza.png");
console.log("OK botón de limpieza en ambos campos → flow-02-boton-limpieza.png");

// 3) Los 7 chips van al nodo que anuncia su etiqueta.
for (let i = 0; i < CHIPS.length; i++) {
  await runChip(CHIPS[i], i);
}

// 4) Nodo interactivo: botonera + mutate en vivo sin recargar.
await page.goto(baseURL, { waitUntil: "domcontentloaded" });
await page.locator(".hints button", { hasText: "Ejemplo: variables y scope" }).click();
const waitInteractive = page.waitForResponse(
  (r) => r.url().includes("/api/nodes/") && r.request().method() === "GET",
  { timeout: 45000 }
);
await page.click("#btn-query");
await waitInteractive;
await page.waitForSelector("#ask-box:not([hidden])", { timeout: 15000 });

const railButtons = page.locator("#rail-body button");
const railCount = await railButtons.count();
assert(railCount > 0, "la botonera debe renderizar opciones");
const railLabels = await railButtons.allInnerTexts();
assert(
  railLabels.some((t) => /Resumen express/i.test(t)),
  `la botonera depth debe usar copy neutral, got: ${JSON.stringify(railLabels)}`
);
assert(
  !railLabels.some((t) => /ELITE|TL;DR/i.test(t)),
  `copy de jerga en la botonera: ${JSON.stringify(railLabels)}`
);
await shot(page, "flow-03-botonera-depth.png");
console.log(`OK botonera depth (${railCount} opciones, copy neutral) → flow-03-botonera-depth.png`);

// Tocar una opción registra la interacción sin romper el Stage.
const waitRecord = page.waitForResponse(
  (r) => r.url().includes("/api/interactions/botonera") && r.request().method() === "POST",
  { timeout: 20000 }
);
await railButtons.first().click();
const recordResp = await waitRecord;
assert(recordResp.ok(), `RecordBotonera falló: ${recordResp.status()}`);
console.log("OK toque de botonera registrado");

await page.fill("#ask-doubt", "y cómo veo el scope cuando debuggeo");
const waitMutate = page.waitForResponse(
  (r) => r.url().includes("/mutate") && r.request().method() === "POST",
  { timeout: 45000 }
);
await page.click("#btn-mutate");
const mutateResp = await waitMutate;
assert(mutateResp.ok(), `mutate falló: ${mutateResp.status()}`);
const mutateBody = await mutateResp.json();
const liveLabel = String(mutateBody.button.label || "");
assert(/^LIVE: /.test(liveLabel), `el botón live debe anunciarse como LIVE, got: ${liveLabel}`);
assert(
  !/\[[^\]]*\.(md|txt|json)\]/i.test(liveLabel),
  `el label expone un archivo interno: ${liveLabel}`
);
await page.waitForFunction(
  () => Array.from(document.querySelectorAll("#rail-body button")).some((b) => /^LIVE/.test(b.textContent.trim())),
  null,
  { timeout: 15000 }
);
await shot(page, "flow-04-mutate-live.png");
console.log(`OK botón live en la botonera: «${liveLabel}» → flow-04-mutate-live.png`);

// 5) La estación honesta se verificó al ejercitar el chip «fuera de tema»
// (contenido del miss path). Re-matchear un nodo live no reexpone live_content:
// eso queda anotado como deuda Ola 4.

// 6) Higiene de repo: el estado local del profile store no se versiona.
const gitignore = fs.readFileSync(path.join(__dirname, "../../../.gitignore"), "utf8");
assert(/(^|\n)data\/profiles\.json(\r?\n|$)/.test(gitignore), ".gitignore debe listar data/profiles.json");
console.log("OK data/profiles.json ignorado");

await browser.close();
console.log("verify OK");
