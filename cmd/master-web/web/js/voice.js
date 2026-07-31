import { auth } from "./session.js";
import { configureVoiceStop, el, renderDev } from "./state.js";
import { reportError, setStatus, setTextareaValue } from "./ui.js";

const MAX_RECORD_MS = 60000;
const SpeechRec = window.SpeechRecognition || window.webkitSpeechRecognition || null;

let activeVoice = null;
let voiceMode = "none"; // local | webspeech | none

export function getVoiceMode() {
  return voiceMode;
}

function micIconSVG() {
  return '<svg class="mic-pulse" viewBox="0 0 24 24" aria-hidden="true" focusable="false">' +
    '<path fill="currentColor" d="M12 14a3 3 0 0 0 3-3V6a3 3 0 0 0-6 0v5a3 3 0 0 0 3 3zm5-3a5 5 0 0 1-10 0H5a7 7 0 0 0 6 6.93V21h2v-3.07A7 7 0 0 0 19 11h-2z"/>' +
    "</svg>";
}

function setMicIdle(btn, idleLabel) {
  btn.setAttribute("aria-pressed", "false");
  btn.setAttribute("aria-label", idleLabel);
  btn.classList.remove("mic-recording");
  btn.innerHTML = micIconSVG() + '<span class="mic-label">Dictar</span>';
}

function setMicRecording(btn) {
  btn.setAttribute("aria-pressed", "true");
  btn.setAttribute("aria-label", "Grabando… tocá de nuevo para detener");
  btn.classList.add("mic-recording");
  btn.innerHTML = micIconSVG() + '<span class="mic-label">Grabando</span>';
}

function voiceErrorMessage(code) {
  switch (String(code || "")) {
    case "not-allowed":
    case "service-not-allowed":
      return "No tengo permiso para usar el micrófono. Podés habilitarlo en el navegador o escribir la duda.";
    case "audio-capture":
      return "No encontré un micrófono disponible. Revisá el dispositivo o escribí la duda.";
    case "no-speech":
      return "No detecté voz. Probá de nuevo o escribí la duda.";
    case "network":
      return "El reconocimiento de voz de la nube tuvo un problema de red. Si tenés STT local (AVLP_STT_URL), usalo; si no, escribí la duda.";
    case "aborted":
      return "";
    default:
      return "No pude completar el dictado. Probá de nuevo o escribí la duda.";
  }
}

function pickRecorderMime() {
  if (typeof MediaRecorder === "undefined") return "";
  const candidates = [
    "audio/webm;codecs=opus",
    "audio/webm",
    "audio/ogg;codecs=opus",
    "audio/mp4",
  ];
  for (const mime of candidates) {
    if (MediaRecorder.isTypeSupported && MediaRecorder.isTypeSupported(mime)) {
      return mime;
    }
  }
  return "";
}

function filenameForMime(mime) {
  if (String(mime).includes("ogg")) return "audio.ogg";
  if (String(mime).includes("mp4") || String(mime).includes("m4a")) return "audio.m4a";
  return "audio.webm";
}

async function postTranscribe(blob, filename) {
  try {
    await auth.ready;
  } catch (_) {
    /* surfaced below */
  }
  const headers = {};
  if (auth.secureMode && auth.token) {
    headers.Authorization = "Bearer " + auth.token;
  }
  const form = new FormData();
  form.append("file", blob, filename);
  const res = await fetch("/api/transcribe", { method: "POST", headers, body: form });
  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch (_) {
    data = { message: text };
  }
  if (!res.ok) {
    const err = new Error((data && (data.student_message || data.message)) || res.statusText);
    err.status = res.status;
    err.data = data;
    throw err;
  }
  return (data && data.text) || "";
}

function stopActiveVoice(opts) {
  opts = opts || {};
  if (!activeVoice) return;
  const session = activeVoice;
  activeVoice = null;
  if (session.timer) {
    clearTimeout(session.timer);
    session.timer = null;
  }
  if (session.kind === "webspeech" && session.recognition) {
    try {
      session.recognition.onresult = null;
      session.recognition.onerror = null;
      session.recognition.onend = null;
      session.recognition.stop();
    } catch (_) {}
  }
  if (session.kind === "local") {
    session.stopping = true;
    try {
      if (session.recorder && session.recorder.state !== "inactive") {
        session.recorder.stop();
      }
    } catch (_) {}
    try {
      if (session.stream) {
        session.stream.getTracks().forEach(function (t) { t.stop(); });
      }
    } catch (_) {}
  }
  setMicIdle(session.btn, session.idleLabel);
  if (opts.cancelled) {
    setStatus("Dictado cancelado. Podés editar el texto o volver a dictar.", "ok");
  }
}

