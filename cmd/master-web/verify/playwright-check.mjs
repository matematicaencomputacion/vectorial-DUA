/**
 * Verificación Playwright local (evidencia Ola 3.b / C1).
 * Uso:
 *   npx --yes playwright install chromium
 *   node cmd/master-web/verify/playwright-check.mjs
 *
 * Usa Chrome del sistema (channel: chrome). Requiere master-web en AVLP_WEB_URL
 * (default http://127.0.0.1:8080) con el router arriba.
 *
 * Modos (AVLP_ONLY): "chips" solo el mapeo de chips, "progress" solo el
 * acordeón que recuerda, "routerdown" solo el chequeo de router caído
 * (levantar master-web sin router). Sin valor corre todo.
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
let page = await browser.newPage();

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
    } else {
      dest = `pending ${route.pending.tracking_ulid}`;
      // Miss async (p. ej. LLM > AVLP_LLM_SYNC_DEADLINE): la UI ya hace poll.
      await page.waitForFunction(
        () => {
          const title = document.getElementById("stage-title")?.textContent || "";
          const status = document.getElementById("status")?.textContent || "";
          return /Estación lista/i.test(title) || /La estación está lista/i.test(status);
        },
        { timeout: 120000 }
      );
    }

    if (/honesto/i.test(chip.label)) {
      if (route.matched) {
        const content = String(route.matched.live_content || "");
        assert(
          /no encontré material verificado|sin inventar/i.test(content),
          `estación honesta debía decir que no sabe, got: ${content.slice(0, 200)}`
        );
      } else {
        const stageText = await page.locator("#stage").innerText();
        assert(
          /no encontré material verificado|sin inventar/i.test(stageText),
          `Stage honesto (vía poll) debía decir que no sabe, got: ${stageText.slice(0, 200)}`
        );
      }
      await page.waitForTimeout(400);
      await shot(page, "flow-05-live-honesto.png");
      console.log("OK estación en vivo honesta → flow-05-live-honesto.png");

      // Rematch: misma duda → nodo live:// + live_content rehidratado del ledger.
      const waitRematch = page.waitForResponse(
        (r) => r.url().includes("/api/query") && r.request().method() === "POST",
        { timeout: 45000 }
      );
      await page.click("#btn-query");
      const rematch = await (await waitRematch).json();
      assert(rematch.matched, `rematch honesto sin match: ${JSON.stringify(rematch)}`);
      assert(
        String(rematch.matched.resource_url).startsWith("live://stations/"),
        `rematch debía ir a live://, fue ${rematch.matched.resource_url}`
      );
      const rematchContent = String(rematch.matched.live_content || "");
      assert(
        rematchContent.length > 0,
        "rematch de estación live debe traer live_content del ledger"
      );
      assert(
        /no encontré material verificado|sin inventar/i.test(rematchContent),
        `rematch debía conservar el contenido honesto, got: ${rematchContent.slice(0, 200)}`
      );
      await page.waitForTimeout(400);
      const stageText = await page.locator("#stage").innerText();
      assert(
        !/live:\/\/stations\//i.test(stageText),
        `Stage no debe mostrar la URL cruda tras rematch, got: ${stageText.slice(0, 200)}`
      );
      await shot(page, "flow-07-live-rematch.png");
      console.log("OK rematch live con contenido → flow-07-live-rematch.png");
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

async function subtopicProgressCheck() {
  let progressGets = 0;
  const automovilID = "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FC0";
  // El test apunta al acordeón, no a calibrar embeddings: fija solo la decisión
  // de routing y deja nodo, progreso y Record* contra el stack real.
  await page.route("**/api/query", async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        matched: {
          node_id: automovilID,
          dimension_dua: "Representacion",
          resource_url: "interactive://" + automovilID,
          similarity_score: 0.9,
          is_live_generated: false,
          retrieved_sources: [],
          live_content: "",
          has_interactive_payload: true,
        },
      }),
    });
  });
  page.on("request", (req) => {
    if (req.method() === "GET" && /\/api\/nodes\/.+\/progress\?/.test(req.url())) progressGets++;
  });

  async function loadAutomovil() {
    await page.goto(baseURL, { waitUntil: "domcontentloaded" });
    await page.locator(".hints button", { hasText: "Ejemplo: automóvil" }).click();
    const waitProgress = page.waitForResponse(
      (r) => /\/api\/nodes\/.+\/progress\?/.test(r.url()) && r.request().method() === "GET",
      { timeout: 45000 }
    );
    await page.click("#btn-query");
    const progressResponse = await waitProgress;
    assert(progressResponse.ok(), `GET progress falló: ${progressResponse.status()}`);
    await page.waitForSelector(".progress-summary", { timeout: 15000 });
  }

  await loadAutomovil();
  const summary = page.locator(".progress-summary");
  assert(/Exploraste 0 de 5 subtemas/.test(await summary.innerText()), `contador inicial: ${await summary.innerText()}`);
  assert(
    (await summary.getAttribute("aria-label")) === "Exploraste 0 de 5 subtemas",
    `aria-label inicial: ${await summary.getAttribute("aria-label")}`
  );
  const cleanStates = await page.locator(".subtopic-state").allInnerTexts();
  assert(cleanStates.length === 5, `esperaba 5 estados, got: ${cleanStates.length}`);
  assert(cleanStates.every((s) => /○ Por explorar/.test(s)), `estados iniciales: ${JSON.stringify(cleanStates)}`);
  await shot(page, "progress-01-clean.png");
  console.log("OK acordeón limpio: 0 de 5, símbolos + texto → progress-01-clean.png");

  const caja = page.locator('.accordion-trigger[data-subtopic-id="sub_caja_central"]');
  const motor = page.locator('.accordion-trigger[data-subtopic-id="sub_motor"]');
  await caja.click();
  await motor.click();
  const waitRecord = page.waitForResponse(
    (r) => r.url().includes("/api/interactions/subtopic") && r.request().method() === "POST",
    { timeout: 20000 }
  );
  await page.locator("#acc-sub_motor .subtopic-select").first().click();
  const record = await waitRecord;
  assert(record.ok(), `RecordSubtopicInteraction falló: ${record.status()}`);

  assert(/Exploraste 1 de 5 subtemas/.test(await summary.innerText()), `contador optimista: ${await summary.innerText()}`);
  assert(/◐ Exploración iniciada/.test(await caja.innerText()), `Caja Central: ${await caja.innerText()}`);
  assert(/✓ Visitado/.test(await motor.innerText()), `Motor: ${await motor.innerText()}`);
  assert(progressGets === 1, `la apertura no debe re-fetch completo; GET progress=${progressGets}`);
  assert(!/%|100|complet/i.test(await summary.innerText()), `copy gamificado: ${await summary.innerText()}`);
  const dev = await page.locator("#dev-panel").textContent();
  assert(/progreso crudo de subtemas/.test(dev) && /progreso local/.test(dev) && /sub_motor/.test(dev), "panel dev sin progreso crudo/local");
  await shot(page, "progress-02-motor-visited.png");
  console.log("OK Motor visitado + Caja parcial + 1 de 5, sin re-fetch → progress-02-motor-visited.png");

  // Misma pestaña: sessionStorage conserva student_id; una nueva carga
  // reconcilia el optimismo contra InteractionStore en el router.
  await loadAutomovil();
  assert(/Exploraste 1 de 5 subtemas/.test(await summary.innerText()), `contador reconciliado: ${await summary.innerText()}`);
  const reconciledMotor = page.locator('.accordion-trigger[data-subtopic-id="sub_motor"]');
  assert(/✓ Visitado/.test(await reconciledMotor.innerText()), `Motor reconciliado: ${await reconciledMotor.innerText()}`);
  assert(progressGets === 2, `esperaba un GET por carga, got: ${progressGets}`);
  await shot(page, "progress-03-reconciled.png");
  console.log("OK recarga con mismo student_id conserva Motor → progress-03-reconciled.png");

  // Condición de carrera: búsqueda A lenta + B inmediata → solo B pinta.
  await page.unroute("**/api/query");
  const nodeA = "dua::Representacion::basico::visual::01ARZ3NDEKTSV4RRFFQ69G5FAV"; // variables-scope
  const nodeB = automovilID;
  let queryN = 0;
  await page.route("**/api/query", async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    queryN += 1;
    const nodeID = queryN === 1 ? nodeA : nodeB;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        matched: {
          node_id: nodeID,
          dimension_dua: "Representacion",
          resource_url: "interactive://" + nodeID,
          similarity_score: 0.9,
          is_live_generated: false,
          retrieved_sources: [],
          live_content: "",
          has_interactive_payload: true,
        },
      }),
    });
  });
  // Retrasa solo el GET del payload de A (no el de progreso ni el de B).
  await page.route(`**/api/nodes/${encodeURIComponent(nodeA)}`, async (route) => {
    if (route.request().method() !== "GET") return route.continue();
    await new Promise((r) => setTimeout(r, 1500));
    return route.continue();
  });

  await page.goto(baseURL, { waitUntil: "domcontentloaded" });
  await page.locator(".hints button", { hasText: "Ejemplo: variables y scope" }).click();
  await page.click("#btn-query");
  await page.waitForTimeout(80);
  await page.locator(".hints button", { hasText: "Ejemplo: automóvil" }).click();
  await page.click("#btn-query");
  await page.waitForSelector(".progress-summary", { timeout: 20000 });
  // Dejá que A termine de resolver: si no hay invalidación, pisaría el rail.
  await page.waitForTimeout(1800);
  const topic = (await page.locator("#rail-topic").innerText()).trim();
  assert(/Automóvil|automóvil/i.test(topic), `rail debía quedar en B (automóvil), got: ${topic}`);
  assert((await page.locator(".progress-summary").count()) === 1, "B debe mostrar el resumen de progreso");
  assert(
    !(await page.locator("#rail-body").innerText()).match(/Resumen express/i),
    "A (variables/scope) no debía pintar la botonera depth tras B"
  );
  await shot(page, "progress-04-stale-race.png");
  console.log("OK carrera A→B: solo B pinta el rail → progress-04-stale-race.png");
}

