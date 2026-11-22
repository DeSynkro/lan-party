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

  if (!document.body.hasAttribute("data-live-root") || !("EventSource" in window)) {
    return;
  }

  let source;
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
