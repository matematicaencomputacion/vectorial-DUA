import { studentId } from "./session.js";

const VE_LABELS = ["Dominio", "Sensorial", "Frustración", "Ritmo", "Autonomía"];
let stopVoice = function () {};

export const state = {
  ve: [0.5, 0.5, 0.5, 0.5, 0.5],
  lastSimilarity: null,
  currentNode: null,
  currentProgress: null,
  rawProgress: null,
  hierarchyUI: null,
  interactiveSession: false,
  activeTabId: null,
};

export const el = {
  form: document.getElementById("query-form"),
  doubt: document.getElementById("doubt"),
  doubtClear: document.getElementById("doubt-clear"),
  frustration: document.getElementById("frustration"),
  stageTitle: document.getElementById("stage-title"),
  stageMeta: document.getElementById("stage-meta"),
  stageMedia: document.getElementById("stage-media"),
  railTopic: document.getElementById("rail-topic"),
  railBody: document.getElementById("rail-body"),
  askBox: document.getElementById("ask-box"),
  askHint: document.getElementById("ask-hint"),
  askDoubt: document.getElementById("ask-doubt"),
  askDoubtClear: document.getElementById("ask-doubt-clear"),
  btnMutate: document.getElementById("btn-mutate"),
  status: document.getElementById("status"),
  dev: document.getElementById("dev-panel"),
};

export function configureVoiceStop(callback) {
  stopVoice = callback;
}

export function setAskMode(interactive) {
  state.interactiveSession = !!interactive;
  el.askBox.hidden = !state.interactiveSession;
  el.askHint.hidden = state.interactiveSession;
  if (!state.interactiveSession && el.askDoubt) {
    el.askDoubt.value = "";
    el.askDoubt.dispatchEvent(new Event("input", { bubbles: true }));
  }
  if (!state.interactiveSession) stopVoice();
}

function clamp01(x) {
  return Math.max(0, Math.min(1, x));
}

export function applyDelta(delta) {
  if (!Array.isArray(delta) || !delta.length) return;
  for (let i = 0; i < state.ve.length; i++) {
    state.ve[i] = clamp01(state.ve[i] + (Number(delta[i]) || 0));
  }
  renderDev();
}

export function renderDev() {
  const lines = [
    "student_id (sesión del navegador): " + studentId,
    "V_e estimado (sesión; se actualiza con preference_delta de Record*):",
    state.ve.map((v, i) => "  [" + i + "] " + VE_LABELS[i] + " = " + v.toFixed(3)).join("\n"),
    "última similitud de ruteo: " + (state.lastSimilarity == null ? "—" : Number(state.lastSimilarity).toFixed(4)),
    "nodo actual: " + (state.currentNode && state.currentNode.node_id ? state.currentNode.node_id : "—"),
    "progreso crudo de subtemas (API):",
    state.rawProgress ? JSON.stringify(state.rawProgress, null, 2) : "—",
    "progreso local (incluye actualización optimista):",
    state.currentProgress ? JSON.stringify(state.currentProgress, null, 2) : "—",
  ];
  el.dev.textContent = lines.join("\n");
}