if (mode === "routerdown") {
  await routerDownCheck();
  await browser.close();
  console.log("verify OK (router caído)");
  process.exit(0);
}

if (mode === "progress") {
  await subtopicProgressCheck();
  await browser.close();
  console.log("verify OK (progreso de subtemas)");
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

// Auth: modo abierto por defecto — sesión sin Bearer obligatorio; «Soy docente» oculto.
{
  const sess = await page.evaluate(async () => {
    const r = await fetch("/api/session", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ student_id: "stu-verify-open" }),
    });
    return { status: r.status, body: await r.json() };
  });
  assert(sess.status === 200, `POST /api/session falló: ${sess.status}`);
  assert(sess.body.secure_mode === false, "lab por defecto debe ser modo abierto");
  assert(sess.body.stt_enabled === false, "lab por defecto sin AVLP_STT_URL → stt_enabled=false");
  const teacherDetails = page.locator("#teacher-details");
  assert((await teacherDetails.count()) === 1, "debe existir el bloque Soy docente");
  assert(await teacherDetails.isHidden(), "Soy docente oculto en modo abierto");
  assert(await page.locator("#btn-promote").isHidden(), "promover oculto sin rol teacher");
  console.log("OK auth modo abierto (secure_mode=false, UI docente oculta)");
}

