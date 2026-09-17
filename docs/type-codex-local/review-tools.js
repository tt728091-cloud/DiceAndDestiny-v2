/* Local review aids. These never authorize implementation or contact a server. */
(() => {
  'use strict';
  const storageKey = 'dice-destiny-type-review-v1';
  let notes = {};
  let storageAvailable = true;
  try { notes = JSON.parse(localStorage.getItem(storageKey) || '{}') || {}; }
  catch (_) { storageAvailable = false; }
  if (typeof notes !== 'object' || Array.isArray(notes)) notes = {};
  function save(type) {
    const panel = document.getElementById(type + '-panel');
    const check = panel.querySelector('[data-review-check]');
    const field = panel.querySelector('[data-review-notes]');
    notes[type] = { reviewed: check.checked, notes: field.value, draftVersion: Number(panel.dataset.draftVersion), updatedAt: new Date().toISOString() };
    try { localStorage.setItem(storageKey, JSON.stringify(notes)); storageAvailable = true; }
    catch (_) { storageAvailable = false; }
    panel.querySelector('[data-review-save]').textContent = storageAvailable ? 'Saved in this browser.' : 'Local storage unavailable. Export notes before closing.';
    const tab = document.getElementById('type-tab-' + type);
    if (tab) tab.classList.toggle('draft-reviewed', check.checked);
  }
  document.querySelectorAll('[data-review-check]').forEach(check => {
    const type = check.dataset.reviewCheck;
    const panel = document.getElementById(type + '-panel');
    const field = panel.querySelector('[data-review-notes]');
    const saved = notes[type] || {};
    check.checked = saved.reviewed === true && Number(saved.draftVersion) === Number(panel.dataset.draftVersion);
    field.value = typeof saved.notes === 'string' ? saved.notes : '';
    check.addEventListener('change', () => save(type));
    field.addEventListener('input', () => save(type));
    if (!storageAvailable) panel.querySelector('[data-review-save]').textContent = 'Local storage unavailable. Export notes before closing.';
  });
  document.querySelectorAll('[data-export-reviews]').forEach(button => button.addEventListener('click', () => {
    const blob = new Blob([JSON.stringify({scope:'Design review notes; not implementation authorization', exportedAt:new Date().toISOString(), types:notes}, null, 2)], {type:'application/json'});
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a'); link.href = url; link.download = 'dice-destiny-type-review-notes.json'; link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }));
  document.querySelectorAll('[data-card-search]').forEach(input => input.addEventListener('input', () => {
    const type = input.dataset.cardSearch;
    const panel = document.getElementById(type + '-panel');
    const query = input.value.toLocaleLowerCase().trim();
    let count = 0;
    panel.querySelectorAll('[data-draft-card]').forEach(card => {
      card.hidden = !card.textContent.toLocaleLowerCase().includes(query);
      if (!card.hidden) count++;
    });
    panel.querySelectorAll('[data-card-group]').forEach(group => { group.hidden = !Array.from(group.querySelectorAll('[data-draft-card]')).some(card => !card.hidden); });
    document.getElementById(type + '-card-count').textContent = count + ' of 24 cards';
  }));
  document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('[data-review-check]').forEach(check => {
      const tab = document.getElementById('type-tab-' + check.dataset.reviewCheck);
      if (tab) tab.classList.toggle('draft-reviewed', check.checked);
    });
  });
})();
