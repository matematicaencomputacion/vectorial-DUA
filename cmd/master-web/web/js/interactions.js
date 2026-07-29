import { studentId } from "./session.js";
import { applyDelta, renderDev, state } from "./state.js";
import { api, setStatus } from "./ui.js";

export async function recordBotonera(variantId, formatId, preferenceDelta, kind) {
  if (!state.currentNode) return;
  const body = {
    node_id: state.currentNode.node_id,
    student_id: studentId,
    schema_kind: kind || (state.currentNode.botonera_schema && state.currentNode.botonera_schema.kind) || "BOTONERA_SCHEMA_FLAT",
    variant_id: variantId,
    format_id: formatId || "",
    preference_delta: preferenceDelta || [],
    timestamp_unix_ms: Date.now(),
  };
  try {
    await api("/api/interactions/botonera", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    applyDelta(preferenceDelta);
    setStatus("Preferencia registrada en el perfil.", "ok");
  } catch (e) {
    // reportError already ran inside api()
  }
}

function progressStateCopy(state) {
  if (state === "visited") return { symbol: "✓", text: "Visitado" };
  if (state === "partial") return { symbol: "◐", text: "Exploración iniciada" };
  return { symbol: "○", text: "Por explorar" };
}

export function updateHierarchyProgress() {
  if (!state.hierarchyUI || !state.currentProgress) return;
  var root = state.hierarchyUI.root;
  var openedSet = new Set(state.currentProgress.opened_subtopic_ids || []);
  var explored = openedSet.size;
  var total = Number(state.currentProgress.total_subtopics || 0);

  state.hierarchyUI.summary.setAttribute("aria-label", "Exploraste " + explored + " de " + total + " subtemas");
  state.hierarchyUI.summaryText.textContent = "Exploraste " + explored + " de " + total + " subtemas.";

  var pendingRoots = [];
  (state.currentProgress.root_states || []).forEach(function (rootState) {
    if (rootState.state !== "visited") pendingRoots.push(rootState.title);
  });
  state.hierarchyUI.invitation.textContent = pendingRoots.length
    ? "Te queda por explorar: " + pendingRoots.join(", ") + ". Elegí por dónde seguir."
    : "Ya recorriste estas ramas. Podés volver a la que te resulte más útil.";

  var statesByID = {};
  (state.currentProgress.node_states || []).forEach(function (nodeState) {
    statesByID[nodeState.subtopic_id] = nodeState;
  });
  root.querySelectorAll("[data-subtopic-id]").forEach(function (trigger) {
    var nodeID = trigger.getAttribute("data-subtopic-id");
    var nodeState = statesByID[nodeID];
    var state = nodeState ? nodeState.state : "unvisited";
    var copy = progressStateCopy(state);
    var badge = trigger.querySelector(".subtopic-state");
    badge.dataset.state = state;
    badge.textContent = copy.symbol + " " + copy.text;
  });
}

function markSubtopicOpened(subtopicID, pathIDs) {
  if (!state.currentNode || !state.currentNode.hierarchy || !state.currentProgress) return;
  var opened = new Set((state.currentProgress && state.currentProgress.opened_subtopic_ids) || []);
  if (opened.has(subtopicID)) return;
  opened.add(subtopicID);
  state.currentProgress.opened_subtopic_ids = Array.from(opened);

  var statesByID = {};
  (state.currentProgress.node_states || []).forEach(function (nodeState) {
    statesByID[nodeState.subtopic_id] = nodeState;
  });
  (pathIDs || [subtopicID]).forEach(function (id) {
    var nodeState = statesByID[id];
    if (!nodeState) return;
    nodeState.opened_in_subtree = Number(nodeState.opened_in_subtree || 0) + 1;
    nodeState.state = nodeState.opened_in_subtree >= Number(nodeState.total_in_subtree || 0)
      ? "visited"
      : "partial";
  });
  (state.currentProgress.root_states || []).forEach(function (rootState) {
    var nodeState = statesByID[rootState.subtopic_id];
    if (nodeState) rootState.state = nodeState.state;
  });
  updateHierarchyProgress();
  renderDev();
}

export async function recordSubtopic(subtopicId, pathIds, orbitDelta) {
  if (!state.currentNode) return;
  // Reflejo inmediato; la próxima carga reconcilia contra el router.
  markSubtopicOpened(subtopicId, pathIds);
  const body = {
    parent_node_id: state.currentNode.node_id,
    student_id: studentId,
    subtopic_id: subtopicId,
    path_ids: pathIds || [],
    preference_delta: orbitDelta || [],
    timestamp_unix_ms: Date.now(),
  };
  try {
    await api("/api/interactions/subtopic", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    applyDelta(orbitDelta);
    setStatus("Subtema registrado.", "ok");
  } catch (e) {
    // reportError already ran inside api()
  }
}
