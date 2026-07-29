import { renderRail } from "./rail.js";
import { requestGeneration, studentId } from "./session.js";
import { el, renderDev, setAskMode, state } from "./state.js";
import { api, reportError, setStageFromMedia, setStatus, setTextareaValue, showWaiting } from "./ui.js";
import "./voice.js";

let pollTimer = null;

function stopPoll() {
  if (pollTimer) clearInterval(pollTimer);
  pollTimer = null;
}

async function loadProgressForNode(node, token) {
  if (token == null) token = requestGeneration.token();
  if (!requestGeneration.isCurrent(token)) return;
  state.rawProgress = null;
  state.currentProgress = null;
  if (node.hierarchy) {
    try {
      const progress = await api(
        "/api/nodes/" + encodeURIComponent(node.node_id) + "/progress?student_id=" + encodeURIComponent(studentId)
      );
      if (!requestGeneration.isCurrent(token)) return;
      state.rawProgress = progress;
      state.currentProgress = JSON.parse(JSON.stringify(state.rawProgress));
    } catch (_) {
      if (!requestGeneration.isCurrent(token)) return;
      // El nodo sigue siendo navegable; la próxima carga vuelve a reconciliar.
      state.currentProgress = {
        student_id: studentId,
        parent_node_id: node.node_id,
        opened_subtopic_ids: [],
        total_subtopics: 0,
        root_states: [],
        node_states: [],
      };
    }
  }
}

async function loadInteractiveNode(nodeId, token) {
  if (token == null) token = requestGeneration.token();
  if (!requestGeneration.isCurrent(token)) return;
  const node = await api("/api/nodes/" + encodeURIComponent(nodeId));
  if (!requestGeneration.isCurrent(token)) return;
  state.currentNode = node;
  await loadProgressForNode(node, token);
  if (!requestGeneration.isCurrent(token)) return;
  renderRail(node);
  setStageFromMedia({
    title: node.titulo,
    meta: (node.dimension_dua || "") + " · interactivo",
    mediaUrl: node.stage_media_default || (node.hierarchy && node.hierarchy.macro_media_url),
    markdown: node.stage_markdown_default,
    clipTitle: node.titulo,
  });
  renderDev();
  setStatus("Nodo interactivo cargado.", "ok");
}

function showNonInteractiveRail(kind) {
  state.currentNode = null;
  state.rawProgress = null;
  state.currentProgress = null;
  state.hierarchyUI = null;
  setAskMode(false);
  if (kind === "live") {
    el.railTopic.textContent = "Estación en vivo";
    el.railBody.innerHTML = "<p style='margin:0;color:var(--ink-soft)'>Contenido listo en el Stage. Para una duda nueva sobre otro tema, buscá arriba.</p>";
  } else {
    el.railTopic.textContent = "Nodo estático";
    el.railBody.innerHTML = "<p style='margin:0;color:var(--ink-soft)'>Sin payload interactivo. Mostramos el recurso ruteado.</p>";
  }
  renderDev();
}

async function handleMatched(matched, token) {
  if (token == null) token = requestGeneration.token();
  if (!requestGeneration.isCurrent(token)) return;
  state.lastSimilarity = matched.similarity_score;
  renderDev();
  if (matched.has_interactive_payload) {
    await loadInteractiveNode(matched.node_id, token);
    return;
  }
  if (!requestGeneration.isCurrent(token)) return;
  showNonInteractiveRail(matched.is_live_generated ? "live" : "static");
  if (matched.live_content) {
    setStageFromMedia({
      title: "Estación en vivo",
      meta: "similitud " + Number(matched.similarity_score || 0).toFixed(3),
      markdown: matched.live_content,
    });
  } else {
    setStageFromMedia({
      title: matched.node_id || "Nodo",
      meta: (matched.dimension_dua || "") + " · sim " + Number(matched.similarity_score || 0).toFixed(3),
      mediaUrl: matched.resource_url,
      clipTitle: matched.node_id,
    });
  }
  setStatus(matched.is_live_generated ? "Estación generada en el miss path." : "Match estático.", "ok");
}

