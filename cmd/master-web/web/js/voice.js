import { configureVoiceStop, el } from "./state.js";
import { reportError, setStatus, setTextareaValue } from "./ui.js";

let activeVoice = null;

// —— Dictado por voz (progresivo: solo si el browser expone SpeechRecognition) ——
const SpeechRec = window.SpeechRecognition || window.webkitSpeechRecognition || null;

function micIconSVG() {
  return '<svg class="mic-pulse" viewBox="0 0 24 24" aria-hidden="true" focusable="false">' +
    '<path fill="currentColor" d="M12 14a3 3 0 0 0 3-3V6a3 3 0 0 0-6 0v5a3 3 0 0 0 3 3zm5-3a5 5 0 0 1-10 0H5a7 7 0 0 0 6 6.93V21h2v-3.07A7 7 0 0 0 19 11h-2z"/>' +
    "</svg>";
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
      return "El reconocimiento de voz tuvo un problema de red. Probá otra vez o escribí la duda.";
    case "aborted":
      return "";
    default:
      return "No pude completar el dictado. Probá de nuevo o escribí la duda.";
  }
}

function stopActiveVoice(opts) {
  opts = opts || {};
  if (!activeVoice) return;
  var session = activeVoice;
  activeVoice = null;
  try { session.recognition.onresult = null; session.recognition.onerror = null; session.recognition.onend = null; session.recognition.stop(); } catch (_) {}
  session.btn.setAttribute("aria-pressed", "false");
  session.btn.setAttribute("aria-label", session.idleLabel);
  if (opts.cancelled) {
    setStatus("Dictado cancelado. Podés editar el texto o volver a dictar.", "ok");
  }
}

function startVoiceFor(textarea, btn, idleLabel, readyHint) {
  if (!SpeechRec) return;
  if (activeVoice && activeVoice.btn === btn) {
    stopActiveVoice({ cancelled: true });
    return;
  }
  if (activeVoice) {
    stopActiveVoice({ cancelled: true });
  }

  var recognition = new SpeechRec();
  recognition.lang = "es-AR";
  recognition.interimResults = true;
  recognition.continuous = false;
  recognition.maxAlternatives = 1;

  var base = textarea.value;
  activeVoice = { recognition: recognition, btn: btn, textarea: textarea, base: base, idleLabel: idleLabel };

  btn.setAttribute("aria-pressed", "true");
  btn.setAttribute("aria-label", "Escuchando… tocá de nuevo para cancelar");
  setStatus("Escuchando… hablá tu duda. Un segundo toque cancela.", "ok");

  recognition.onresult = function (event) {
    var interim = "";
    var finalText = "";
    for (var i = event.resultIndex; i < event.results.length; i++) {
      var piece = event.results[i][0].transcript;
      if (event.results[i].isFinal) finalText += piece;
      else interim += piece;
    }
    var prefix = base && String(base).trim() ? String(base).replace(/\s+$/, "") + " " : "";
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
    var msg = voiceErrorMessage(event && event.error);
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

function mountMicButton(wrap, textarea, opts) {
  if (!SpeechRec || !wrap || !textarea) return null;
  var idleLabel = opts.idleLabel || "Dictar por voz";
  var btn = document.createElement("button");
  btn.type = "button";
  btn.className = "mic-btn";
  btn.setAttribute("aria-pressed", "false");
  btn.setAttribute("aria-label", idleLabel);
  btn.title = opts.title || idleLabel;
  btn.innerHTML = micIconSVG();
  btn.addEventListener("click", function () {
    startVoiceFor(textarea, btn, idleLabel, opts.readyHint);
  });
  wrap.appendChild(btn);
  return btn;
}

var doubtMicHint = document.getElementById("doubt-mic-hint");
var doubtMicBtn = mountMicButton(
  document.getElementById("doubt-mic-wrap"),
  el.doubt,
  {
    idleLabel: "Dictar tu duda por voz",
    title: "Dictar (Ctrl+M). Segundo toque cancela.",
    readyHint: "Dictado listo. Revisá el texto y pulsá «Buscar estación».",
  }
);
var askMicBtn = mountMicButton(
  document.getElementById("ask-mic-wrap"),
  el.askDoubt,
  {
    idleLabel: "Dictar la duda diferente por voz",
    title: "Dictar. Segundo toque cancela.",
    readyHint: "Dictado listo. Revisá el texto y pulsá «Generar botón en vivo».",
  }
);
if (doubtMicBtn && doubtMicHint) {
  doubtMicHint.hidden = false;
}

document.addEventListener("keydown", function (ev) {
  if (!(ev.ctrlKey || ev.metaKey) || String(ev.key).toLowerCase() !== "m") return;
  if (!doubtMicBtn) return;
  var tag = (ev.target && ev.target.tagName) || "";
  // Permitir desde textarea de duda u otros controles; no interferir con inputs de contraseña ajenos.
  if (tag === "INPUT" && ev.target !== el.frustration) return;
  ev.preventDefault();
  doubtMicBtn.click();
});


configureVoiceStop(function () {
  if (activeVoice && activeVoice.textarea === el.askDoubt) {
    stopActiveVoice({ cancelled: true });
  }
});
