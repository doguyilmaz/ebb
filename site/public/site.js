for (const button of document.querySelectorAll("button[data-copy]")) {
  const label = button.getAttribute("aria-label");
  let timer;
  button.addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(button.dataset.copy);
    } catch {
      return;
    }
    clearTimeout(timer);
    button.setAttribute("data-done", "");
    button.setAttribute("aria-label", "Copied");
    timer = setTimeout(() => {
      button.removeAttribute("data-done");
      button.setAttribute("aria-label", label);
    }, 1400);
  });
}
