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
})();