async function pollStation(trackingUlid, token) {
  if (token == null) token = requestGeneration.token();
  stopPoll();
  const tick = async function () {
    if (!requestGeneration.isCurrent(token)) {
      return;
    }
    try {
      const st = await api("/api/stations/" + encodeURIComponent(trackingUlid) + "?student_id=" + encodeURIComponent(studentId));
      if (!requestGeneration.isCurrent(token)) {
        return;
      }
      const msg = st.student_message || "Seguimos preparando tu estación…";
      if (st.status === "ready") {
        stopPoll();
        state.lastSimilarity = null;
        if (st.live_content) {
          showNonInteractiveRail("live");
          setStageFromMedia({
            title: "Estación lista",
            meta: (st.retrieved_sources || []).join(", ") || (st.node_id || ""),
            markdown: st.live_content,
          });
          // If the live node also has an interactive seed (rare), upgrade the rail.
          if (st.node_id) {
            try {
              const node = await fetch("/api/nodes/" + encodeURIComponent(st.node_id)).then(function (r) {
                return r.ok ? r.json() : null;
              });
              if (!requestGeneration.isCurrent(token)) return;
              if (node && node.node_id) {
                state.currentNode = node;
                await loadProgressForNode(node, token);
                if (!requestGeneration.isCurrent(token)) return;
                renderRail(node);
                renderDev();
              }
            } catch (_) { /* keep non-interactive rail */ }
          }
        } else if (st.node_id) {
          try {
            await loadInteractiveNode(st.node_id, token);
          } catch (_) {
            if (!requestGeneration.isCurrent(token)) return;
            showNonInteractiveRail("live");
            setStageFromMedia({
              title: "Estación lista",
              meta: st.node_id,
              markdown: msg,
            });
          }
        } else {
          showNonInteractiveRail("live");
          setStageFromMedia({
            title: "Estación lista",
            meta: "",
            markdown: msg,
          });
        }
        if (!requestGeneration.isCurrent(token)) return;
        setStatus("La estación está lista.", "ok");
        return;
      }
      if (st.status === "failed") {
        stopPoll();
        setAskMode(false);
        showWaiting(msg, trackingUlid);
        setStatus(msg, "error");
        return;
      }
      showWaiting(msg, trackingUlid);
      setStatus(msg, "");
    } catch (e) {
      if (!requestGeneration.isCurrent(token)) {
        return;
      }
      stopPoll();
      setAskMode(false);
      showWaiting((e.data && e.data.student_message) || e.message, trackingUlid);
      // reportError already ran inside api()
    }
  };
  await tick();
  if (requestGeneration.isCurrent(token)) {
    pollTimer = setInterval(tick, 2000);
  }
}

el.form.addEventListener("submit", async function (ev) {
  ev.preventDefault();
  const queryText = el.doubt.value.trim();
  if (!queryText) return;
  // Nueva búsqueda: invalida cargas/polling en vuelo y para el intervalo.
  const token = requestGeneration.begin();
  stopPoll();
  const frustration = Number(el.frustration.value);
  setStatus("Consultando el router…", "");
  try {
    const route = await api("/api/query", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        student_id: studentId,
        query_text: queryText,
        frustration: frustration,
      }),
    });
    if (!requestGeneration.isCurrent(token)) return;
    if (route.matched) {
      await handleMatched(route.matched, token);
    } else if (route.pending) {
      state.currentNode = null;
      state.rawProgress = null;
      state.currentProgress = null;
      state.hierarchyUI = null;
      setAskMode(false);
      state.lastSimilarity = null;
      renderDev();
      showWaiting(route.pending.message, route.pending.tracking_ulid);
      setStatus(route.pending.message || "Estación pendiente…", "");
      await pollStation(route.pending.tracking_ulid, token);
    } else {
      setStatus("Respuesta de ruteo inesperada.", "error");
    }
  } catch (e) {
    if (!requestGeneration.isCurrent(token)) return;
    // reportError already ran inside api() for HTTP failures
    if (!e || !e.status) {
      reportError(e, "Error de consulta. Probá de nuevo en un momento.");
    }
  }
});

el.btnMutate.addEventListener("click", async function () {
  if (!state.interactiveSession || !state.currentNode || !state.currentNode.node_id) {
    reportError(null, "Para una duda nueva sobre otro tema, buscá arriba.");
    setAskMode(false);
    return;
  }
  const doubt = el.askDoubt.value.trim();
  if (!doubt) {
    setStatus("Escribí la duda diferente antes de generar el botón.", "error");
    el.askDoubt.focus();
    return;
  }
  const token = requestGeneration.token();
  el.btnMutate.disabled = true;
  try {
    const res = await api("/api/nodes/" + encodeURIComponent(state.currentNode.node_id) + "/mutate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        student_id: studentId,
        doubt_text: doubt,
        frustration: Number(el.frustration.value),
      }),
    });
    if (!requestGeneration.isCurrent(token)) return;
    state.currentNode = res.node || state.currentNode;
    renderRail(state.currentNode);
    if (res.button) {
      setStageFromMedia({
        title: state.currentNode.titulo,
        meta: "Nuevo botón en vivo",
        mediaUrl: res.button.media_url,
        cellCode: res.button.cell_code,
        clipTitle: res.button.label,
      });
    }
    setTextareaValue(el.askDoubt, "");
    setStatus("Botón en vivo agregado a la botonera.", "ok");
    renderDev();
  } catch (e) {
    // reportError already ran inside api()
  } finally {
    el.btnMutate.disabled = false;
  }
});

document.querySelectorAll(".hints [data-hint]").forEach(function (btn) {
  btn.addEventListener("click", function () {
    setTextareaValue(el.doubt, btn.getAttribute("data-hint"));
    el.doubt.focus();
    setStatus("Ejemplo cargado en «Tu duda». Pulsá «Buscar estación» para ejecutar.", "ok");
  });
});

setAskMode(false);
renderDev();