// Voz: sin STT y sin SpeechRecognition → no hay micrófono; panel refleja mode=none.
{
  await page.addInitScript(() => {
    try { delete window.SpeechRecognition; } catch (_) {}
    try { delete window.webkitSpeechRecognition; } catch (_) {}
    window.SpeechRecognition = undefined;
    window.webkitSpeechRecognition = undefined;
  });
  await page.goto(baseURL, { waitUntil: "domcontentloaded" });
  await page.waitForFunction(() => {
    const t = document.getElementById("dev-panel")?.textContent || "";
    return /voice:\s*mode=/.test(t);
  }, null, { timeout: 10000 });
  const micCount = await page.locator(".mic-btn").count();
  assert(micCount === 0, `sin STT ni SpeechRecognition no debe haber mic, got ${micCount}`);
  const dev = await page.locator("#dev-panel").evaluate((el) => el.textContent || "");
  assert(/voice:\s*mode=none/.test(dev), `panel debe mostrar voice mode=none: ${dev.slice(0, 200)}`);
  assert(/stt_enabled=false/.test(dev), "panel debe mostrar stt_enabled=false");
  await shot(page, "flow-09-voice-fallback-none.png");
  console.log("OK voz fallback sin mic → flow-09-voice-fallback-none.png");
  // Quitar el init script para el resto del flujo: nueva context page.
  await page.close();
  page = await browser.newPage();
  await page.goto(baseURL, { waitUntil: "domcontentloaded" });
}

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

// 2b) Toggle de transcripción en un nodo interactivo con transcript.
{
  await page.goto(baseURL, { waitUntil: "domcontentloaded" });
  const chipBtn = page.locator(".hints button", { hasText: "Ejemplo: variables y scope" });
  await chipBtn.click();
  const waitQuery = page.waitForResponse(
    (r) => r.url().endsWith("/api/query") && r.request().method() === "POST",
    { timeout: 45000 }
  );
  await page.click("#btn-query");
  await waitQuery;
  await page.waitForSelector("#ask-box:not([hidden])", { timeout: 15000 });
  const toggle = page.locator(".transcript-toggle");
  assert((await toggle.count()) === 1, "debe existir el toggle Ver transcripción");
  assert((await toggle.getAttribute("aria-expanded")) === "false", "toggle inicia cerrado");
  assert(/Ver transcripción/i.test(await toggle.innerText()), "toggle debe tener texto, no solo ícono");
  await toggle.click();
  assert((await toggle.getAttribute("aria-expanded")) === "true", "toggle abierto");
  const panel = page.locator("#stage-transcript-panel");
  assert(await panel.isVisible(), "panel de transcripción visible");
  const panelText = await panel.innerText();
  assert(/scope|variable/i.test(panelText), `transcript vacío o incorrecto: ${panelText.slice(0, 120)}`);
  await shot(page, "flow-08-transcript-toggle.png");
  console.log("OK toggle Ver transcripción → flow-08-transcript-toggle.png");
}

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

// 5) Rematch con live_content se verificó al re-preguntar el chip «fuera de tema».

// 6) Higiene de repo: el estado local del profile store no se versiona.
const gitignore = fs.readFileSync(path.join(__dirname, "../../../.gitignore"), "utf8");
assert(/(^|\n)data\/profiles\.json(\r?\n|$)/.test(gitignore), ".gitignore debe listar data/profiles.json");
console.log("OK data/profiles.json ignorado");

await browser.close();
console.log("verify OK");
