(() => {
  "use strict";
  const root = document.documentElement;
  let preference;
  try { preference = localStorage.getItem("blogo-theme"); } catch (_) {}
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  root.dataset.theme = preference === "light" || preference === "dark" ? preference : (media.matches ? "dark" : "light");
  const toggle = document.querySelector(".theme-toggle");
  if (toggle) {
    toggle.hidden = false;
    toggle.addEventListener("click", () => {
      preference = root.dataset.theme === "dark" ? "light" : "dark";
      root.dataset.theme = preference;
      try { localStorage.setItem("blogo-theme", preference); } catch (_) {}
    });
    media.addEventListener("change", (event) => { if (!preference) root.dataset.theme = event.matches ? "dark" : "light"; });
  }
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
