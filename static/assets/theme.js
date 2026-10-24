(() => {
  "use strict";
  const root = document.documentElement;
  document.querySelectorAll('.site-profile [data-email]').forEach(node => {
    const bytes = Uint8Array.from(atob(node.dataset.email), character => character.charCodeAt(0) ^ 0xa7);
    node.textContent = new TextDecoder().decode(bytes);
    node.removeAttribute('data-email');
  });
  const icons = {
    light: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5"/>',
    dark: '<path d="M20.5 14a8.5 8.5 0 0 1-10.5-10.5A8.5 8.5 0 1 0 20.5 14Z"/>',
    wrapOff: '<path d="M3 5h18M3 10h18M3 15h17m-3-3 3 3-3 3M3 20h11"/>',
    wrapOn: '<path d="M3 5h18M3 10h13a4 4 0 0 1 0 8h-4m3-3-3 3 3 3M3 15h4M3 20h4"/>',
    copy: '<rect x="8" y="3" width="12" height="14" rx="2"/><path d="M16 17v3a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V8a1 1 0 0 1 1-1h3"/>'
  };
  icons.system = `<defs><clipPath id="blogo-sun-half"><path d="M0 0H24L0 24Z"/></clipPath><clipPath id="blogo-moon-half"><path d="M24 0V24H0Z"/></clipPath></defs><g clip-path="url(#blogo-sun-half)">${icons.light}</g><g clip-path="url(#blogo-moon-half)">${icons.dark}</g><path d="M3 21 21 3"/>`;
  const icon = name => `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${icons[name]}</svg>`;
  let tooltipOwner, tooltipTimer;
  const hideTooltip = () => {
    clearTimeout(tooltipTimer);
    if (tooltipOwner) tooltipOwner.classList.remove('tooltip-visible');
    tooltipOwner = null;
  };
  const showTooltip = button => {
    if (root.dataset.input !== 'touch') return;
    hideTooltip();
    tooltipOwner = button;
    button.classList.add('tooltip-visible');
    tooltipTimer = setTimeout(() => button.classList.remove('tooltip-visible'),1200);
  };
  document.addEventListener('pointerdown', event => {
    root.dataset.input = event.pointerType === 'touch' || event.pointerType === 'pen' ? 'touch' : 'mouse';
    if (tooltipOwner && !tooltipOwner.contains(event.target)) hideTooltip();
  });
  document.addEventListener('pointermove', event => {
    if (event.pointerType === 'mouse' && root.dataset.input === 'touch') {
      root.dataset.input = 'mouse';
      hideTooltip();
    }
  });
  document.addEventListener('keydown', () => { root.dataset.input = 'keyboard'; hideTooltip(); });
  document.addEventListener('contextmenu', event => {
    if (root.dataset.input === 'touch' && event.target.closest('.icon-button')) event.preventDefault();
  });
  let diagrams = [], activeMain, mermaidLoading;
  let drawing = Promise.resolve(), diagramID = 0;
  const renderDiagrams = () => {
    drawing = drawing.then(async () => {
      if (!window.mermaid || !diagrams.length) return;
      window.mermaid.initialize({startOnLoad:false, securityLevel:'strict', suppressErrorRendering:true, theme:root.dataset.theme === 'dark' ? 'dark' : 'default', htmlLabels:false, fontFamily:'system-ui'});
      for (const diagram of diagrams) {
        try {
          const result = await window.mermaid.render(`blogo-diagram-${++diagramID}`, diagram.text);
          if (!diagram.source.isConnected) continue;
          diagram.output.innerHTML = result.svg;
          diagram.output.hidden = false;
          diagram.source.hidden = true;
        } catch (_) {
          diagram.output.hidden = true;
          diagram.source.hidden = false;
        }
      }
    });
    return drawing;
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
      showTooltip(toggle);
      try { localStorage.setItem("blogo-theme", preference); } catch (_) {}
    });
    media.addEventListener("change", () => { if (preference === 'system') applyTheme(); });
  }
  const destroyPage = () => {
    hideTooltip();
    document.querySelectorAll('.lightbox').forEach(dialog => dialog.remove());
    diagrams = [];
    activeMain = null;
  };
  const initPage = async () => {
    const main = document.querySelector('#main');
    if (!main || main === activeMain) return;
    activeMain = main;
    main.querySelectorAll('.friend-card img').forEach(image => {
      const useDefault = () => {
        image.removeEventListener('error', useDefault);
        image.src = '/images/avatar-default.svg';
      };
      image.addEventListener('error', useDefault);
      if (image.complete && image.naturalWidth === 0) useDefault();
    });
    diagrams = [...main.querySelectorAll('pre.mermaid')].map(source => {
      const output = document.createElement('div');
      output.className = 'mermaid-diagram';
      output.hidden = true;
      source.after(output);
      return {source, output, text:source.textContent};
    });
    main.querySelectorAll('.prose pre:not(.mermaid)').forEach(pre => {
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
      const wrap = button('wrapOff','开启自动换行');
      wrap.setAttribute('aria-pressed','false');
      wrap.addEventListener('click', () => {
        const enabled = frame.classList.toggle('is-wrapped');
        wrap.setAttribute('aria-pressed',String(enabled));
        wrap.innerHTML = icon(enabled ? 'wrapOn' : 'wrapOff');
        wrap.dataset.tooltip = enabled ? '关闭自动换行' : '开启自动换行';
        wrap.setAttribute('aria-label',wrap.dataset.tooltip);
        showTooltip(wrap);
      });
      const copy = button('copy','复制代码');
      let feedbackTimer;
      copy.addEventListener('click', async () => {
        clearTimeout(feedbackTimer);
        showTooltip(copy);
        try {
          await navigator.clipboard.writeText(source);
          copy.dataset.tooltip = '已复制';
        } catch (_) { copy.dataset.tooltip = '复制失败'; }
        copy.setAttribute('aria-label',copy.dataset.tooltip);
        if (tooltipOwner === copy) showTooltip(copy);
        feedbackTimer = setTimeout(() => { copy.dataset.tooltip = '复制代码'; copy.setAttribute('aria-label','复制代码'); },1600);
      });
      frame.append(tools);
    });
    if (window.katex) {
      main.querySelectorAll(".math-inline,.math-block").forEach((node) => {
        const block = node.classList.contains("math-block");
        const delimiter = block ? 2 : 1;
        const source = node.textContent.trim().slice(delimiter, -delimiter).trim();
        try { window.katex.render(source, node, {displayMode: block, throwOnError: false, trust: false}); } catch (_) {}
      });
    }
    main.querySelectorAll(".prose img").forEach((image) => {
      if (image.closest("a")) return;
      image.tabIndex = 0;
      image.setAttribute("role", "button");
      image.setAttribute("aria-label", "放大图片：" + image.alt);
      const open = async () => {
        if (!image.naturalWidth || document.querySelector('.lightbox')) return;
        const dialog = document.createElement("dialog");
        dialog.className = "lightbox";
        const close = document.createElement("button");
        close.type = "button"; close.textContent = "×"; close.setAttribute("aria-label", "关闭图片");
        const enlarged = document.createElement("img"); enlarged.src = image.currentSrc || image.src; enlarged.alt = image.alt;
        dialog.append(close, enlarged); document.body.append(dialog);
        const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
        let animation, closing = false;
        const finish = () => { if (dialog.isConnected) dialog.close(); };
        const dismiss = () => {
          if (closing) return;
          closing = true;
          dialog.classList.remove('is-open');
          if (!animation || motion.matches) { finish(); return; }
          animation.reverse();
          animation.finished.then(finish, () => {});
        };
        close.addEventListener("click", dismiss);
        dialog.addEventListener("click", (event) => { if (event.target === dialog) dismiss(); });
        dialog.addEventListener("cancel", (event) => { event.preventDefault(); dismiss(); });
        dialog.addEventListener("close", () => { animation?.cancel(); dialog.remove(); image.focus({preventScroll:true}); }, {once: true});
        await enlarged.decode().catch(() => {});
        if (!dialog.isConnected || !image.isConnected || !enlarged.naturalWidth) { dialog.remove(); return; }
        dialog.showModal();
        if (!motion.matches) {
          const from = image.getBoundingClientRect(), to = enlarged.getBoundingClientRect();
          animation = enlarged.animate([
            {transform:`translate(${from.left-to.left}px,${from.top-to.top}px) scale(${from.width/to.width},${from.height/to.height})`},
            {transform:'none'}
          ], {duration:240,easing:'cubic-bezier(.2,.7,.2,1)',fill:'both'});
        }
        // Resolve the transparent backdrop before starting its transition.
        getComputedStyle(dialog,'::backdrop').backgroundColor;
        dialog.classList.add('is-open');
      };
      image.addEventListener("click", open);
      image.addEventListener("keydown", (event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); open(); } });
    });
    fetch("/data/stats.json", {cache: "no-cache"}).then((response) => { if (!response.ok) throw new Error("stats"); return response.json(); }).then((data) => {
      if (main !== activeMain) return;
      main.querySelectorAll("[data-views-for]").forEach((node) => {
        const count = data.pages?.[node.dataset.viewsFor]?.views;
        if (Number.isSafeInteger(count) && count >= 0) { node.textContent = count.toLocaleString() + " 次阅读"; node.hidden = false; }
      });
      const list = document.querySelector("[data-popular-posts]");
      if (list && !list.dataset.loaded && Array.isArray(data.popular_30d)) {
        list.dataset.loaded = 'true';
        data.popular_30d.slice(0, 5).forEach((post) => {
          if (typeof post.url !== "string" || !/^\/(?:[a-z0-9-]+\/)+$/.test(post.url) || typeof post.title !== "string") return;
          const item = document.createElement("li"); const link = document.createElement("a");
          link.href = post.url; link.textContent = post.title; item.append(link); list.append(item);
        });
        if (list.childElementCount) list.closest("section").hidden = false;
      }
    }).catch(() => {});
    if (diagrams.length && !window.mermaid && main.dataset.mermaidSrc) {
      if (!mermaidLoading) mermaidLoading = new Promise((resolve,reject) => {
        const script = document.createElement('script');
        script.src = main.dataset.mermaidSrc;
        script.onload = resolve;
        script.onerror = () => { script.remove(); mermaidLoading = null; reject(new Error('Mermaid failed to load')); };
        document.head.append(script);
      });
      await mermaidLoading.catch(() => {});
    }
    if (main !== activeMain) return;
    await renderDiagrams();
    if (main !== activeMain) return;
  };
  window.BloGoPage = {init:initPage, destroy:destroyPage};
  const initialPage = () => { initPage().catch(() => {}); };
  if (document.readyState === 'complete') initialPage();
  else document.addEventListener('DOMContentLoaded',initialPage,{once:true});
})();