function startWebSpeech(textarea, btn, idleLabel, readyHint) {
  if (!SpeechRec) return;
  if (activeVoice && activeVoice.btn === btn) {
    stopActiveVoice({ cancelled: true });
    return;
  }
  if (activeVoice) stopActiveVoice({ cancelled: true });

  const recognition = new SpeechRec();
  recognition.lang = "es-AR";
  recognition.interimResults = true;
  recognition.continuous = false;
  recognition.maxAlternatives = 1;

  const base = textarea.value;
  activeVoice = { kind: "webspeech", recognition, btn, textarea, base, idleLabel };
  setMicRecording(btn);
  setStatus("Escuchando (Web Speech)… hablá tu duda. Un segundo toque cancela.", "ok");

  recognition.onresult = function (event) {
    let interim = "";
    let finalText = "";
    for (let i = event.resultIndex; i < event.results.length; i++) {
      const piece = event.results[i][0].transcript;
      if (event.results[i].isFinal) finalText += piece;
      else interim += piece;
    }
    const prefix = base && String(base).trim() ? String(base).replace(/\s+$/, "") + " " : "";
    if (finalText) {
      setTextareaValue(textarea, prefix + finalText.trim());
      stopActiveVoice({});
      setStatus(readyHint || "Dictado listo. Revisá el texto y confirmá con el botón correspondiente.", "ok");
      textarea.focus();
    } else {
      setTextareaValue(textarea, prefix + interim);
    }
  };

  recognition.onerror = function (event) {
    const msg = voiceErrorMessage(event && event.error);
    stopActiveVoice({});
    if (msg) reportError(null, msg);
  };

  recognition.onend = function () {
    if (activeVoice && activeVoice.recognition === recognition) {
      stopActiveVoice({});
    }
  };

  try {
    recognition.start();
  } catch (e) {
    stopActiveVoice({});
    reportError(e, "No pude iniciar el micrófono. Probá de nuevo o escribí la duda.");
  }
}

async function startLocalRecord(textarea, btn, idleLabel, readyHint) {
  if (activeVoice && activeVoice.btn === btn) {
    // Second tap: stop and transcribe.
    const session = activeVoice;
    session.userStop = true;
    try {
      if (session.recorder && session.recorder.state !== "inactive") session.recorder.stop();
    } catch (_) {}
    return;
  }
  if (activeVoice) stopActiveVoice({ cancelled: true });

  if (typeof MediaRecorder === "undefined" || !navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
    reportError(null, "Este navegador no puede grabar audio localmente. Escribí la duda o usá un navegador compatible.");
    return;
  }

  let stream;
  try {
    stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  } catch (e) {
    const name = (e && e.name) || "";
    if (name === "NotAllowedError" || name === "PermissionDeniedError") {
      reportError(null, voiceErrorMessage("not-allowed"));
    } else if (name === "NotFoundError" || name === "DevicesNotFoundError") {
      reportError(null, voiceErrorMessage("audio-capture"));
    } else {
      reportError(e, "No pude acceder al micrófono. Probá de nuevo o escribí la duda.");
    }
    return;
  }

  const mime = pickRecorderMime();
  let recorder;
  try {
    recorder = mime ? new MediaRecorder(stream, { mimeType: mime }) : new MediaRecorder(stream);
  } catch (e) {
    stream.getTracks().forEach(function (t) { t.stop(); });
    reportError(e, "No pude iniciar la grabación. Probá de nuevo o escribí la duda.");
    return;
  }

  const chunks = [];
  const session = {
    kind: "local",
    btn,
    textarea,
    idleLabel,
    readyHint,
    stream,
    recorder,
    chunks,
    timer: null,
    userStop: false,
    stopping: false,
    autoCut: false,
  };
  activeVoice = session;
  setMicRecording(btn);
  setStatus("Grabando (STT local)… tocá de nuevo para detener. Máximo 60 s.", "ok");

  recorder.ondataavailable = function (ev) {
    if (ev.data && ev.data.size) chunks.push(ev.data);
  };

  recorder.onerror = function () {
    stopActiveVoice({});
    reportError(null, "La grabación falló. Probá de nuevo o escribí la duda.");
  };

  recorder.onstop = async function () {
    const mine = session;
    try {
      stream.getTracks().forEach(function (t) { t.stop(); });
    } catch (_) {}
    if (mine.timer) {
      clearTimeout(mine.timer);
      mine.timer = null;
    }
    if (activeVoice === mine) activeVoice = null;
    setMicIdle(btn, idleLabel);

    if (mine.stopping && !mine.userStop && !mine.autoCut) {
      return;
    }
    const blob = new Blob(chunks, { type: recorder.mimeType || mime || "audio/webm" });
    if (!blob.size) {
      reportError(null, "No capturé audio. Probá de nuevo o escribí la duda.");
      return;
    }
    setStatus("Transcribiendo…", "ok");
    try {
      const text = await postTranscribe(blob, filenameForMime(blob.type || mime));
      const base = textarea.value;
      const prefix = base && String(base).trim() ? String(base).replace(/\s+$/, "") + " " : "";
      setTextareaValue(textarea, prefix + String(text || "").trim());
      const hint = readyHint || "Dictado listo. Revisá el texto y confirmá con el botón correspondiente.";
      const cut = mine.autoCut ? " (corte a 60 s)." : ".";
      setStatus(hint.replace(/\.$/, "") + cut, "ok");
      textarea.focus();
    } catch (err) {
      reportError(err, "No pude transcribir el audio. Probá de nuevo o escribí la duda.");
    }
  };

  session.timer = setTimeout(function () {
    if (activeVoice !== session) return;
    session.autoCut = true;
    session.userStop = true;
    try {
      if (recorder.state !== "inactive") recorder.stop();
    } catch (_) {}
  }, MAX_RECORD_MS);

  try {
    recorder.start(250);
  } catch (e) {
    stopActiveVoice({});
    reportError(e, "No pude iniciar la grabación. Probá de nuevo o escribí la duda.");
  }
}

