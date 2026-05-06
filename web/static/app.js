(function () {
  const tabs = Array.from(document.querySelectorAll('[data-tab]'));
  const panels = Array.from(document.querySelectorAll('[data-panel]'));
  const filterButtons = Array.from(document.querySelectorAll('[data-filter]'));
  const articleRows = Array.from(document.querySelectorAll('tbody tr[data-status]'));

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

  function setArticleFilter(name) {
    filterButtons.forEach((button) => {
      const active = button.getAttribute('data-filter') === name;
      button.classList.toggle('is-active', active);
      button.setAttribute('aria-pressed', active ? 'true' : 'false');
    });

    articleRows.forEach((row) => {
      const status = row.getAttribute('data-status');
      const visible = name === 'all' || status === name;
      row.hidden = !visible;
    });
  }

  tabs.forEach((tab) => {
    tab.addEventListener('click', () => setActiveTab(tab.getAttribute('data-tab')));
  });

  filterButtons.forEach((button) => {
    button.addEventListener('click', () => setArticleFilter(button.getAttribute('data-filter')));
  });

  if (tabs.length > 0) {
    setActiveTab(tabs[0].getAttribute('data-tab'));
  }

  if (filterButtons.length > 0) {
    setArticleFilter('all');
  }
})();
