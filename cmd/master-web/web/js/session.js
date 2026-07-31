const STUDENT_SESSION_KEY = "avlp-master-student-id";

function resolveStudentId() {
  var generated = "stu-" + Math.random().toString(36).slice(2, 10) + "-" + Date.now().toString(36);
  try {
    var existing = window.sessionStorage.getItem(STUDENT_SESSION_KEY);
    if (existing) return existing;
    window.sessionStorage.setItem(STUDENT_SESSION_KEY, generated);
  } catch (_) {
    // En contextos que bloquean storage, conserva el comportamiento en memoria.
  }
  return generated;
}

export class RequestGeneration {
  #value = 0;

  begin() {
    this.#value += 1;
    return this.#value;
  }

  token() {
    return this.#value;
  }

  isCurrent(token) {
    return token === this.#value;
  }

  ifCurrent(token, callback) {
    if (!this.isCurrent(token)) return undefined;
    return callback();
  }
}

export const studentId = resolveStudentId();
export const requestGeneration = new RequestGeneration();

/** Session token kept in memory only (never localStorage). */
export const auth = {
  token: "",
  role: "student",
  secureMode: false,
  sttEnabled: false,
  voiceMode: "none",
  ready: null,
};

export async function ensureSession(teacherKey) {
  const body = { student_id: studentId };
  if (teacherKey) body.teacher_key = teacherKey;
  const res = await fetch("/api/session", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
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
  auth.token = data.token || "";
  auth.role = data.role || "student";
  auth.secureMode = !!data.secure_mode;
  auth.sttEnabled = !!data.stt_enabled;
  return data;
}

auth.ready = ensureSession();
