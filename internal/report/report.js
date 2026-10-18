(() => {
  const data = JSON.parse(document.getElementById("ebb-data").textContent);
  const root = data.tree;
  const box = document.getElementById("map"), canvas = box.querySelector("canvas"), tip = document.getElementById("tip");
  if (!root || !root.children) { box.style.display = "none"; return; }
  const css = (v) => getComputedStyle(document.documentElement).getPropertyValue(v).trim();
  const fmt = (n) => { const u = ["B", "kB", "MB", "GB", "TB"]; let i = 0; while (n >= 1000 && i < u.length - 1) { n /= 1000; i++; } return (i ? n.toFixed(1).replace(/\.0$/, "") : n) + " " + u[i]; };
  // A folder that is almost entirely one subfolder says nothing on its own:
  // show "Users/me" and that subfolder's contents instead.
  const collapse = (n) => {
    let name = n.name, node = n;
    while (node.children && node.children.length) {
      const big = node.children.reduce((a, b) => (b.alloc > a.alloc ? b : a));
      if (big.kind !== 0 || big.alloc < 0.85 * node.alloc) break;
      name += "/" + big.name; node = big;
    }
    return { ...node, name, alloc: n.alloc, files: n.files };
  };
  const tops = root.children.filter((c) => c.alloc > 0).map(collapse);
  const slot = (i, node) => node.kind === 3 || i >= 8 ? "--other" : "--s" + (i + 1);

  // Squarified treemap: lay rows along the shorter side, keeping aspect ratios near 1.
  function squarify(items, x, y, w, h, out) {
    const total = items.reduce((s, c) => s + c.alloc, 0);
    if (!total || w <= 0 || h <= 0) return;
    const scale = (w * h) / total;
    let rest = items.slice(), rx = x, ry = y, rw = w, rh = h;
    while (rest.length) {
      const side = Math.min(rw, rh);
      let row = [], best = Infinity;
      for (const it of rest) {
        const trial = row.concat(it), sum = trial.reduce((s, c) => s + c.alloc * scale, 0);
        const worst = Math.max(...trial.map((c) => { const a = c.alloc * scale, l = sum / side; return Math.max(l * l / a, a / (l * l)); }));
        if (worst > best) break;
        best = worst; row = trial;
      }
      const sum = row.reduce((s, c) => s + c.alloc * scale, 0), thick = sum / side;
      let off = 0;
      for (const c of row) {
        const len = (c.alloc * scale) / thick;
        if (rw >= rh) out.push({ node: c, x: rx, y: ry + off, w: thick, h: len });
        else out.push({ node: c, x: rx + off, y: ry, w: len, h: thick });
        off += len;
      }
      if (rw >= rh) { rx += thick; rw -= thick; } else { ry += thick; rh -= thick; }
      rest = rest.slice(row.length);
    }
  }

  let shapes = [];
  function draw() {
    const dpr = window.devicePixelRatio || 1, W = box.clientWidth, H = box.clientHeight;
    canvas.width = W * dpr; canvas.height = H * dpr;
    const ctx = canvas.getContext("2d");
    ctx.scale(dpr, dpr);
    ctx.fillStyle = css("--surface-1"); ctx.fillRect(0, 0, W, H);
    const top = [];
    squarify(tops, 0, 0, W, H, top);
    shapes = [];
    top.forEach((r, i) => {
      const color = css(slot(tops.indexOf(r.node), r.node));
      ctx.fillStyle = color; ctx.globalAlpha = 1;
      ctx.beginPath(); ctx.roundRect(r.x + 1, r.y + 1, Math.max(0, r.w - 2), Math.max(0, r.h - 2), 4); ctx.fill();
      shapes.push({ ...r, path: r.node.name, depth: 1 });
      const kids = (r.node.children || []).filter((c) => c.alloc > 0);
      if (kids.length && r.w > 60 && r.h > 50) {
        const inner = [];
        squarify(kids, r.x + 4, r.y + 22, r.w - 8, r.h - 26, inner);
        for (const k of inner) {
          ctx.globalAlpha = 0.35; ctx.fillStyle = css("--surface-1");
          ctx.beginPath(); ctx.roundRect(k.x + 1, k.y + 1, Math.max(0, k.w - 2), Math.max(0, k.h - 2), 3); ctx.fill();
          shapes.push({ ...k, path: r.node.name + "/" + k.node.name, depth: 2 });
          if (k.w > 70 && k.h > 18) { ctx.globalAlpha = 1; ctx.fillStyle = css("--text-primary"); ctx.font = "11px -apple-system, sans-serif"; ctx.fillText(k.node.name.slice(0, Math.floor(k.w / 7)), k.x + 6, k.y + 14, k.w - 10); }
        }
      }
      ctx.globalAlpha = 1;
      if (r.w > 60 && r.h > 18) { ctx.fillStyle = "#fff"; ctx.font = "600 12px -apple-system, sans-serif"; ctx.fillText(r.node.name.slice(0, Math.floor(r.w / 7.5)), r.x + 8, r.y + 16, r.w - 14); }
    });
    const legend = document.getElementById("legend");
    legend.textContent = "";
    tops.slice(0, 9).forEach((n, i) => {
      const s = document.createElement("span"), sw = document.createElement("i");
      sw.style.background = css(slot(i, n)); s.append(sw, `${n.name} · ${fmt(n.alloc)}`); legend.append(s);
    });
  }
  canvas.addEventListener("mousemove", (e) => {
    const b = canvas.getBoundingClientRect(), x = e.clientX - b.left, y = e.clientY - b.top;
    const hit = shapes.filter((s) => x >= s.x && x <= s.x + s.w && y >= s.y && y <= s.y + s.h).pop();
    if (!hit) { tip.style.display = "none"; return; }
    tip.textContent = `${hit.path} — ${fmt(hit.node.alloc)} · ${(100 * hit.node.alloc / root.alloc).toFixed(1)}% · ${hit.node.files} files`;
    tip.style.display = "block";
    tip.style.left = Math.min(x + 14, b.width - tip.offsetWidth - 8) + "px";
    tip.style.top = Math.min(y + 14, b.height - tip.offsetHeight - 8) + "px";
  });
  canvas.addEventListener("mouseleave", () => (tip.style.display = "none"));
  addEventListener("resize", draw);
  matchMedia("(prefers-color-scheme: dark)").addEventListener("change", draw);
  draw();
})();
