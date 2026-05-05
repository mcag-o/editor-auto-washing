(function () {
  const tabs = Array.from(document.querySelectorAll('[data-tab]'));
  const panels = Array.from(document.querySelectorAll('[data-panel]'));

  function setActiveTab(name) {
    tabs.forEach((tab) => {
      const active = tab.getAttribute('data-tab') === name;
      tab.classList.toggle('is-active', active);
      tab.setAttribute('aria-selected', active ? 'true' : 'false');
    });

    panels.forEach((panel) => {
      const active = panel.getAttribute('data-panel') === name;
      panel.classList.toggle('is-active', active);
      panel.hidden = !active;
    });
  }

  tabs.forEach((tab) => {
    tab.addEventListener('click', () => setActiveTab(tab.getAttribute('data-tab')));
  });

  if (tabs.length > 0) {
    setActiveTab(tabs[0].getAttribute('data-tab'));
  }
})();
