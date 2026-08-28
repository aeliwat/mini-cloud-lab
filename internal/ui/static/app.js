(() => {
  const LAYOUT_KEY = "minicloud-layout-v1";
  const TICK_MS = 650; // real ms per simulated second (snappy live feel)

  const els = {
    nodes: document.getElementById("nodes"),
    empty: document.getElementById("empty"),
    path: document.getElementById("state-path"),
    usersLabel: document.getElementById("users-label"),
    report: document.getElementById("report"),
    reportStats: document.getElementById("report-stats"),
    reportBars: document.getElementById("report-bars"),
    error: document.getElementById("error"),
    stage: document.getElementById("stage"),
    wires: document.getElementById("wires"),
    canvas: document.getElementById("particles"),
    btnLoad: document.getElementById("btn-load"),
    btnRefresh: document.getElementById("btn-refresh"),
    inputUsers: document.getElementById("input-users"),
    inputRps: document.getElementById("input-rps"),
    inputDuration: document.getElementById("input-duration"),
    clients: document.getElementById("clients"),
    tickClock: document.getElementById("tick-clock"),
    scenarioList: document.getElementById("scenario-list"),
    liveStrip: document.getElementById("live-strip"),
    liveSecond: document.getElementById("live-second"),
    liveOk: document.getElementById("live-ok"),
    liveFail: document.getElementById("live-fail"),
    liveCumOk: document.getElementById("live-cum-ok"),
  };

  let servers = [];
  let simulating = false;
  let anim = 0;
  let tickTimer = 0;
  let drag = null;
  let loadState = {};

  function showError(msg) {
    els.error.textContent = msg || "";
    els.error.classList.toggle("hidden", !msg);
  }

  function loadLayout() {
    try {
      return JSON.parse(localStorage.getItem(LAYOUT_KEY) || "{}");
    } catch {
      return {};
    }
  }

  function saveLayout(layout) {
    localStorage.setItem(LAYOUT_KEY, JSON.stringify(layout));
  }

  function defaultPos(role, index, count) {
    const xMap = { lb: 0.34, web: 0.58, db: 0.84 };
    return {
      x: (xMap[role] || 0.5) * 100,
      y: ((index + 1) / (count + 1)) * 100,
    };
  }

  function stageRect() {
    return els.stage.getBoundingClientRect();
  }

  function clientCenter() {
    const c = els.clients.querySelector(".clients-node").getBoundingClientRect();
    const s = stageRect();
    return {
      x: c.left + c.width / 2 - s.left,
      y: c.top + c.height / 2 - s.top,
      el: els.clients,
    };
  }

  function clientAnchorToward(target) {
    const c = els.clients.querySelector(".clients-node").getBoundingClientRect();
    const s = stageRect();
    const cx = c.left + c.width / 2 - s.left;
    const cy = c.top + c.height / 2 - s.top;
    const hw = c.width / 2;
    const hh = c.height / 2;
    return boxEdgePoint(cx, cy, hw, hh, target.x, target.y);
  }

  function nodeMeta(id) {
    const el = els.nodes.querySelector(`[data-id="${id}"]`);
    if (!el) return null;
    const n = el.getBoundingClientRect();
    const s = stageRect();
    return {
      el,
      cx: n.left + n.width / 2 - s.left,
      cy: n.top + n.height / 2 - s.top,
      hw: n.width / 2,
      hh: n.height / 2,
      isLB: el.classList.contains("lb"),
    };
  }

  // Point on a rectangle edge along the ray from center → target.
  function boxEdgePoint(cx, cy, hw, hh, tx, ty) {
    const dx = tx - cx;
    const dy = ty - cy;
    if (Math.abs(dx) < 0.001 && Math.abs(dy) < 0.001) {
      return { x: cx, y: cy };
    }
    const sx = Math.abs(dx) < 0.001 ? Infinity : hw / Math.abs(dx);
    const sy = Math.abs(dy) < 0.001 ? Infinity : hh / Math.abs(dy);
    const t = Math.min(sx, sy);
    return { x: cx + dx * t, y: cy + dy * t };
  }

  // Point on a diamond edge (LB) along the ray from center → target.
  function diamondEdgePoint(cx, cy, radius, tx, ty) {
    const dx = tx - cx;
    const dy = ty - cy;
    if (Math.abs(dx) < 0.001 && Math.abs(dy) < 0.001) {
      return { x: cx, y: cy };
    }
    // Diamond: |x|/r + |y|/r = 1
    const t = radius / (Math.abs(dx) + Math.abs(dy));
    return { x: cx + dx * t, y: cy + dy * t };
  }

  function nodeAnchorToward(id, target) {
    const m = nodeMeta(id);
    if (!m || !target) return null;
    if (m.isLB) {
      // Visual diamond is ~71% of the hit-box (112 / 158).
      const r = Math.min(m.hw, m.hh) * 0.71;
      return diamondEdgePoint(m.cx, m.cy, r, target.x, target.y);
    }
    return boxEdgePoint(m.cx, m.cy, m.hw - 1, m.hh - 1, target.x, target.y);
  }

  function curvePoints(a, b) {
    const dx = b.x - a.x;
    const dy = b.y - a.y;
    const dist = Math.hypot(dx, dy) || 1;
    // Pull control points along the chord so curves stay smooth after free drag.
    const pull = Math.min(90, Math.max(36, dist * 0.4));
    const nx = dx / dist;
    const ny = dy / dist;
    return {
      p0: a,
      p1: { x: a.x + nx * pull, y: a.y + ny * pull },
      p2: { x: b.x - nx * pull, y: b.y - ny * pull },
      p3: b,
    };
  }

  function cubic(p0, p1, p2, p3, t) {
    const u = 1 - t;
    return {
      x: u * u * u * p0.x + 3 * u * u * t * p1.x + 3 * u * t * t * p2.x + t * t * t * p3.x,
      y: u * u * u * p0.y + 3 * u * u * t * p1.y + 3 * u * t * t * p2.y + t * t * t * p3.y,
    };
  }

  async function fetchServers() {
    if (simulating) return;
    showError("");
    const res = await fetch("/api/servers");
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "failed to load servers");
    servers = data.servers || [];
    els.path.textContent = data.path || "";
    renderNodes();
    drawWires();
    applyLoadVisuals(1);
  }

  async function loadScenarios() {
    const res = await fetch("/api/scenarios");
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "failed to load scenarios");
    els.scenarioList.innerHTML = "";
    for (const sc of data.scenarios || []) {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "scenario-btn";
      btn.innerHTML = `<strong>${sc.name}</strong><span>${sc.description}</span>`;
      btn.addEventListener("click", () => applyScenario(sc.id));
      els.scenarioList.appendChild(btn);
    }
  }

  async function removeServer(id) {
    if (simulating) return;
    showError("");
    const res = await fetch(`/api/servers/${encodeURIComponent(id)}`, { method: "DELETE" });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "failed to remove server");
    const layout = loadLayout();
    delete layout[id];
    saveLayout(layout);
    servers = data.servers || [];
    clearLoadVisuals();
    renderNodes();
    drawWires();
    applyLoadVisuals(1);
  }

  async function applyScenario(id) {
    if (simulating) return;
    showError("");
    clearLoadVisuals();
    localStorage.removeItem(LAYOUT_KEY);
    const res = await fetch(`/api/scenarios/${id}`, { method: "POST" });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "scenario failed");
    els.inputUsers.value = data.users;
    els.inputRps.value = data.rps;
    els.inputDuration.value = data.duration;
    servers = data.servers || [];
    els.path.textContent = "scenario: " + data.name;
    renderNodes();
    drawWires();
  }

  function renderNodes() {
    const layout = loadLayout();
    els.nodes.innerHTML = "";
    els.empty.classList.toggle("hidden", servers.length > 0);

    const byRole = {
      lb: servers.filter((s) => s.role === "lb"),
      web: servers.filter((s) => s.role === "web"),
      db: servers.filter((s) => s.role === "db"),
    };

    for (const role of ["lb", "web", "db"]) {
      byRole[role].forEach((srv, i) => {
        const pos = layout[srv.id] || defaultPos(role, i, byRole[role].length);
        const el = document.createElement("div");
        el.className = `node server-node ${srv.role}${srv.status === "stopped" ? " stopped" : ""}${srv.healthy === false ? " unhealthy" : ""}`;
        el.dataset.id = srv.id;
        el.dataset.role = srv.role;
        el.style.left = `${pos.x}%`;
        el.style.top = `${pos.y}%`;

        const cap = srv.capacity_rps ? ` · ${srv.capacity_rps} rps` : "";
        if (srv.role === "lb") {
          el.innerHTML = `
            <button type="button" class="node-remove" title="Remove server" aria-label="Remove ${srv.id}">×</button>
            <div class="lb-diamond" aria-hidden="true"></div>
            <div class="lb-face">
              <span class="node-kicker">LB</span>
              <strong>${srv.id}</strong>
              <span class="mono">${srv.ram}</span>
              <span class="mono cap-line">${srv.status}${cap}</span>
              <span class="load-badge" data-badge="${srv.id}">idle</span>
              <div class="meter"><span data-meter="${srv.id}"></span></div>
            </div>`;
        } else {
          el.innerHTML = `
            <button type="button" class="node-remove" title="Remove server" aria-label="Remove ${srv.id}">×</button>
            <div class="node-head">
              <span class="node-kicker">${srv.role}</span>
              <span class="load-badge" data-badge="${srv.id}">idle</span>
            </div>
            <strong>${srv.id}</strong>
            <span class="mono">${srv.ram} · ${srv.disk}</span>
            <span class="mono cap-line">${srv.status}${srv.capacity_rps ? ` · ${srv.capacity_rps} rps cap` : ""}</span>
            <div class="meter"><span data-meter="${srv.id}"></span></div>`;
        }
        el.querySelector(".node-remove").addEventListener("click", (e) => {
          e.stopPropagation();
          removeServer(srv.id).catch((err) => showError(err.message));
        });
        enableDrag(el);
        els.nodes.appendChild(el);
      });
    }
  }

  function enableDrag(el) {
    el.addEventListener("pointerdown", (e) => {
      if (simulating) return;
      if (e.button !== 0) return;
      if (e.target.closest(".node-remove")) return;
      e.preventDefault();
      el.setPointerCapture(e.pointerId);
      const s = stageRect();
      const r = el.getBoundingClientRect();
      const centerX = r.left + r.width / 2;
      const centerY = r.top + r.height / 2;
      drag = {
        id: el.dataset.id,
        el,
        // Offset from node center to pointer (keeps grab point stable).
        grabX: e.clientX - centerX,
        grabY: e.clientY - centerY,
        sw: s.width,
        sh: s.height,
        sl: s.left,
        st: s.top,
      };
      el.classList.add("dragging");
    });

    el.addEventListener("pointermove", (e) => {
      if (!drag || drag.el !== el) return;
      const centerX = e.clientX - drag.grabX;
      const centerY = e.clientY - drag.grabY;
      const cx = ((centerX - drag.sl) / drag.sw) * 100;
      const cy = ((centerY - drag.st) / drag.sh) * 100;
      el.style.left = `${Math.min(92, Math.max(8, cx))}%`;
      el.style.top = `${Math.min(90, Math.max(10, cy))}%`;
      drawWires();
    });

    el.addEventListener("pointerup", () => {
      if (!drag || drag.el !== el) return;
      el.classList.remove("dragging");
      const layout = loadLayout();
      layout[drag.id] = {
        x: parseFloat(el.style.left),
        y: parseFloat(el.style.top),
      };
      saveLayout(layout);
      drag = null;
      drawWires();
    });
  }

  function wireDot(p, color) {
    const c = document.createElementNS("http://www.w3.org/2000/svg", "circle");
    c.setAttribute("cx", String(p.x));
    c.setAttribute("cy", String(p.y));
    c.setAttribute("r", "3.2");
    c.setAttribute("fill", color);
    c.setAttribute("opacity", "0.9");
    c.classList.add("wire-dot");
    els.wires.appendChild(c);
  }

  function wirePath(a, b, color, width = 2) {
    const { p0, p1, p2, p3 } = curvePoints(a, b);
    const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
    path.setAttribute("d", `M ${p0.x} ${p0.y} C ${p1.x} ${p1.y}, ${p2.x} ${p2.y}, ${p3.x} ${p3.y}`);
    path.setAttribute("fill", "none");
    path.setAttribute("stroke", color);
    path.setAttribute("stroke-width", String(width));
    path.setAttribute("stroke-linecap", "round");
    path.setAttribute("stroke-linejoin", "round");
    path.setAttribute("stroke-dasharray", "6 8");
    path.classList.add("flow-wire");
    els.wires.appendChild(path);
    wireDot(a, color);
    wireDot(b, color);
  }

  function connect(fromIdOrClient, toId, color, width) {
    let a;
    let b;
    if (fromIdOrClient === "clients") {
      const meta = nodeMeta(toId);
      if (!meta) return;
      const toward = { x: meta.cx, y: meta.cy };
      a = clientAnchorToward(toward);
      b = nodeAnchorToward(toId, a);
    } else {
      const fromMeta = nodeMeta(fromIdOrClient);
      const toMeta = nodeMeta(toId);
      if (!fromMeta || !toMeta) return;
      // Two-pass so both ends sit on the true silhouette edge.
      a = nodeAnchorToward(fromIdOrClient, { x: toMeta.cx, y: toMeta.cy });
      b = nodeAnchorToward(toId, a);
      a = nodeAnchorToward(fromIdOrClient, b);
    }
    if (a && b) wirePath(a, b, color, width);
  }

  function drawWires() {
    const s = stageRect();
    els.wires.setAttribute("width", String(s.width));
    els.wires.setAttribute("height", String(s.height));
    els.wires.setAttribute("viewBox", `0 0 ${s.width} ${s.height}`);
    els.wires.innerHTML = "";

    const lb = servers.filter((srv) => srv.role === "lb" && srv.status === "running" && srv.healthy !== false);
    const web = servers.filter((srv) => srv.role === "web" && srv.status === "running" && srv.healthy !== false);
    const db = servers.filter((srv) => srv.role === "db");

    if (lb.length > 0) {
      connect("clients", lb[0].id, "rgba(255,179,71,0.75)", 2.4);
      for (const srv of web) {
        connect(lb[0].id, srv.id, "rgba(92,225,255,0.65)", 2);
      }
    } else {
      for (const srv of web) {
        connect("clients", srv.id, "rgba(57,255,138,0.65)", 2);
      }
    }

    web.forEach((w, wi) => {
      db.forEach((d, di) => {
        if (db.length > 1 && wi % db.length !== di) return;
        connect(w.id, d.id, "rgba(199,146,234,0.5)", 1.6);
      });
    });
  }

  function resizeCanvas() {
    const rect = stageRect();
    els.canvas.width = rect.width * devicePixelRatio;
    els.canvas.height = rect.height * devicePixelRatio;
    els.canvas.style.width = `${rect.width}px`;
    els.canvas.style.height = `${rect.height}px`;
    const ctx = els.canvas.getContext("2d");
    ctx.setTransform(devicePixelRatio, 0, 0, devicePixelRatio, 0, 0);
    return ctx;
  }

  function clearLoadVisuals() {
    loadState = {};
    document.querySelectorAll(".server-node").forEach((n) => {
      n.classList.remove("overloaded", "warm", "hot");
    });
    document.querySelectorAll("[data-meter]").forEach((m) => {
      m.style.width = "0%";
      m.classList.remove("hot", "critical");
    });
    document.querySelectorAll("[data-badge]").forEach((b) => {
      b.textContent = "idle";
      b.className = "load-badge";
    });
  }

  function applyLoadVisuals() {
    for (const [id, st] of Object.entries(loadState)) {
      const loadPct = Math.min(100, st.loadPct);
      const overloaded = st.loadPct >= 100 || st.failed > 0;
      const node = document.querySelector(`[data-id="${id}"]`);
      const meter = document.querySelector(`[data-meter="${id}"]`);
      const badge = document.querySelector(`[data-badge="${id}"]`);
      if (!node) continue;

      node.classList.toggle("warm", !overloaded && st.loadPct >= 70);
      node.classList.toggle("hot", !overloaded && st.loadPct >= 90);
      node.classList.toggle("overloaded", overloaded);

      if (meter) {
        meter.style.width = `${loadPct}%`;
        meter.classList.toggle("hot", st.loadPct >= 70 && !overloaded);
        meter.classList.toggle("critical", overloaded);
      }
      if (badge) {
        if (overloaded) {
          badge.textContent = "OVERLOAD";
          badge.className = "load-badge badge-red";
        } else {
          badge.textContent = `${Math.round(loadPct)}%`;
          badge.className = "load-badge " + (st.loadPct >= 70 ? "badge-amber" : "badge-ok");
        }
      }
    }
  }

  function pickWeighted(items, weightFn) {
    const weights = items.map(weightFn);
    const sum = weights.reduce((a, b) => a + b, 0) || 1;
    let r = Math.random() * sum;
    for (let i = 0; i < items.length; i++) {
      r -= weights[i];
      if (r <= 0) return items[i];
    }
    return items[items.length - 1];
  }

  function spawnTickParticles(tick, viaLB) {
    cancelAnimationFrame(anim);
    const ctx = resizeCanvas();
    const lbNode = (tick.nodes || []).find((n) => n.role === "lb");
    const lbMeta = lbNode ? nodeMeta(lbNode.id) : null;
    const useLB = viaLB && lbMeta;

    const webNodes = (tick.nodes || [])
      .filter((n) => n.role === "web")
      .map((n) => {
        const meta = nodeMeta(n.id);
        return meta ? { ...n, meta } : null;
      })
      .filter(Boolean);

    const lbFailRate = lbNode && lbNode.success + lbNode.failed
      ? lbNode.failed / (lbNode.success + lbNode.failed)
      : 0;

    const count = Math.min(90, 24 + Math.floor(((tick.success + tick.failed) || 0) / 80));
    const particles = [];
    for (let i = 0; i < count; i++) {
      const failAtLB = useLB && Math.random() < lbFailRate;
      let web = null;
      let failAtWeb = false;
      if (!failAtLB && webNodes.length) {
        web = pickWeighted(webNodes, (w) => Math.max(1, w.target_rps));
        const total = web.success + web.failed;
        failAtWeb = total ? Math.random() < web.failed / total : false;
      }

      const segments = [];
      if (useLB) {
        const towardLB = { x: lbMeta.cx, y: lbMeta.cy };
        const from = clientAnchorToward(towardLB);
        const lbIn = nodeAnchorToward(lbNode.id, from);
        segments.push(curvePoints(from, lbIn));
        if (!failAtLB && web) {
          const lbOut = nodeAnchorToward(lbNode.id, { x: web.meta.cx, y: web.meta.cy });
          const webIn = nodeAnchorToward(web.id, lbOut);
          segments.push(curvePoints(lbOut, webIn));
        }
      } else if (web) {
        const toward = { x: web.meta.cx, y: web.meta.cy };
        const from = clientAnchorToward(toward);
        const webIn = nodeAnchorToward(web.id, from);
        segments.push(curvePoints(from, webIn));
      } else continue;

      particles.push({
        segments,
        failAtLB,
        failAtWeb,
        t: Math.random() * 0.2,
        speed: 0.01 + Math.random() * 0.012,
        trail: [],
      });
    }

    const started = performance.now();
    const life = Math.min(TICK_MS - 40, 580);

    function draw(p, point, failed) {
      const g = ctx.createRadialGradient(point.x, point.y, 0, point.x, point.y, 11);
      g.addColorStop(0, failed ? "rgba(255,92,92,0.95)" : "rgba(57,255,138,0.95)");
      g.addColorStop(1, "transparent");
      ctx.beginPath();
      ctx.fillStyle = g;
      ctx.arc(point.x, point.y, 11, 0, Math.PI * 2);
      ctx.fill();
      ctx.beginPath();
      ctx.fillStyle = failed ? "#ff8a8a" : "#d8ffe6";
      ctx.arc(point.x, point.y, failed ? 2.1 : 2.6, 0, Math.PI * 2);
      ctx.fill();
      if (p.trail.length > 1) {
        ctx.beginPath();
        ctx.strokeStyle = failed ? "rgba(255,92,92,0.35)" : "rgba(57,255,138,0.35)";
        ctx.lineWidth = 1.4;
        ctx.moveTo(p.trail[0].x, p.trail[0].y);
        for (let i = 1; i < p.trail.length; i++) ctx.lineTo(p.trail[i].x, p.trail[i].y);
        ctx.stroke();
      }
    }

    function frame(now) {
      const elapsed = now - started;
      ctx.clearRect(0, 0, els.canvas.width, els.canvas.height);
      for (const p of particles) {
        p.t += p.speed;
        if (p.t > 1) {
          p.t = 0;
          p.trail = [];
        }
        const segs = p.segments;
        const local = p.t * segs.length;
        const idx = Math.min(segs.length - 1, Math.floor(local));
        const st = local - idx;
        const seg = segs[idx];
        const point = cubic(seg.p0, seg.p1, seg.p2, seg.p3, st);
        let failed = false;
        if (p.failAtLB && idx === 0 && st > 0.9) {
          failed = true;
          point.y += (st - 0.9) * 70;
        }
        if (p.failAtWeb && idx === segs.length - 1 && st > 0.86) {
          failed = true;
          point.y += (st - 0.86) * 60;
        }
        p.trail.push({ x: point.x, y: point.y });
        if (p.trail.length > 7) p.trail.shift();
        draw(p, point, failed);
      }
      if (elapsed < life) anim = requestAnimationFrame(frame);
      else ctx.clearRect(0, 0, els.canvas.width, els.canvas.height);
    }
    anim = requestAnimationFrame(frame);
  }

  function updateLiveStrip(tick, cumOk, totalSeconds) {
    els.liveStrip.classList.remove("hidden");
    els.liveSecond.textContent = `${tick.second} / ${totalSeconds}`;
    els.liveOk.textContent = String(tick.success);
    els.liveFail.textContent = String(tick.failed);
    els.liveCumOk.textContent = String(cumOk);
    els.tickClock.textContent = `t = ${tick.second}s / ${totalSeconds}s`;
  }

  function playTicks(result) {
    return new Promise((resolve) => {
      const ticks = result.ticks || [];
      if (!ticks.length) {
        resolve();
        return;
      }
      let i = 0;
      let cumOk = 0;
      document.body.classList.add("simulating");
      document.querySelectorAll(".flow-wire").forEach((w) => w.classList.add("active"));

      const step = () => {
        if (i >= ticks.length) {
          document.body.classList.remove("simulating");
          document.querySelectorAll(".flow-wire").forEach((w) => w.classList.remove("active"));
          els.tickClock.textContent = `t = ${ticks.length}s · done`;
          resolve();
          return;
        }
        const tick = ticks[i];
        cumOk += tick.success;
        updateLiveStrip(tick, cumOk, ticks.length);

        // Update per-node load from this second's target vs capacity.
        for (const n of tick.nodes || []) {
          const loadPct = (n.target_rps / Math.max(n.capacity, 1)) * 100;
          const prev = loadState[n.id] || { failed: 0 };
          loadState[n.id] = {
            loadPct,
            failed: prev.failed + n.failed,
            ok: (prev.ok || 0) + n.success,
            capacity: n.capacity,
            target: n.target_rps,
          };
        }
        applyLoadVisuals();
        spawnTickParticles(tick, result.via_lb);
        i += 1;
        tickTimer = setTimeout(step, TICK_MS);
      };
      step();
    });
  }

  function renderReport(result) {
    els.report.classList.remove("hidden");
    const okRate = result.total_requests
      ? ((result.success / result.total_requests) * 100).toFixed(1)
      : "0.0";
    els.reportStats.innerHTML = `
      <dt>Users</dt><dd>${result.users || 0}</dd>
      <dt>Via LB</dt><dd>${result.via_lb ? "yes" : "no"}</dd>
      <dt>Target RPS</dt><dd>${result.target_rps}</dd>
      <dt>Duration</dt><dd>${result.duration}</dd>
      <dt>Succeeded</dt><dd>${result.success}</dd>
      <dt>Failed</dt><dd>${result.failed}</dd>
      <dt>Success rate</dt><dd>${okRate}%</dd>
      <dt>Latency avg</dt><dd>${result.latency ? result.latency.avg_ms + " ms" : "—"}</dd>
      <dt>Latency p50</dt><dd>${result.latency ? result.latency.p50_ms + " ms" : "—"}</dd>
      <dt>Latency p95</dt><dd>${result.latency ? result.latency.p95_ms + " ms" : "—"}</dd>
    `;
    els.reportBars.innerHTML = "";
    for (const st of result.servers || []) {
      const total = st.success + st.failed || 1;
      const okPct = (st.success / total) * 100;
      const failPct = (st.failed / total) * 100;
      const loadPct = (st.target_rps / Math.max(st.capacity, 1)) * 100;
      const overloaded = loadPct >= 100 || st.failed > 0;
      const health = st.healthy === false ? " · UNHEALTHY" : "";
      const row = document.createElement("div");
      row.className = "bar-row" + (overloaded || st.healthy === false ? " row-overloaded" : "");
      row.innerHTML = `
        <div class="label">
          <span>${st.role} · ${st.id}</span>
          <span>${st.target_rps}/${st.capacity} rps${overloaded ? " · OVERLOAD" : ""}${health}</span>
        </div>
        <div class="bar-track">
          <div class="bar-ok" style="width:${okPct}%"></div>
          <div class="bar-fail" style="width:${failPct}%"></div>
        </div>`;
      els.reportBars.appendChild(row);
    }
  }

  async function runLoad() {
    showError("");
    clearLoadVisuals();
    els.report.classList.add("hidden");
    els.btnLoad.disabled = true;
    simulating = true;
    els.usersLabel.textContent = `${Number(els.inputUsers.value || 0).toLocaleString()} online`;
    try {
      const res = await fetch("/api/load", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          rps: Number(els.inputRps.value || 0),
          duration: els.inputDuration.value || "10s",
          users: Number(els.inputUsers.value || 0),
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "load failed");
      await playTicks(data);
      renderReport(data);
      // Final aggregate overload state
      for (const st of data.servers || []) {
        loadState[st.id] = {
          loadPct: (st.target_rps / Math.max(st.capacity, 1)) * 100,
          failed: st.failed,
          ok: st.success,
          capacity: st.capacity,
          target: st.target_rps,
        };
      }
      applyLoadVisuals();
      // Refresh topology so UNHEALTHY / DOWN badges appear
      simulating = false;
      await fetchServers();
      simulating = true;
    } catch (err) {
      showError(err.message);
      document.body.classList.remove("simulating");
    } finally {
      simulating = false;
      els.btnLoad.disabled = false;
      clearTimeout(tickTimer);
    }
  }

  els.btnLoad.addEventListener("click", runLoad);
  els.btnRefresh.addEventListener("click", () => {
    clearLoadVisuals();
    fetchServers().catch((e) => showError(e.message));
  });
  window.addEventListener("resize", () => {
    resizeCanvas();
    drawWires();
  });

  loadScenarios().catch((e) => showError(e.message));
  fetchServers().catch((e) => showError(e.message));
  setInterval(() => fetchServers().catch(() => {}), 6000);
})();
