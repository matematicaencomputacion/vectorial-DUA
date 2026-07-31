import { el, setAskMode, state } from "./state.js";

function syncClearButton(textarea, btn) {
  if (!textarea || !btn) return;
  btn.hidden = !String(textarea.value || "").length;
}

export function wireClearButton(textarea, btn) {
  if (!textarea || !btn) return;
  syncClearButton(textarea, btn);
  textarea.addEventListener("input", function () {
    syncClearButton(textarea, btn);
  });
  btn.addEventListener("click", function () {
    setTextareaValue(textarea, "");
    textarea.focus();
  });
}

wireClearButton(el.doubt, el.doubtClear);
wireClearButton(el.askDoubt, el.askDoubtClear);

export function setTextareaValue(textarea, value) {
  textarea.value = value;
  textarea.dispatchEvent(new Event("input", { bubbles: true }));
}

export function setStatus(msg, tone) {
  el.status.textContent = msg || "";
  el.status.dataset.tone = tone || "";
  const isError = tone === "error";
  el.status.setAttribute("aria-live", isError ? "assertive" : "polite");
  el.status.setAttribute("role", isError ? "alert" : "status");
}

export function reportError(err, fallback) {
  var msg = fallback || "Algo no salió bien; probá de nuevo o reformulá la duda.";
  if (err && err.data && err.data.student_message) {
    msg = err.data.student_message;
  } else if (err && err.message && String(err.message).trim()) {
    msg = err.message;
  }
  msg = sanitizeStudentError(msg, err && err.status);
  setStatus(msg, "error");
}

