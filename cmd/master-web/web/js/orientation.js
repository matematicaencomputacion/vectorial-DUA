import { requestGeneration, studentId } from "./session.js";
import { el, renderDev, state } from "./state.js";
import { api } from "./ui.js";

/** Copy fijado por el plan Ola 7.4 (texto al estudiante). */
export function orientationCopy(topicTitle) {
  const x = String(topicTitle || "").trim() || "un tema previo";
  return (
    "Esto se apoya en " +
    x +
    ". Si querés lo miramos antes, o seguí acá y volvés cuando te sirva"
  );
}

export function clearOrientation() {
  state.rawOrientation = null;
  const root = document.getElementById("orientation-block");
  if (!root) return;
  root.hidden = true;
  root.replaceChildren();
}

/**
 * Consulta orientación después del render. Respeta el guard de generación:
 * respuestas stale no pintan el rail.
 */
export async function loadOrientation(nodeId, token) {
  if (token == null) token = requestGeneration.token();
  if (!requestGeneration.isCurrent(token)) return;

  clearOrientation();
  renderDev();
  if (!nodeId) return;

  try {
    const data = await api(
      "/api/nodes/" +
        encodeURIComponent(nodeId) +
        "/orientation?student_id=" +
        encodeURIComponent(studentId)
    );
    if (!requestGeneration.isCurrent(token)) return;
    state.rawOrientation = data;
    renderDev();
    if (!data || data.available === false) return;
    const gaps = Array.isArray(data.gaps) ? data.gaps : [];
    if (!gaps.length) return;
    renderOrientationBlock(gaps[0]);
  } catch (_) {
    if (!requestGeneration.isCurrent(token)) return;
    state.rawOrientation = null;
    renderDev();
  }
}

function renderOrientationBlock(gap) {
  const root = document.getElementById("orientation-block");
  if (!root) return;
  const title = String((gap && gap.title) || "").trim() || "un tema previo";

  const heading = document.createElement("h4");
  heading.className = "orientation-title";
  heading.textContent = "Para ubicarte";

  const copy = document.createElement("p");
  copy.className = "orientation-copy";
  copy.textContent = orientationCopy(title);

  const cta = document.createElement("button");
  cta.type = "button";
  cta.className = "orientation-cta";
  cta.textContent = "Mirar «" + title + "»";
  cta.addEventListener("click", function () {
    navigateToSuggested(title);
  });

  root.replaceChildren(heading, copy, cta);
  root.hidden = false;
}

function navigateToSuggested(title) {
  if (!el.form || !el.doubt) return;
  const q = "quiero entender " + title + " antes de seguir";
  el.doubt.value = q;
  el.doubt.dispatchEvent(new Event("input", { bubbles: true }));
  if (typeof el.form.requestSubmit === "function") {
    el.form.requestSubmit();
  } else {
    el.form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
  }
}
