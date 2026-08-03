/**
 * Stack hermético para Playwright: copia solo seeds interactive trackeados
 * en git (`git ls-files`) a un dir temporal y, por defecto, levanta router +
 * master-web apuntando AVLP_INTERACTIVE_NODES_DIR allí.
 *
 * Así un promoted-*.json u otro artefacto local del operador no altera el
 * índice ni la similitud de los chips canónicos.
 */
import { execFileSync, spawn } from "node:child_process";
import fs from "node:fs";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
export const repoRoot = path.resolve(__dirname, "../../..");

const VERIFY_SECRET = "verify-hermetic-session-secret-ola7";

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

export function freePort() {
  return new Promise((resolve, reject) => {
    const s = net.createServer();
    s.once("error", reject);
    s.listen(0, "127.0.0.1", () => {
      const addr = s.address();
      const port = typeof addr === "object" && addr ? addr.port : 0;
      s.close((err) => (err ? reject(err) : resolve(port)));
    });
  });
}

/**
 * Seeds canónicos = archivos bajo data/nodes/interactive/ que git trackea.
 * Untracked / promoted locales quedan fuera por construcción.
 */
export function prepareTrackedInteractiveFixtures() {
  const out = execFileSync(
    "git",
    ["ls-files", "-z", "--", "data/nodes/interactive/*.json"],
    { cwd: repoRoot }
  );
  const rels = out
    .toString("utf8")
    .split("\0")
    .map((s) => s.trim())
    .filter(Boolean);
  if (!rels.length) {
    throw new Error(
      "hermetic-stack: git ls-files no listó seeds en data/nodes/interactive/*.json"
    );
  }
  const promoted = rels.filter((r) => /^promoted-/i.test(path.basename(r)));
  if (promoted.length) {
    throw new Error(
      "hermetic-stack: seeds promoted-* trackeados en git (deben vivir en data/nodes/promoted-local/, gitignore):\n  " +
        promoted.join("\n  ")
    );
  }
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "avlp-interactive-fixtures-"));
  for (const rel of rels) {
    const src = path.join(repoRoot, rel);
    if (!fs.existsSync(src)) {
      throw new Error(`hermetic-stack: seed trackeado ausente en working tree: ${rel}`);
    }
    fs.copyFileSync(src, path.join(dir, path.basename(rel)));
  }
  return { dir, files: rels.map((r) => path.basename(r)) };
}

function killTree(child) {
  if (!child || child.exitCode != null || child.killed) return;
  try {
    if (process.platform === "win32") {
      spawn("taskkill", ["/pid", String(child.pid), "/T", "/F"], { stdio: "ignore" });
    } else {
      process.kill(-child.pid, "SIGTERM");
    }
  } catch (_) {
    try {
      child.kill("SIGTERM");
    } catch (__) {
      /* already gone */
    }
  }
}

function spawnGo(pkg, env) {
  const child = spawn("go", ["run", pkg], {
    cwd: repoRoot,
    env: { ...process.env, ...env },
    stdio: ["ignore", "pipe", "pipe"],
    detached: process.platform !== "win32",
  });
  child._avlpLog = "";
  const append = (buf) => {
    child._avlpLog += buf.toString("utf8");
    if (child._avlpLog.length > 32000) {
      child._avlpLog = child._avlpLog.slice(-16000);
    }
  };
  child.stdout.on("data", append);
  child.stderr.on("data", append);
  return child;
}

async function waitForListen(child, needle, label, timeoutMs = 60000) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (child.exitCode != null) {
      throw new Error(`${label} salió antes de escuchar (code=${child.exitCode}):\n${child._avlpLog}`);
    }
    if (child._avlpLog.includes(needle)) return;
    await sleep(200);
  }
  killTree(child);
  throw new Error(`${label} no escuchó a tiempo:\n${child._avlpLog}`);
}

/**
 * @param {{ withRouter?: boolean }} opts
 * @returns {Promise<{ baseURL: string, fixturesDir: string, fixtureFiles: string[], cleanup: () => Promise<void> }>}
 */
export async function startHermeticStack(opts = {}) {
  const withRouter = opts.withRouter !== false;
  const prepared = prepareTrackedInteractiveFixtures();
  const children = [];
  let cleaned = false;

  const cleanup = async () => {
    if (cleaned) return;
    cleaned = true;
    for (const c of children.reverse()) killTree(c);
    await sleep(300);
    try {
      fs.rmSync(prepared.dir, { recursive: true, force: true });
    } catch (_) {
      /* best-effort */
    }
  };

  try {
    const webPort = await freePort();
    const routerPort = withRouter ? await freePort() : 1;
    // Umbral hash calibrado el 2026-08-03 por:
    //   go run ./cmd/harness -suite calibrate -embedder hash
    // → suggested_threshold=0.482, worst_correct=0.389, best_incorrect=0.576,
    //   margen=-0.187 (WARNING overlap en goldens SeedDemo: bajo hash los
    //   expected_outcome_hash son live; el midpoint es el dato de calibrate,
    //   no un piso bajado a mano para chips). Tras doc-expansion Ola 2.b en
    //   interactive/*.json, chips canónicos quedan ≥0.55 contra su nodo.
    const HASH_CALIBRATED_THRESHOLD = "0.482";
    const sharedEnv = {
      // Cero configuración ambiental: pin explícito; no heredar Ollama/umbral
      // del shell del operador ni data/avlp.json local.
      AVLP_SESSION_SECRET: VERIFY_SECRET,
      AVLP_LLM_URL: "",
      AVLP_EMBEDDING_URL: "",
      AVLP_STT_URL: "",
      AVLP_TEACHER_KEY: "",
      AVLP_INTERACTIVE_NODES_DIR: prepared.dir,
      AVLP_SIMILARITY_THRESHOLD: HASH_CALIBRATED_THRESHOLD,
      // Path inexistente: el router no debe caer en data/avlp.json del cwd
      // si alguien quita el pin de THRESHOLD por error.
      AVLP_CONFIG_PATH: path.join(prepared.dir, ".no-avlp-config.json"),
    };

    if (withRouter) {
      const router = spawnGo("./cmd/router", {
        ...sharedEnv,
        AVLP_ROUTER_ADDR: `127.0.0.1:${routerPort}`,
      });
      children.push(router);
      await waitForListen(router, "listening on", "router");
    }

    const web = spawnGo("./cmd/master-web", {
      ...sharedEnv,
      AVLP_WEB_ADDR: `127.0.0.1:${webPort}`,
      AVLP_ROUTER_ADDR: `127.0.0.1:${routerPort}`,
    });
    children.push(web);
    await waitForListen(web, "listening on", "master-web");

    console.log(
      `hermetic-stack: fixtures=${prepared.files.length} thr=${HASH_CALIBRATED_THRESHOLD} (calibrate hash 2026-08-03) → http://127.0.0.1:${webPort}` +
        (withRouter ? ` (router :${routerPort})` : " (sin router)")
    );

    return {
      baseURL: `http://127.0.0.1:${webPort}`,
      fixturesDir: prepared.dir,
      fixtureFiles: prepared.files,
      cleanup,
    };
  } catch (e) {
    await cleanup();
    throw e;
  }
}
