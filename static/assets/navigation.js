(() => {
  "use strict";
  const main = document.querySelector('#main');
  if (!main || main.querySelector('script') || !window.Swup || !window.SwupScrollPlugin || !window.BloGoPage) return;
  if (!window.fetch || !window.DOMParser || !window.history?.pushState || !window.history?.replaceState) return;

  const metadata = 'meta[name="description"],meta[name="robots"],meta[property],link[rel="canonical"]';
  const runtime = doc => [...doc.querySelectorAll('[data-blogo-runtime]')].map(el => el.getAttribute('src') || el.getAttribute('href')).join('\n');
  const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
  let swup;
  let viewSerial = 0;
  try {
    swup = new window.Swup({
      containers:['#main'],
      animationSelector:'#main',
      animateHistoryBrowsing:true,
      cache:false,
      timeout:10000,
      ignoreVisit:(url,{el}={}) => {
        const path = new URL(url,location.href).pathname;
        return !!el?.closest('[data-no-swup]') || /\.[a-z0-9]+$/i.test(path) && !/\.html?$/i.test(path);
      },
      plugins:[new window.SwupScrollPlugin({animateScroll:false})]
    });
  } catch (_) { return; }

  swup.hooks.on('visit:start', visit => {
    if (motion.matches) visit.animation.animate = false;
  });
  // Replacement hooks propagate failures to Swup's normal-navigation fallback.
  swup.hooks.replace('content:replace', async (visit, args, replace) => {
    const incoming = visit.to.document;
    const next = incoming?.querySelector('#main');
    if (!next || next.querySelector('script') || runtime(incoming) !== runtime(document)) {
      visit.ignore();
      throw new Error('This page requires normal navigation');
    }
    document.dispatchEvent(new Event('blogo:page-leave'));
    window.BloGoPage.destroy();
    await replace(visit,args);
    document.head.querySelectorAll(metadata).forEach(el => el.remove());
    incoming.head.querySelectorAll(metadata).forEach(el => document.head.append(el.cloneNode(true)));
    for (const name of ['lang','dir']) {
      const value = incoming.documentElement.getAttribute(name);
      if (value === null) document.documentElement.removeAttribute(name);
      else document.documentElement.setAttribute(name,value);
    }
    document.querySelectorAll('.nav-links a').forEach(link => {
      const next = [...incoming.querySelectorAll('.nav-links a')].find(el => el.getAttribute('href') === link.getAttribute('href'));
      if (next?.hasAttribute('aria-current')) link.setAttribute('aria-current',next.getAttribute('aria-current'));
      else link.removeAttribute('aria-current');
    });
    const main = document.querySelector('#main');
    document.body.dataset.pageId = main.dataset.pageId;
    document.body.dataset.siteTimezone = main.dataset.siteTimezone;
    document.body.dataset.blogoView = String(++viewSerial);
    await window.BloGoPage.init();
  });
  swup.hooks.before('content:scroll', visit => {
    if (!visit.history.popstate) document.querySelectorAll('[data-swup-scroll-container]').forEach(el => { el.scrollTop = 0; el.scrollLeft = 0; });
  });
  swup.hooks.on('page:view', visit => {
    const main = document.querySelector('#main');
    if (!visit.to.hash) main.focus({preventScroll:true});
    if (main.dataset.statsSrc) {
      const script = document.createElement('script');
      script.src = main.dataset.statsSrc;
      script.dataset.pageId = main.dataset.pageId;
      script.dataset.siteTimezone = main.dataset.siteTimezone;
      script.dataset.referrer = new URL(visit.from.url,location.origin).href;
      script.dataset.blogoView = document.body.dataset.blogoView;
      script.onload = script.onerror = () => script.remove();
      document.body.append(script);
    }
  });
})();
