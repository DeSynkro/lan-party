// LAN Party web host: small enhancements only. Every flow works without this
// file; it adds clipboard copy and live status updates.
(() => {
  "use strict";

  document.addEventListener("click", (event) => {
    const button = event.target.closest("[data-copy]");
    if (!button) return;
    const target = document.getElementById(button.dataset.copy);
    if (!target) return;
    const text = target.value !== undefined ? target.value : target.textContent;
    const done = () => {
      const label = button.textContent;
      button.textContent = "Copied";
      window.setTimeout(() => {
        button.textContent = label;
      }, 1600);
    };
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(done, () => select(target));
    } else {
      select(target);
    }
  });

  function select(target) {
    if (target.select) {
      target.select();
      return;
    }
    const range = document.createRange();
    range.selectNodeContents(target);
    const selection = window.getSelection();
    selection.removeAllRanges();
    selection.addRange(range);
  }

  const dialog = document.getElementById("modal");
  const modalBody = document.getElementById("modal-body");

  if (dialog && modalBody && typeof dialog.showModal === "function") {
    let pageScrollY = 0;

    const render = (html) => {
      const doc = new DOMParser().parseFromString(html, "text/html");
      const main = doc.querySelector("main");
      modalBody.replaceChildren();
      if (main) {
        for (const node of Array.from(main.childNodes)) {
          modalBody.append(document.importNode(node, true));
        }
      } else {
        const message = document.createElement("p");
        message.className = "muted";
        message.textContent = "Could not load that form.";
        modalBody.append(message);
      }
      const heading = modalBody.querySelector("h1");
      if (heading) {
        if (!heading.id) heading.id = "modal-title";
        dialog.setAttribute("aria-labelledby", heading.id);
      }
      const focusable = modalBody.querySelector(
        "input:not([type=hidden]), select, textarea, button, a[href]",
      );
      if (focusable) focusable.focus({ preventScroll: true });
    };

    const showError = (text) => {
      const message = document.createElement("p");
      message.className = "banner danger";
      message.textContent = text;
      modalBody.prepend(message);
    };

    const open = async (url) => {
      if (!dialog.open) {
        pageScrollY = window.scrollY;
        dialog.showModal();
      }
      modalBody.replaceChildren();
      const loading = document.createElement("p");
      loading.className = "muted";
      loading.textContent = "Loading…";
      modalBody.append(loading);
      try {
        const response = await fetch(url, { credentials: "same-origin" });
        render(await response.text());
      } catch (_) {
        modalBody.replaceChildren();
        showError("Could not load that form. Check the connection and try again.");
      }
    };

    document.addEventListener("click", (event) => {
      if (
        event.defaultPrevented ||
        event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey
      ) {
        return;
      }
      const link = event.target.closest("a[data-modal]");
      if (!link || (link.target && link.target !== "_self")) return;
      event.preventDefault();
      open(link.href);
    });

    dialog.addEventListener("click", (event) => {
      if (event.target === dialog) dialog.close();
    });

    dialog.addEventListener("close", () => {
      modalBody.replaceChildren();
      dialog.removeAttribute("aria-labelledby");
      const y = pageScrollY;
      window.setTimeout(() => window.scrollTo(0, y), 0);
    });

    dialog.addEventListener("submit", async (event) => {
      const form = event.target;
      if (form.method === "dialog") return;
      event.preventDefault();

      const method = (form.getAttribute("method") || "get").toLowerCase();
      if (method === "get") {
        const url = new URL(form.action, window.location.href);
        url.search = new URLSearchParams(new FormData(form)).toString();
        open(url.toString());
        return;
      }

      try {
        const response = await fetch(form.action, {
          method: "POST",
          body: new URLSearchParams(new FormData(form)),
          credentials: "same-origin",
        });
        if (response.redirected) {
          window.location.href = response.url;
          return;
        }
        const html = await response.text();
        if (response.ok) {
          window.location.reload();
          return;
        }
        render(html);
      } catch (_) {
        showError("Could not save that form. Check the connection and try again.");
      }
    });
  }

  if (!document.body.hasAttribute("data-live-root") || !("EventSource" in window)) {
    return;
  }  let source;
  let retry = 1000;

  const pill = (element, live, text) => {
    if (!element) return;
    element.replaceChildren();
    const dot = document.createElement("span");
    dot.className = live ? "dot live" : "dot";
    element.append(dot, document.createTextNode(text));
  };

  const selectors = (name, id) =>
    document.querySelectorAll("[" + name + '="' + window.CSS.escape(id) + '"]');

  const groupID = document.body.dataset.groupId || "";
  const expectedMembers = document.body.dataset.members || "";
  const expectedInvites = document.body.dataset.invites || "";
  let pendingReload = false;

  const isTyping = () => {
    const element = document.activeElement;
    return Boolean(
      element &&
        (element.matches("input, textarea, select") || element.isContentEditable),
    );
  };

  document.addEventListener("focusout", () => {
    if (!pendingReload) return;
    window.setTimeout(() => {
      if (!isTyping()) window.location.reload();
    }, 150);
  });

  const groupStructureChanged = (group) => {
    const members = (group.members || []).map((member) => member.id).join(",");
    const invites = (group.invites || []).join(",");
    return members !== expectedMembers || invites !== expectedInvites;
  };

  const apply = (snap) => {
    const approval = snap.approval || "";
    if (approval !== (document.body.dataset.approval || "")) {
      window.location.reload();
      return;
    }
    if (groupID) {
      const current = (snap.groups || []).find((group) => group.id === groupID);
      if (current && groupStructureChanged(current)) {
        if (isTyping()) {
          pendingReload = true;
          return;
        }
        window.location.reload();
        return;
      }
    }
    for (const group of snap.groups || []) {
      for (const element of selectors("data-connected", group.id)) {
        element.textContent = String(group.connected);
      }
      for (const element of selectors("data-group-live", group.id)) {
        pill(element, group.live, group.live ? "Live" : "Offline");
      }
      for (const element of selectors("data-session-state", group.id)) {
        pill(element, group.live, group.live ? "Live" : "Session off");
      }
      for (const member of group.members || []) {
        for (const element of selectors("data-member-state", member.id)) {
          pill(element, member.live, member.live ? "Connected" : "Offline");
        }
      }
    }
  };

  const connect = () => {
    source = new EventSource("/events");
    source.addEventListener("message", (event) => {
      retry = 1000;
      try {
        apply(JSON.parse(event.data));
      } catch (_) {
        /* ignore a malformed frame */
      }
    });
    source.addEventListener("error", () => {
      source.close();
      window.setTimeout(connect, retry);
      retry = Math.min(retry * 2, 30000);
    });
  };

  connect();
})();
