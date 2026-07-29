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
