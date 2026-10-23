(() => {
  "use strict";
  const root = document.documentElement;
  const icons = {
    system: '<rect x="3" y="4" width="18" height="13" rx="2"/><path d="M8 21h8m-4-4v4"/>',
    light: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5"/>',
    dark: '<path d="M20.5 14a8.5 8.5 0 0 1-10.5-10.5A8.5 8.5 0 1 0 20.5 14Z"/>',
    wrap: '<path d="M3 5h18M3 10h13a4 4 0 0 1 0 8h-4m3-3-3 3 3 3M3 15h4M3 20h4"/>',
    copy: '<rect x="8" y="3" width="12" height="14" rx="2"/><path d="M16 17v3a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V8a1 1 0 0 1 1-1h3"/>'
  };
  const icon = name => `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${icons[name]}</svg>`;
  const diagrams = [...document.querySelectorAll('pre.mermaid')].map(source => {
    const output = document.createElement('div');
    output.className = 'mermaid-diagram';
    output.hidden = true;
    source.after(output);
    return {source, output, text:source.textContent};
  });
  let drawing = Promise.resolve(), diagramID = 0;
  const renderDiagrams = () => {
    drawing = drawing.then(async () => {
      if (!window.mermaid || !diagrams.length) return;
      window.mermaid.initialize({startOnLoad:false, securityLevel:'strict', suppressErrorRendering:true, theme:root.dataset.theme === 'dark' ? 'dark' : 'default', htmlLabels:false, fontFamily:'system-ui'});
      for (const diagram of diagrams) {
        try {
          const result = await window.mermaid.render(`blogo-diagram-${++diagramID}`, diagram.text);
          diagram.output.innerHTML = result.svg;
          diagram.output.hidden = false;
          diagram.source.hidden = true;
        } catch (_) {
          diagram.output.hidden = true;
          diagram.source.hidden = false;
        }
      }
    });
  };
  let preference;
  try { preference = localStorage.getItem("blogo-theme"); } catch (_) {}
  if (!['system','light','dark'].includes(preference)) preference = 'system';
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  const toggle = document.querySelector(".theme-toggle");
  const applyTheme = () => {
    root.dataset.theme = preference === 'system' ? (media.matches ? 'dark' : 'light') : preference;
    if (toggle) {
      const name = {system:'系统',light:'浅色',dark:'深色'}[preference];
      toggle.dataset.mode = preference;
      toggle.dataset.tooltip = name;
      toggle.setAttribute('aria-label', `${name}，点击切换主题模式`);
      toggle.innerHTML = icon(preference);
    }
    renderDiagrams();
  };
  applyTheme();
  if (toggle) {
    toggle.hidden = false;
    toggle.addEventListener("click", () => {
      preference = {system:'light',light:'dark',dark:'system'}[preference];
      applyTheme();
      try { localStorage.setItem("blogo-theme", preference); } catch (_) {}
    });
    media.addEventListener("change", () => { if (preference === 'system') applyTheme(); });
  }
  document.querySelectorAll('.prose pre:not(.mermaid)').forEach(pre => {
    const source = pre.textContent;
    const frame = document.createElement('div'); frame.className = 'code-block';
    pre.before(frame); frame.append(pre);
    const tools = document.createElement('div'); tools.className = 'code-tools';
    const button = (name,label) => {
      const node = document.createElement('button');
      node.type = 'button'; node.className = 'icon-button';
      node.setAttribute('aria-label',label); node.dataset.tooltip = label; node.innerHTML = icon(name);
      tools.append(node); return node;
    };
    const wrap = button('wrap','开启自动换行');
    wrap.setAttribute('aria-pressed','false');
    wrap.addEventListener('click', () => {
      const enabled = frame.classList.toggle('is-wrapped');
      wrap.setAttribute('aria-pressed',String(enabled));
      wrap.dataset.tooltip = enabled ? '关闭自动换行' : '开启自动换行';
      wrap.setAttribute('aria-label',wrap.dataset.tooltip);
    });
    const copy = button('copy','复制代码');
    copy.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(source);
        copy.dataset.tooltip = '已复制';
      } catch (_) { copy.dataset.tooltip = '复制失败'; }
      copy.setAttribute('aria-label',copy.dataset.tooltip);
      setTimeout(() => { copy.dataset.tooltip = '复制代码'; copy.setAttribute('aria-label','复制代码'); },1600);
    });
    frame.append(tools);
  });
  document.addEventListener("DOMContentLoaded", () => {
    if (window.katex) {
      document.querySelectorAll(".math-inline,.math-block").forEach((node) => {
        const block = node.classList.contains("math-block");
        const delimiter = block ? 2 : 1;
        const source = node.textContent.trim().slice(delimiter, -delimiter).trim();
        try { window.katex.render(source, node, {displayMode: block, throwOnError: false, trust: false}); } catch (_) {}
      });
    }
  });
  document.querySelectorAll(".prose img").forEach((image) => {
    if (image.closest("a")) return;
    image.tabIndex = 0;
    image.setAttribute("role", "button");
    image.setAttribute("aria-label", "放大图片：" + image.alt);
    const open = () => {
      const dialog = document.createElement("dialog");
      dialog.className = "lightbox";
      const close = document.createElement("button");
      close.type = "button"; close.textContent = "×"; close.setAttribute("aria-label", "关闭图片");
      const enlarged = document.createElement("img"); enlarged.src = image.currentSrc || image.src; enlarged.alt = image.alt;
      dialog.append(close, enlarged); document.body.append(dialog);
      close.addEventListener("click", () => dialog.close());
      dialog.addEventListener("click", (event) => { if (event.target === dialog) dialog.close(); });
      dialog.addEventListener("close", () => { dialog.remove(); image.focus(); }, {once: true});
      dialog.showModal();
    };
    image.addEventListener("click", open);
    image.addEventListener("keydown", (event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); open(); } });
  });
  fetch("/data/stats.json", {cache: "no-cache"}).then((response) => { if (!response.ok) throw new Error("stats"); return response.json(); }).then((data) => {
    document.querySelectorAll("[data-views-for]").forEach((node) => {
      const count = data.pages?.[node.dataset.viewsFor]?.views;
      if (Number.isSafeInteger(count) && count >= 0) { node.textContent = count.toLocaleString() + " 次阅读"; node.hidden = false; }
    });
    const list = document.querySelector("[data-popular-posts]");
    if (list && Array.isArray(data.popular_30d)) {
      data.popular_30d.slice(0, 5).forEach((post) => {
        if (typeof post.url !== "string" || !/^\/(?:[a-z0-9-]+\/)+$/.test(post.url) || typeof post.title !== "string") return;
        const item = document.createElement("li"); const link = document.createElement("a");
        link.href = post.url; link.textContent = post.title; item.append(link); list.append(item);
      });
      if (list.childElementCount) list.closest("section").hidden = false;
    }
  }).catch(() => {});
})();