function sanitizeStudentError(msg, status) {
  var raw = String(msg || "");
  var technical = /dial tcp|transport:|connection error|ECONNREFUSED|rpc error|bad gateway|502|503|504/i.test(raw);
  if ((status && status >= 500) || technical) {
    if (raw && raw !== "No pudimos conectar con el tutor en este momento; probá de nuevo en un instante") {
      console.log("detalle técnico (oculto al estudiante):", status, raw);
    }
    return "No pudimos conectar con el tutor en este momento; probá de nuevo en un instante";
  }
  return raw || "Algo no salió bien; probá de nuevo o reformulá la duda.";
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function simpleMarkdown(md) {
  const lines = String(md || "").split(/\r?\n/);
  const out = [];
  let inCode = false;
  let codeBuf = [];
  function flushCode() {
    if (!codeBuf.length) return;
    out.push("<pre><code>" + escapeHtml(codeBuf.join("\n")) + "</code></pre>");
    codeBuf = [];
  }
  for (const raw of lines) {
    if (raw.trim().startsWith("```")) {
      if (inCode) { flushCode(); inCode = false; }
      else { inCode = true; }
      continue;
    }
    if (inCode) { codeBuf.push(raw); continue; }
    let line = escapeHtml(raw);
    line = line.replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>");
    line = line.replace(/`([^`]+)`/g, "<code>$1</code>");
    if (/^### /.test(raw)) out.push("<h3>" + escapeHtml(raw.slice(4)) + "</h3>");
    else if (/^## /.test(raw)) out.push("<h2>" + escapeHtml(raw.slice(3)) + "</h2>");
    else if (/^# /.test(raw)) out.push("<h1>" + escapeHtml(raw.slice(2)) + "</h1>");
    else if (/^[-*] /.test(raw)) out.push("<li>" + line.replace(/^[-*] /, "") + "</li>");
    else if (raw.trim() === "") out.push("");
    else out.push("<p>" + line + "</p>");
  }
  flushCode();
  return out.join("\n");
}

export function schemaKind(schema) {
  const k = (schema && schema.kind) || "";
  const s = String(k).toLowerCase();
  if (s.includes("depth") || s === "1") return "depth";
  if (s.includes("cognitive") || s === "2") return "cognitive";
  if (s.includes("emergency") || s === "3") return "emergency";
  if (s.includes("combined") || s === "4") return "combined";
  if (s.includes("flat") || s === "0") return "flat";
  return s || "flat";
}

export function actionIsAsk(btn) {
  const a = String(btn.action_type || "").toUpperCase();
  return a.includes("ASK_AGENT") || a === "2" || btn.id_btn === "ask_different";
}

export async function api(path, opts) {
  var res;
  try {
    res = await fetch(path, opts);
  } catch (netErr) {
    reportError(netErr, "No pude contactar al servidor. Revisá que el router y master-web estén en marcha.");
    throw netErr;
  }
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch (_) { data = { message: text }; }
  if (!res.ok) {
    const err = new Error((data && (data.student_message || data.message)) || res.statusText);
    err.status = res.status;
    err.data = data;
    if (data && (data.message || data.student_message)) {
      console.log("api error detail:", res.status, data);
    }
    reportError(err);
    throw err;
  }
  return data;
}

export function showWaiting(message, tracking) {
  el.stageTitle.textContent = "Preparando tu estación";
  el.stageMeta.textContent = tracking ? ("seguimiento " + tracking) : "";
  el.stageMedia.innerHTML =
    '<div class="waiting">' +
    '<div class="pulse" aria-hidden="true"></div>' +
    "<p><strong>" + escapeHtml(message || "Estamos preparando una estación para tu duda.") + "</strong></p>" +
    "<p>Seguimos consultando el estado; este mensaje se actualiza solo.</p>" +
    "</div>";
  el.railTopic.textContent = "Estación en vivo";
  el.railBody.innerHTML = "<p style='margin:0;color:var(--ink-soft)'>Cuando esté lista, el contenido aparece en el Stage. Para una duda nueva sobre otro tema, buscá arriba.</p>";
  setAskMode(false);
}

function showStageContent(title, meta, contentHtml) {
  el.stageTitle.textContent = title || "Stage";
  el.stageMeta.textContent = meta || "";
  el.stageMedia.innerHTML = contentHtml;
  wireTranscriptToggle(el.stageMedia);
}

function escapeAttr(s) {
  return escapeHtml(s).replace(/"/g, "&quot;");
}

function looksLikeVideo(url) {
  const u = String(url || "");
  return /\.(mp4|webm|ogg)(\?|$)/i.test(u) || /\/videos\//i.test(u);
}

function transcriptBlock(transcript) {
  return (
    '<div class="transcript-block">' +
    '<button type="button" class="transcript-toggle" aria-expanded="false" aria-controls="stage-transcript-panel">' +
    '<span aria-hidden="true">📄</span> Ver transcripción</button>' +
    '<div id="stage-transcript-panel" class="transcript-panel md" hidden>' +
    simpleMarkdown(transcript) +
    "</div></div>"
  );
}

function a11yFooter(opts) {
  let html = "";
  if (opts.altText) {
    html +=
      '<p class="media-alt">' + escapeHtml(opts.altText) + "</p>";
  }
  if (opts.transcript) {
    html += transcriptBlock(opts.transcript);
  }
  return html;
}

function wireTranscriptToggle(root) {
  const btn = root.querySelector(".transcript-toggle");
  const panel = root.querySelector(".transcript-panel");
  if (!btn || !panel) return;
  btn.addEventListener("click", function () {
    const open = btn.getAttribute("aria-expanded") === "true";
    btn.setAttribute("aria-expanded", open ? "false" : "true");
    panel.hidden = open;
    btn.innerHTML = open
      ? '<span aria-hidden="true">📄</span> Ver transcripción'
      : '<span aria-hidden="true">📄</span> Ocultar transcripción';
  });
}

function mediaPlaceholder(title, mediaUrl, extraHtml) {
  return (
    "<div>" +
    '<p class="placeholder-title">' + escapeHtml(title || "Clip activo") + "</p>" +
    (mediaUrl ? '<p class="media-url">' + escapeHtml(mediaUrl) + "</p>" : "") +
    (extraHtml || "") +
    '<p class="media-url" style="margin-top:0.75rem">Vista prototipo: la URL del contrato se muestra aunque el CDN de ejemplo no sirva bytes.</p>' +
    "</div>"
  );
}

function mediaVideo(opts) {
  const track = opts.captionsUrl
    ? '<track kind="captions" srclang="es" label="Español" src="' +
      escapeAttr(opts.captionsUrl) +
      '" default>'
    : "";
  const label = opts.altText
    ? ' aria-label="' + escapeAttr(opts.altText) + '"'
    : "";
  return (
    "<div>" +
    '<video class="stage-video" controls' +
    label +
    ">" +
    '<source src="' +
    escapeAttr(opts.mediaUrl) +
    '">' +
    track +
    "Tu navegador no reproduce este video.</video>" +
    a11yFooter(opts) +
    '<p class="media-url" style="margin-top:0.75rem">Vista prototipo: el <code>&lt;video&gt;</code> respeta captions del contrato aunque el CDN de ejemplo no sirva bytes.</p>' +
    "</div>"
  );
}

export function mediaA11yFrom(src) {
  if (!src) return {};
  return {
    altText: src.alt_text || "",
    transcript: src.transcript || "",
    captionsUrl: src.captions_url || "",
    audioDescriptionUrl: src.audio_description_url || "",
  };
}

export function setStageFromMedia(opts) {
  const title = opts.title || (state.currentNode && state.currentNode.titulo) || "Stage";
  const meta = opts.meta || "";
  if (opts.hintText) {
    showStageContent(title, meta, '<div class="hint">' + escapeHtml(opts.hintText) + "</div>");
    return;
  }
  if (opts.cellCode) {
    showStageContent(
      title,
      meta,
      "<pre><code>" + escapeHtml(opts.cellCode) + "</code></pre>" + a11yFooter(opts)
    );
    return;
  }
  if (opts.markdown) {
    showStageContent(
      title,
      meta,
      '<div class="md">' +
        simpleMarkdown(opts.markdown) +
        "</div>" +
        a11yFooter({
          altText: opts.altText,
          // Si el markdown del Stage ya es el transcript (p. ej. estación promovida), no duplicar.
          transcript: opts.transcript && opts.transcript !== opts.markdown ? opts.transcript : "",
        })
    );
    return;
  }
  if (looksLikeVideo(opts.mediaUrl)) {
    showStageContent(title, meta, mediaVideo(opts));
    return;
  }
  showStageContent(
    title,
    meta,
    mediaPlaceholder(opts.clipTitle || title, opts.mediaUrl, a11yFooter(opts))
  );
}
