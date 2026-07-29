import { recordBotonera, recordSubtopic, updateHierarchyProgress } from "./interactions.js";
import { el, setAskMode, state } from "./state.js";
import { actionIsAsk, schemaKind, setStageFromMedia } from "./ui.js";

function renderLegacyButtons(buttons, mount) {
  const list = document.createElement("div");
  list.className = "btn-list";
  list.setAttribute("role", "group");
  list.setAttribute("aria-label", "Botones del nodo");
  (buttons || []).forEach(function (btn) {
    if (actionIsAsk(btn)) return; // handled by ask-box
    const b = document.createElement("button");
    b.type = "button";
    b.className = "legacy-btn";
    b.textContent = btn.label || btn.id_btn;
    if (btn.is_live_generated) b.textContent = (btn.label || "LIVE") + " ✦";
    b.addEventListener("click", async function () {
      list.querySelectorAll("button").forEach(function (x) { x.setAttribute("aria-pressed", "false"); });
      b.setAttribute("aria-pressed", "true");
      setStageFromMedia({
        title: state.currentNode.titulo,
        meta: btn.is_live_generated ? "Generado en vivo" : "Botón legacy",
        mediaUrl: btn.media_url,
        cellCode: btn.cell_code,
        clipTitle: btn.label,
      });
      // Legacy flat: RecordBotonera with variant_id = id_btn
      await recordBotonera(btn.id_btn, "", btn.vector_delta, "BOTONERA_SCHEMA_FLAT");
    });
    list.appendChild(b);
  });
  mount.appendChild(list);
}

function renderSchemaTabs(options, kind, mount) {
  const tablist = document.createElement("div");
  tablist.className = "tablist";
  tablist.setAttribute("role", "tablist");
  tablist.setAttribute("aria-label", "Opciones de botonera");
  const tabs = [];

  options.forEach(function (opt, idx) {
    const tab = document.createElement("button");
    tab.type = "button";
    tab.setAttribute("role", "tab");
    tab.id = "tab-" + (opt.variant_id || idx);
    tab.setAttribute("aria-selected", "false");
    tab.setAttribute("tabindex", idx === 0 ? "0" : "-1");
    tab.textContent = opt.label || opt.variant_id;
    tab.addEventListener("click", async function () {
      activateTab(idx);
      setStageFromMedia({
        title: state.currentNode.titulo,
        meta: kind,
        mediaUrl: opt.media_url || opt.walkthrough_url,
        cellCode: opt.cell_code,
        hintText: opt.hint_text,
        clipTitle: opt.label,
      });
      await recordBotonera(opt.variant_id, "", opt.preference_delta, state.currentNode.botonera_schema.kind);
    });
    tab.addEventListener("keydown", function (ev) {
      const keys = { ArrowDown: 1, ArrowRight: 1, ArrowUp: -1, ArrowLeft: -1 };
      if (!(ev.key in keys)) return;
      ev.preventDefault();
      const next = (idx + keys[ev.key] + options.length) % options.length;
      tabs[next].focus();
      tabs[next].click();
    });
    tabs.push(tab);
    tablist.appendChild(tab);
  });

  function activateTab(i) {
    tabs.forEach(function (t, j) {
      t.setAttribute("aria-selected", j === i ? "true" : "false");
      t.setAttribute("tabindex", j === i ? "0" : "-1");
    });
    state.activeTabId = options[i] && options[i].variant_id;
  }

  mount.appendChild(tablist);
}

function renderCombinedMatrix(schema, mount) {
  const depths = schema.depth_axis || [];
  const formats = schema.format_axis || [];
  const cells = schema.matrix_cells || [];
  const byKey = {};
  cells.forEach(function (c) {
    byKey[c.depth_id + "|" + c.format_id] = c;
  });

  const grid = document.createElement("div");
  grid.className = "matrix";
  grid.style.gridTemplateColumns = "minmax(4.5rem,auto) repeat(" + formats.length + ", minmax(4.5rem,1fr))";
  grid.setAttribute("role", "grid");
  grid.setAttribute("aria-label", "Matriz profundidad × formato");

  const corner = document.createElement("div");
  corner.className = "axis-label";
  corner.textContent = "prof. \\ fmt";
  grid.appendChild(corner);
  formats.forEach(function (f) {
    const h = document.createElement("div");
    h.className = "axis-label";
    h.textContent = f;
    grid.appendChild(h);
  });

  depths.forEach(function (d) {
    const lab = document.createElement("div");
    lab.className = "axis-label";
    lab.textContent = d;
    grid.appendChild(lab);
    formats.forEach(function (f) {
      const cell = byKey[d + "|" + f];
      const btn = document.createElement("button");
      btn.type = "button";
      btn.setAttribute("role", "gridcell");
      if (!cell) {
        btn.disabled = true;
        btn.textContent = "—";
      } else {
        btn.textContent = f;
        btn.title = d + " × " + f;
        btn.addEventListener("click", async function () {
          grid.querySelectorAll("button").forEach(function (x) { x.setAttribute("aria-pressed", "false"); });
          btn.setAttribute("aria-pressed", "true");
          setStageFromMedia({
            title: state.currentNode.titulo,
            meta: "combined · " + d + " × " + f,
            mediaUrl: cell.media_url,
            cellCode: cell.cell_code,
            clipTitle: d + " / " + f,
          });
          await recordBotonera(d, f, cell.preference_delta || [], schema.kind);
        });
      }
      grid.appendChild(btn);
    });
  });
  mount.appendChild(grid);
}

