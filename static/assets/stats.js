(() => {
  "use strict";
  const script = document.currentScript;
  if (!script || !script.dataset.pageId) return;
  const randomID = () => Array.from(crypto.getRandomValues(new Uint8Array(16)), n => n.toString(16).padStart(2, "0")).join("");
  const send = () => {
    document.removeEventListener("visibilitychange", visible);
    try {
      const parts = new Intl.DateTimeFormat("en-CA", {timeZone: script.dataset.siteTimezone, year: "numeric", month: "2-digit", day: "2-digit"}).formatToParts(new Date());
      const part = name => parts.find(p => p.type === name).value;
      const day = `${part("year")}-${part("month")}-${part("day")}`;
      const eventID = randomID();
      let visitorID;
      try {
        const saved = JSON.parse(localStorage.getItem("blogo:visitor:v1"));
        if (saved && saved.day === day && /^[0-9a-f]{32}$/.test(saved.id)) visitorID = saved.id;
      } catch (_) {}
      if (!visitorID) visitorID = randomID();
      try {localStorage.setItem("blogo:visitor:v1", JSON.stringify({day, id: visitorID}));} catch (_) {}
      let source = "";
      try {if (document.referrer) source = new URL(document.referrer).hostname;} catch (_) {}
      const body = new Blob([JSON.stringify({v: 1, event_id: eventID, page_id: script.dataset.pageId, visitor_id: visitorID, referrer_host: source})], {type: "application/json"});
      const queue = () => {try {return navigator.sendBeacon("/api/events", body);} catch (_) {return false;}};
      if (!queue()) setTimeout(queue, 1000);
    } catch (_) {}
  };
  const visible = () => {if (document.visibilityState === "visible") send();};
  if (document.visibilityState === "visible") send();
  else document.addEventListener("visibilitychange", visible);
})();