function startVoiceFor(textarea, btn, idleLabel, readyHint) {
  if (voiceMode === "local") {
    startLocalRecord(textarea, btn, idleLabel, readyHint);
    return;
  }
  if (voiceMode === "webspeech") {
    startWebSpeech(textarea, btn, idleLabel, readyHint);
  }
}

function mountMicButton(wrap, textarea, opts) {
  if (!wrap || !textarea || voiceMode === "none") return null;
  const idleLabel = opts.idleLabel || "Dictar por voz";
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "mic-btn";
  setMicIdle(btn, idleLabel);
  btn.title = opts.title || idleLabel;
  btn.addEventListener("click", function () {
    startVoiceFor(textarea, btn, idleLabel, opts.readyHint);
  });
  wrap.appendChild(btn);
  return btn;
}

let doubtMicBtn = null;
let askMicBtn = null;

function mountAllMics() {
  const doubtMicHint = document.getElementById("doubt-mic-hint");
  doubtMicBtn = mountMicButton(
    document.getElementById("doubt-mic-wrap"),
    el.doubt,
    {
      idleLabel: "Dictar tu duda por voz",
      title: "Dictar (Ctrl+M). Segundo toque detiene/cancela.",
      readyHint: "Dictado listo. Revisá el texto y pulsá «Buscar estación».",
    }
  );
  askMicBtn = mountMicButton(
    document.getElementById("ask-mic-wrap"),
    el.askDoubt,
    {
      idleLabel: "Dictar la duda diferente por voz",
      title: "Dictar. Segundo toque detiene/cancela.",
      readyHint: "Dictado listo. Revisá el texto y pulsá «Generar botón en vivo».",
    }
  );
  if (doubtMicBtn && doubtMicHint) {
    doubtMicHint.hidden = false;
  }
}

function resolveVoiceMode() {
  if (auth.sttEnabled) return "local";
  if (SpeechRec) return "webspeech";
  return "none";
}

auth.ready.then(function () {
  voiceMode = resolveVoiceMode();
  auth.voiceMode = voiceMode;
  mountAllMics();
  renderDev();
}).catch(function () {
  voiceMode = SpeechRec ? "webspeech" : "none";
  auth.voiceMode = voiceMode;
  mountAllMics();
  renderDev();
});

document.addEventListener("keydown", function (ev) {
  if (!(ev.ctrlKey || ev.metaKey) || String(ev.key).toLowerCase() !== "m") return;
  if (!doubtMicBtn) return;
  const tag = (ev.target && ev.target.tagName) || "";
  if (tag === "INPUT" && ev.target !== el.frustration) return;
  ev.preventDefault();
  doubtMicBtn.click();
});

configureVoiceStop(function () {
  if (activeVoice && activeVoice.textarea === el.askDoubt) {
    stopActiveVoice({ cancelled: true });
  }
});