function renderHierarchy(tree, mount) {
  const root = document.createElement("div");
  root.className = "accordion";
  root.setAttribute("role", "group");
  root.setAttribute("aria-label", tree.main_topic_title || "Subtemas");

  const summary = document.createElement("p");
  summary.className = "progress-summary";
  summary.setAttribute("role", "status");
  summary.setAttribute("aria-live", "polite");
  summary.setAttribute("aria-atomic", "true");
  const summaryText = document.createElement("span");
  const invitation = document.createElement("span");
  invitation.className = "progress-invitation";
  summary.appendChild(summaryText);
  summary.appendChild(invitation);
  mount.appendChild(summary);

  const macro = document.createElement("button");
  macro.type = "button";
  macro.className = "subtopic-select";
  macro.textContent = "Vista macro: " + (tree.main_topic_title || "tema");
  macro.addEventListener("click", function () {
    setStageFromMedia({
      title: state.currentNode.titulo,
      meta: "Jerarquía · macro",
      mediaUrl: tree.macro_media_url || state.currentNode.stage_media_default,
      clipTitle: tree.main_topic_title,
    });
  });
  mount.appendChild(macro);

  const nodesByID = {};

  function addNodes(nodes, parentEl, path) {
    (nodes || []).forEach(function (node) {
      nodesByID[node.subtopic_id] = node;
      const item = document.createElement("div");
      item.className = "accordion-item";
      const id = "acc-" + node.subtopic_id;
      const trigger = document.createElement("button");
      trigger.type = "button";
      trigger.className = "accordion-trigger";
      trigger.setAttribute("aria-expanded", "false");
      trigger.setAttribute("aria-controls", id);
      trigger.setAttribute("data-subtopic-id", node.subtopic_id);
      const label = document.createElement("span");
      label.className = "accordion-label";
      const title = document.createElement("span");
      title.textContent = node.title + (node.is_optional ? " (opcional)" : "");
      const progressState = document.createElement("span");
      progressState.className = "subtopic-state";
      label.appendChild(title);
      label.appendChild(progressState);
      const chevron = document.createElement("span");
      chevron.className = "chev";
      chevron.setAttribute("aria-hidden", "true");
      chevron.textContent = "▸";
      trigger.appendChild(label);
      trigger.appendChild(chevron);

      const panel = document.createElement("div");
      panel.className = "accordion-panel";
      panel.id = id;
      panel.hidden = true;

      const select = document.createElement("button");
      select.type = "button";
      select.className = "subtopic-select";
      select.textContent = "Abrir en Stage: " + node.title;
      select.addEventListener("click", async function () {
        const nextPath = path.concat(node.subtopic_id);
        setStageFromMedia({
          title: state.currentNode.titulo,
          meta: "Subtema · " + node.title,
          mediaUrl: node.media_url,
          clipTitle: node.title,
        });
        await recordSubtopic(node.subtopic_id, nextPath, node.orbit_delta);
      });
      panel.appendChild(select);

      trigger.addEventListener("click", function () {
        const open = trigger.getAttribute("aria-expanded") === "true";
        trigger.setAttribute("aria-expanded", open ? "false" : "true");
        panel.hidden = open;
        panel.classList.toggle("open", !open);
      });

      item.appendChild(trigger);
      item.appendChild(panel);
      parentEl.appendChild(item);

      if (node.child_subtopics && node.child_subtopics.length) {
        const childWrap = document.createElement("div");
        childWrap.style.marginLeft = "0.55rem";
        childWrap.style.marginTop = "0.35rem";
        panel.appendChild(childWrap);
        addNodes(node.child_subtopics, childWrap, path.concat(node.subtopic_id));
      }
    });
  }

  addNodes(tree.subtopics, root, []);
  mount.appendChild(root);
  state.hierarchyUI = {
    tree: tree,
    root: root,
    summary: summary,
    summaryText: summaryText,
    invitation: invitation,
    nodesByID: nodesByID,
  };
  updateHierarchyProgress();
}

export function renderRail(node) {
  el.railBody.innerHTML = "";
  state.hierarchyUI = null;
  el.railTopic.textContent = (node.botonera_schema && node.botonera_schema.topic_title) ||
    (node.hierarchy && node.hierarchy.main_topic_title) ||
    node.titulo ||
    "Botonera";

  const schema = node.botonera_schema;
  if (schema) {
    const kind = schemaKind(schema);
    if (kind === "depth") renderSchemaTabs(schema.depth_options || [], kind, el.railBody);
    else if (kind === "cognitive") renderSchemaTabs(schema.cognitive_options || [], kind, el.railBody);
    else if (kind === "emergency") renderSchemaTabs(schema.emergency_options || [], kind, el.railBody);
    else if (kind === "combined") renderCombinedMatrix(schema, el.railBody);
    else if (schema.flat_buttons && schema.flat_buttons.length) renderLegacyButtons(schema.flat_buttons, el.railBody);
  }

  if (node.hierarchy) {
    const sep = document.createElement("h3");
    sep.textContent = "Acordeón / subtemas";
    sep.style.marginTop = "0.5rem";
    el.railBody.appendChild(sep);
    renderHierarchy(node.hierarchy, el.railBody);
  }

  if (node.botonera && node.botonera.length) {
    const sep = document.createElement("h3");
    sep.textContent = "Acciones";
    sep.style.marginTop = "0.5rem";
    el.railBody.appendChild(sep);
    renderLegacyButtons(node.botonera, el.railBody);
  }

  setAskMode(true);
}
