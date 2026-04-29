const state = {
  overview: null,
  users: [],
  homepageUsers: [],
  currentView: "home",
  deckPollingBusy: false,
  editorInteractionUntil: 0,
  selectedUserId: null,
  selectedDeckKey: "",
  selectedTable: null,
  tableRows: [],
  tableAnnotations: [],
  tableSchema: null,
  tableSearch: "",
  weaponInventorySearch: "",
  weaponEquippedOnly: false,
  gachaBannerCatalog: null,
  gachaBannerSearch: "",
  bannerSubtab: "usable",
  bannerShowSelectedOnly: false,
  eventCatalog: null,
  eventSearch: "",
  eventSubtab: "event_quests",
  eventShowSelectedOnly: false,
  eventCategoryFilter: "all",
  presetCatalog: null,
  selectedPresetIds: [],
  lookupOptions: {},
  activeDeck: null,
  bundleCatalog: null,
  bundleSchedule: null,
};

const els = {
  dbPath: document.getElementById("db-path"),
  overviewStats: document.getElementById("overview-stats"),
  homeButton: document.getElementById("home-button"),
  bannerButton: document.getElementById("banner-button"),
  eventsButton: document.getElementById("events-button"),
  presetsPanel: document.getElementById("presets-panel"),
  presetsStatus: document.getElementById("presets-status"),
  presetsList: document.getElementById("presets-list"),
  presetsReload: document.getElementById("presets-reload"),
  presetsClear: document.getElementById("presets-clear"),
  presetsApplyAdd: document.getElementById("presets-apply-add"),
  presetsApplyReplace: document.getElementById("presets-apply-replace"),
  homeView: document.getElementById("home-view"),
  bannerView: document.getElementById("banner-view"),
  eventsView: document.getElementById("events-view"),
  editorView: document.getElementById("editor-view"),
  homepageUsers: document.getElementById("homepage-users"),
  userList: document.getElementById("user-list"),
  usersEmpty: document.getElementById("users-empty"),
  refreshUsers: document.getElementById("refresh-users"),
  heroTitle: document.getElementById("hero-title"),
  heroSubtitle: document.getElementById("hero-subtitle"),
  heroScreen: document.getElementById("hero-screen"),
  heroFallbackImage: document.getElementById("hero-fallback-image"),
  heroDeckToolbar: document.getElementById("hero-deck-toolbar"),
  deckSelect: document.getElementById("deck-select"),
  deckVisual: document.getElementById("deck-visual"),
  userSummaryPanel: document.getElementById("user-summary-panel"),
  userSummaryGrid: document.getElementById("user-summary-grid"),
  deleteUser: document.getElementById("delete-user"),
  invalidateSessions: document.getElementById("invalidate-sessions"),
  tablePanel: document.getElementById("table-panel"),
  tableHeading: document.getElementById("table-heading"),
  tableSearch: document.getElementById("table-search"),
  tableSelectWrap: document.getElementById("table-select-wrap"),
  tableSelectTrigger: document.getElementById("table-select-trigger"),
  tableSelectLabel: document.getElementById("table-select-label"),
  tableSelectPanel: document.getElementById("table-select-panel"),
  addRow: document.getElementById("add-row"),
  editorTable: document.getElementById("editor-table"),
  tableEmpty: document.getElementById("table-empty"),
  tableMeta: document.getElementById("table-meta"),
  weaponTableFilters: document.getElementById("weapon-table-filters"),
  weaponTableSearch: document.getElementById("weapon-table-search"),
  weaponEquippedOnly: document.getElementById("weapon-equipped-only"),
  bannerStatus: document.getElementById("banner-status"),
  bannerUsableTab: document.getElementById("banner-usable-tab"),
  bannerUnusableTab: document.getElementById("banner-unusable-tab"),
  bannerSelectedToggle: document.getElementById("banner-selected-toggle"),
  bannerSummaryStats: document.getElementById("banner-summary-stats"),
  bannerAppliedSummary: document.getElementById("banner-applied-summary"),
  bannerAppliedList: document.getElementById("banner-applied-list"),
  bannerSearch: document.getElementById("banner-search"),
  bannerList: document.getElementById("banner-list"),
  bannerSelectAll: document.getElementById("banner-select-all"),
  bannerClearAll: document.getElementById("banner-clear-all"),
  bannerReload: document.getElementById("banner-reload"),
  bannerSave: document.getElementById("banner-save"),
  eventsStatus: document.getElementById("events-status"),
  eventsEventTab: document.getElementById("events-event-tab"),
  eventsSideTab: document.getElementById("events-side-tab"),
  eventsCategoryNav: document.getElementById("events-category-nav"),
  eventsSelectedToggle: document.getElementById("events-selected-toggle"),
  eventsSummaryStats: document.getElementById("events-summary-stats"),
  eventsAppliedSummary: document.getElementById("events-applied-summary"),
  eventsAppliedList: document.getElementById("events-applied-list"),
  eventsSearch: document.getElementById("events-search"),
  eventsList: document.getElementById("events-list"),
  eventsSelectAll: document.getElementById("events-select-all"),
  eventsClearAll: document.getElementById("events-clear-all"),
  eventsReload: document.getElementById("events-reload"),
  eventsSave: document.getElementById("events-save"),
  bundlesButton: document.getElementById("bundles-button"),
  bundlesView: document.getElementById("bundles-view"),
  bundlesStatus: document.getElementById("bundles-status"),
  bundlesGrid: document.getElementById("bundles-grid"),
  bundlesSummaryStats: document.getElementById("bundles-summary-stats"),
  bundlesUnreleased: document.getElementById("bundles-unreleased"),
  bundlesSelectAll: document.getElementById("bundles-select-all"),
  bundlesClearAll: document.getElementById("bundles-clear-all"),
  bundlesRegenerate: document.getElementById("bundles-regenerate"),
  bundlesApply: document.getElementById("bundles-apply"),
};

async function fetchJSON(url, options = {}) {
  const response = await fetch(url, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Request failed: ${response.status}`);
  }
  return response.json();
}

function formatNumber(value) {
  return new Intl.NumberFormat().format(Number(value || 0));
}

function tableLabel(name) {
  return name.replace(/^user_/, "").replaceAll("_", " ");
}

function renderOverview() {
  els.dbPath.textContent = state.overview.dbPath;
  const stats = [
    ["Users", state.overview.userCount],
    ["Tables", state.overview.tableCount],
    ["Weapons", state.overview.rowCounts.user_weapons || 0],
    ["Quests", state.overview.rowCounts.user_quests || 0],
  ];

  els.overviewStats.innerHTML = "";
  for (const [label, value] of stats) {
    const card = document.createElement("div");
    card.className = "stat-card";
    card.innerHTML = `<span class="muted">${label}</span><span class="value">${formatNumber(value)}</span>`;
    els.overviewStats.append(card);
  }
}

function bannerSelectionSet() {
  return new Set((state.gachaBannerCatalog?.activeBannerIds || []).map((value) => String(value)));
}

function isRecordFullySelected(record, selectedIds) {
  return (record.momBannerIds || []).every((value) => selectedIds.has(String(value)));
}

function isRecordPartiallySelected(record, selectedIds) {
  const ids = record.momBannerIds || [];
  const selectedCount = ids.filter((value) => selectedIds.has(String(value))).length;
  return selectedCount > 0 && selectedCount < ids.length;
}

function recordsForCurrentBannerTab(records) {
  if (state.bannerSubtab === "unusable") {
    return state.gachaBannerCatalog?.unusableRecords || [];
  }
  return state.gachaBannerCatalog?.usableRecords || records;
}

function renderSelectionStats(target, items) {
  target.innerHTML = "";
  for (const item of items) {
    const card = document.createElement("div");
    card.className = "selection-stat";
    card.innerHTML = `<span class="selection-stat-label">${item.label}</span><span class="selection-stat-value">${item.value}</span>`;
    target.append(card);
  }
}

function selectedPresets() {
  const presets = state.presetCatalog?.presets || [];
  const chosen = new Set(state.selectedPresetIds);
  return presets.filter((preset) => chosen.has(preset.id));
}

function selectedPresetSummary() {
  const summary = {
    bannerIds: new Set(),
    eventQuestIds: new Set(),
  };
  for (const preset of selectedPresets()) {
    for (const bannerId of preset.bannerIds || []) {
      summary.bannerIds.add(Number(bannerId));
    }
    for (const eventId of preset.eventQuestIds || []) {
      summary.eventQuestIds.add(Number(eventId));
    }
  }
  return summary;
}

function renderPresetManager() {
  const payload = state.presetCatalog;
  els.presetsPanel.classList.toggle("hidden", !payload?.enabled);
  if (!payload?.enabled) {
    els.presetsStatus.textContent = "No preset catalog is available yet.";
    els.presetsList.innerHTML = `<div class="empty-state">No presets found.</div>`;
    return;
  }

  const summary = selectedPresetSummary();
  const checked = new Set(state.selectedPresetIds);
  const presetCount = selectedPresets().length;
  els.presetsStatus.textContent =
    presetCount > 0
      ? `${presetCount} preset${presetCount === 1 ? "" : "s"} selected. Applying them will affect ${formatNumber(summary.bannerIds.size)} banner row${summary.bannerIds.size === 1 ? "" : "s"} and ${formatNumber(summary.eventQuestIds.size)} event chapter${summary.eventQuestIds.size === 1 ? "" : "s"}. Side stories stay on, and nothing is saved until you use Save Banners or Save Events.`
      : `${payload.notes || "Preset application updates banner and event chapter selections only."} Select one or more presets, then add or replace the current working selection.`;

  els.presetsList.innerHTML = "";
  for (const preset of payload.presets || []) {
    const row = document.createElement("label");
    row.className = "preset-row";
    const isChecked = checked.has(preset.id);
    const previewBits = [
      `${preset.bannerCount || 0} banners`,
      `${preset.eventCount || 0} events`,
      (preset.bannerPreview || []).slice(0, 2).join(", "),
    ].filter(Boolean);
    row.innerHTML = `
      <input type="checkbox" class="banner-checkbox" data-preset-id="${preset.id}" ${isChecked ? "checked" : ""}>
      <div class="banner-copy">
        <div class="banner-title-row">
          <span class="banner-title">${preset.label}</span>
        </div>
        <div class="banner-detail">${preset.description || "No description provided."}</div>
        <div class="banner-detail">${previewBits.join(" · ")}</div>
        ${preset.missingEventQuestIds?.length ? `<div class="banner-detail">Missing event ids: ${preset.missingEventQuestIds.join(", ")}</div>` : ""}
      </div>
    `;
    row.querySelector('input[data-preset-id]')?.addEventListener("change", (event) => {
      const next = new Set(state.selectedPresetIds);
      if (event.target.checked) {
        next.add(preset.id);
      } else {
        next.delete(preset.id);
      }
      state.selectedPresetIds = [...next].sort();
      renderPresetManager();
    });
    els.presetsList.append(row);
  }
}

function applySelectedPresets(mode) {
  if (!state.selectedPresetIds.length) return;
  const summary = selectedPresetSummary();
  if (!state.gachaBannerCatalog || !state.eventCatalog?.groups?.event_quests) return;
  const bannerBase = mode === "replace" ? new Set() : bannerSelectionSet();
  for (const bannerId of summary.bannerIds) {
    bannerBase.add(String(bannerId));
  }
  state.gachaBannerCatalog.activeBannerIds = [...bannerBase].map((value) => Number(value)).sort((a, b) => a - b);

  const eventPayload = state.eventCatalog.groups.event_quests;
  const eventBase = mode === "replace" && summary.eventQuestIds.size > 0
    ? new Set()
    : new Set((eventPayload.activeIds || []).map((value) => String(value)));
  for (const eventId of summary.eventQuestIds) {
    eventBase.add(String(eventId));
  }
  eventPayload.activeIds = [...eventBase].map((value) => Number(value)).sort((a, b) => a - b);

  renderPresetManager();
  renderGachaBannerManager();
  renderEventSelector();
}

function renderAppliedItems(target, records, removeLabel, onRemove) {
  if (!records.length) {
    target.innerHTML = `<div class="empty-state">No selected items yet.</div>`;
    return;
  }
  target.innerHTML = "";
  for (const record of records) {
    const row = document.createElement("div");
    row.className = "applied-item";
    row.innerHTML = `
      <div class="applied-item-copy">
        <div class="applied-item-title">${record.label}</div>
        <div class="banner-detail">${record.detail || `id ${record.id}`}</div>
      </div>
      <button type="button" class="ghost-button applied-item-remove">${removeLabel}</button>
    `;
    row.querySelector(".applied-item-remove")?.addEventListener("click", () => onRemove(record));
    target.append(row);
  }
}

function renderGachaBannerManager() {
  const payload = state.gachaBannerCatalog;
  if (!payload?.enabled) {
    els.bannerStatus.textContent =
      "The Mom banner table is not available at the configured path, so banner editing is disabled.";
    els.bannerList.innerHTML = `<div class="empty-state">Banner catalog unavailable.</div>`;
    return;
  }

  const selectedIds = bannerSelectionSet();
  const records = payload.usableRecords || payload.records || [];
  const tabRecords = recordsForCurrentBannerTab(records);
  const appliedRecords = state.bannerSubtab === "usable"
    ? tabRecords.filter((record) => isRecordFullySelected(record, selectedIds))
    : [];
  const partiallyAppliedRecords = state.bannerSubtab === "usable"
    ? tabRecords.filter((record) => isRecordPartiallySelected(record, selectedIds))
    : [];
  const search = state.gachaBannerSearch.trim().toLowerCase();
  const filteredBase = tabRecords.filter((record) => {
    if (!search) return true;
    const haystack = `${record.label || ""} ${record.detail || ""} ${record.assetName || ""} ${record.group || ""} ${record.id || ""} ${record.gameGachaId || ""} ${record.usabilityReason || ""}`.toLowerCase();
    return haystack.includes(search);
  });
  const filtered = state.bannerShowSelectedOnly
    ? filteredBase.filter((record) => state.bannerSubtab === "usable" ? isRecordFullySelected(record, selectedIds) : false)
    : filteredBase;
  const sectionLabel = state.bannerSubtab === "usable" ? "usable" : "unusable";

  els.bannerStatus.textContent =
    `Editing ${payload.currentPath}. Showing ${filtered.length} of ${tabRecords.length} ${sectionLabel} entries derived from the current lunar-tear logic in ${payload.logicSourcePath || "gacha.go"}.`;
  els.bannerUsableTab.classList.toggle("active", state.bannerSubtab === "usable");
  els.bannerUnusableTab.classList.toggle("active", state.bannerSubtab === "unusable");
  els.bannerSelectedToggle.classList.toggle("active", state.bannerShowSelectedOnly);
  els.bannerSelectedToggle.disabled = state.bannerSubtab !== "usable";
  els.bannerSelectedToggle.textContent = state.bannerShowSelectedOnly ? "Show All" : "Selected Only";
  renderSelectionStats(els.bannerSummaryStats, state.bannerSubtab === "usable"
    ? [
      { label: "Selected", value: formatNumber(appliedRecords.length) },
      { label: "Partial", value: formatNumber(partiallyAppliedRecords.length) },
      { label: "Visible", value: formatNumber(filtered.length) },
    ]
    : [
      { label: "Ignored", value: formatNumber(tabRecords.length) },
      { label: "Visible", value: formatNumber(filtered.length) },
      { label: "Selected", value: "0" },
    ]);
  if (state.bannerSubtab === "usable") {
    els.bannerAppliedSummary.textContent =
      appliedRecords.length > 0
        ? `${appliedRecords.length} usable banner entr${appliedRecords.length === 1 ? "y is" : "ies are"} fully selected and will be applied in game after the next server restart.`
        : "No usable banner entries are fully selected right now.";
    renderAppliedItems(els.bannerAppliedList, appliedRecords, "Remove", (record) => {
      const next = bannerSelectionSet();
      for (const value of record.momBannerIds || []) {
        next.delete(String(value));
      }
      state.gachaBannerCatalog.activeBannerIds = [...next].map((item) => Number(item)).sort((a, b) => a - b);
      renderGachaBannerManager();
    });
  } else {
    els.bannerAppliedSummary.textContent =
      `${tabRecords.length} raw MomBanner row${tabRecords.length === 1 ? " is" : "s are"} currently ignored by lunar-tear and will not become in-game banners unless the server logic changes.`;
    els.bannerAppliedList.innerHTML = `<div class="empty-state">Ignored rows do not create applied in-game banners.</div>`;
  }
  els.bannerList.innerHTML = "";

  if (!filtered.length) {
    els.bannerList.innerHTML = `<div class="empty-state">${state.bannerShowSelectedOnly ? "No selected banners match the current search." : "No banners match the current search."}</div>`;
    return;
  }

  for (const record of filtered) {
    const row = document.createElement("label");
    row.className = "banner-row";

    const checked = isRecordFullySelected(record, selectedIds);
    const partial = isRecordPartiallySelected(record, selectedIds);
    const detailBits = [
      record.detail,
      record.assetName ? `asset ${record.assetName}` : "",
      `gacha ${record.gameGachaId || record.destinationDomainId || record.id}`,
      record.mode === "step-up" ? `${record.momBannerCount} linked rows` : `MomBanner ${record.momBannerIds?.[0] || record.id}`,
    ].filter(Boolean);

    row.innerHTML = `
      <input type="checkbox" class="banner-checkbox" data-entry-key="${record.entryKey}" ${checked ? "checked" : ""}>
      <div class="banner-copy">
        <div class="banner-title-row">
          <span class="banner-title">${record.label || `Gacha Banner ${record.id}`}</span>
          ${record.group ? `<span class="banner-group">${record.group}</span>` : ""}
          ${record.mode ? `<span class="banner-group">${record.mode}</span>` : ""}
          ${record.isUsable === false ? `<span class="banner-group">ignored</span>` : ""}
        </div>
        <div class="banner-detail">${detailBits.join(" · ")}</div>
        ${record.usabilityReason ? `<div class="banner-detail">${record.usabilityReason}</div>` : ""}
      </div>
    `;

    const checkbox = row.querySelector(".banner-checkbox");
    if (state.bannerSubtab === "unusable" && checkbox) {
      checkbox.disabled = true;
    }
    if (checkbox && partial) {
      checkbox.indeterminate = true;
    }
    checkbox?.addEventListener("change", (event) => {
      const next = bannerSelectionSet();
      if (event.target.checked) {
        for (const value of record.momBannerIds || []) {
          next.add(String(value));
        }
      } else {
        for (const value of record.momBannerIds || []) {
          next.delete(String(value));
        }
      }
      state.gachaBannerCatalog.activeBannerIds = [...next].map((item) => Number(item)).sort((a, b) => a - b);
      renderGachaBannerManager();
    });

    els.bannerList.append(row);
  }
}

function currentEventGroupPayload() {
  return state.eventCatalog?.groups?.[state.eventSubtab] || null;
}

function eventSelectionSet() {
  return new Set((currentEventGroupPayload()?.activeIds || []).map((value) => String(value)));
}

function slugifyCategory(value) {
  return String(value || "").toLowerCase().replaceAll(" / ", "-").replaceAll(" ", "-");
}

function isGroupedChambersView() {
  return state.eventSubtab === "event_quests" && state.eventCategoryFilter === "chambers-of-dusk";
}

function selectedCountForEventIds(ids, selectedIds) {
  return (ids || []).filter((value) => selectedIds.has(String(value))).length;
}

function buildChambersDisplayRecords(records, selectedIds) {
  const grouped = new Map();
  for (const record of records) {
    const key = record.characterLabel || record.label || String(record.id);
    const existing = grouped.get(key) || {
      id: `chambers:${key}`,
      ids: [],
      label: `Chambers of Dusk: ${key}`,
      detail: "",
      group: record.group || "",
      category: "Chambers of Dusk",
      familyLabel: "Chambers of Dusk",
      characterLabel: key,
      tags: ["family Chambers of Dusk"],
      members: [],
    };
    existing.ids.push(record.id);
    existing.members.push(record);
    grouped.set(key, existing);
  }

  const difficultyRank = { Easy: 0, Normal: 1, Hard: 2, Master: 3 };
  const difficultyFromLabel = (label) => {
    const match = String(label || "").match(/Chambers of Dusk:\s*([^ -]+)/i);
    return match ? match[1] : "";
  };

  const displayRecords = [...grouped.values()].map((group) => {
    group.members.sort((a, b) => {
      const aDifficulty = difficultyFromLabel(a.label);
      const bDifficulty = difficultyFromLabel(b.label);
      return (difficultyRank[aDifficulty] ?? 99) - (difficultyRank[bDifficulty] ?? 99);
    });
    group.ids = group.members.map((member) => member.id);
    const selectedCount = selectedCountForEventIds(group.ids, selectedIds);
    const difficultyLabels = group.members
      .map((member) => difficultyFromLabel(member.label))
      .filter(Boolean);
    const uniqueDifficulties = [...new Set(difficultyLabels)];
    group.detail = `${group.ids.length} chapter${group.ids.length === 1 ? "" : "s"} · ${uniqueDifficulties.join(", ")}`;
    group.selectionCount = selectedCount;
    group.isFullySelected = selectedCount === group.ids.length;
    group.isPartiallySelected = selectedCount > 0 && selectedCount < group.ids.length;
    return group;
  });

  return displayRecords.sort((a, b) => a.label.localeCompare(b.label));
}

function currentFilteredEventRecords(payload, selectedIds) {
  const records = payload.records || [];
  const byCategory = records.filter((record) => {
    if (state.eventSubtab !== "event_quests" || state.eventCategoryFilter === "all") return true;
    return slugifyCategory(record.category) === state.eventCategoryFilter;
  });
  const search = state.eventSearch.trim().toLowerCase();
  const searchMatches = (record) => {
    if (!search) return true;
    const haystack = `${record.label || ""} ${record.detail || ""} ${record.group || ""} ${record.id || ""} ${record.category || ""} ${(record.tags || []).join(" ")}`.toLowerCase();
    return haystack.includes(search);
  };

  if (isGroupedChambersView()) {
    const grouped = buildChambersDisplayRecords(byCategory, selectedIds);
    const filteredBase = grouped.filter((record) => {
      if (!search) return true;
      const memberHaystack = (record.members || []).map((member) => `${member.label || ""} ${member.detail || ""}`).join(" ");
      const haystack = `${record.label || ""} ${record.detail || ""} ${record.group || ""} ${record.characterLabel || ""} ${record.category || ""} ${(record.tags || []).join(" ")} ${memberHaystack}`.toLowerCase();
      return haystack.includes(search);
    });
    return state.eventShowSelectedOnly
      ? filteredBase.filter((record) => record.isFullySelected || record.isPartiallySelected)
      : filteredBase;
  }

  const filteredBase = byCategory.filter(searchMatches);
  return state.eventShowSelectedOnly
    ? filteredBase.filter((record) => selectedIds.has(String(record.id)))
    : filteredBase;
}

function isSelectableEventRecord(record) {
  return record?.isSelectable !== false;
}

function renderEventSelector() {
  const payload = currentEventGroupPayload();
  if (!payload) {
    els.eventsStatus.textContent = "Event catalog unavailable.";
    els.eventsList.innerHTML = `<div class="empty-state">No event catalog loaded.</div>`;
    els.eventsAppliedSummary.textContent = "No active event selection data available.";
    els.eventsAppliedList.innerHTML = "";
    els.eventsCategoryNav.innerHTML = "";
    return;
  }

  const selectedIds = eventSelectionSet();
  const records = payload.records || [];
  const filtered = currentFilteredEventRecords(payload, selectedIds);
  const applied = isGroupedChambersView()
    ? filtered.filter((record) => record.isFullySelected)
    : records.filter((record) => selectedIds.has(String(record.id)));

  els.eventsEventTab.classList.toggle("active", state.eventSubtab === "event_quests");
  els.eventsSideTab.classList.toggle("active", state.eventSubtab === "side_story_quests");
  els.eventsSelectedToggle.classList.toggle("active", state.eventShowSelectedOnly);
  els.eventsSelectedToggle.textContent = state.eventShowSelectedOnly ? "Show All" : "Selected Only";
  els.eventsCategoryNav.innerHTML = "";
  els.eventsCategoryNav.classList.toggle("hidden", state.eventSubtab !== "event_quests");
  if (state.eventSubtab === "event_quests") {
    const categories = payload.categories || [];
    const allButton = document.createElement("button");
    allButton.className = "ghost-button";
    allButton.textContent = `All (${records.length})`;
    allButton.classList.toggle("active", state.eventCategoryFilter === "all");
    allButton.addEventListener("click", () => {
      state.eventCategoryFilter = "all";
      renderEventSelector();
    });
    els.eventsCategoryNav.append(allButton);
    for (const category of categories) {
      const button = document.createElement("button");
      button.className = "ghost-button";
      button.textContent = `${category.label} (${category.count})`;
      button.classList.toggle("active", state.eventCategoryFilter === category.id);
      button.addEventListener("click", () => {
        state.eventCategoryFilter = category.id;
        renderEventSelector();
      });
      els.eventsCategoryNav.append(button);
    }
  }
  const activeCategoryLabel = (payload.categories || []).find((category) => category.id === state.eventCategoryFilter)?.label || "All";
  els.eventsStatus.textContent =
    state.eventSubtab === "event_quests" && state.eventCategoryFilter !== "all"
      ? `Editing ${payload.currentPath}. Showing ${filtered.length} of ${records.length} ${payload.label.toLowerCase()} from ${payload.sourcePath}, filtered to ${activeCategoryLabel}.`
      : `Editing ${payload.currentPath}. Showing ${filtered.length} of ${records.length} ${payload.label.toLowerCase()} from ${payload.sourcePath}.`;
  renderSelectionStats(els.eventsSummaryStats, [
    { label: "Selected", value: formatNumber(applied.length) },
    { label: "Visible", value: formatNumber(filtered.length) },
    { label: "Total", value: formatNumber(isGroupedChambersView() ? buildChambersDisplayRecords(records.filter((record) => slugifyCategory(record.category) === "chambers-of-dusk"), selectedIds).length : records.length) },
  ]);
  els.eventsAppliedSummary.textContent =
    applied.length > 0
      ? `${applied.length} ${isGroupedChambersView() ? "chambers set" : `${payload.label.toLowerCase()} entr`}${applied.length === 1 ? " is" : "s are"} selected and will be written into lunar-tear master data on save.`
      : isGroupedChambersView()
      ? "No chambers sets are selected right now."
      : `No ${payload.label.toLowerCase()} entries are selected right now.`;
  renderAppliedItems(els.eventsAppliedList, applied, "Remove", (record) => {
    const next = eventSelectionSet();
    for (const value of record.ids || [record.id]) {
      next.delete(String(value));
    }
    payload.activeIds = [...next].map((value) => Number(value)).sort((a, b) => a - b);
    renderEventSelector();
  });

  els.eventsList.innerHTML = "";
  if (!filtered.length) {
    els.eventsList.innerHTML = `<div class="empty-state">${state.eventShowSelectedOnly ? "No selected events match the current search." : "No events match the current search."}</div>`;
    return;
  }

  for (const record of filtered) {
    const row = document.createElement("label");
    row.className = "banner-row";
    const ids = record.ids || [record.id];
    const selectedCount = selectedCountForEventIds(ids, selectedIds);
    const checked = selectedCount === ids.length;
    const detailText = isGroupedChambersView()
      ? [record.detail, ...(record.members || []).map((member) => member.label)].filter(Boolean).join(" · ")
      : [record.detail, `id ${record.id}`, ...(record.tags || [])].filter(Boolean).join(" · ");
    row.innerHTML = `
      <input type="checkbox" class="banner-checkbox" data-event-id="${record.id}" ${checked ? "checked" : ""}>
      <div class="banner-copy">
        <div class="banner-title-row">
          <span class="banner-title">${record.label}</span>
          ${record.group ? `<span class="banner-group">${record.group}</span>` : ""}
          ${record.category ? `<span class="banner-group">${record.category}</span>` : ""}
          ${record.familyLabel ? `<span class="banner-group">${record.familyLabel}</span>` : ""}
          ${record.characterLabel ? `<span class="banner-group">${record.characterLabel}</span>` : ""}
        </div>
        <div class="banner-detail">${detailText}</div>
      </div>
    `;
    const checkbox = row.querySelector(".banner-checkbox");
    if (checkbox && !isSelectableEventRecord(record)) {
      checkbox.disabled = true;
    }
    if (checkbox && selectedCount > 0 && selectedCount < ids.length) {
      checkbox.indeterminate = true;
    }
    checkbox?.addEventListener("change", (event) => {
      if (!isSelectableEventRecord(record)) return;
      const next = eventSelectionSet();
      if (event.target.checked) {
        for (const value of ids) {
          next.add(String(value));
        }
      } else {
        for (const value of ids) {
          next.delete(String(value));
        }
      }
      payload.activeIds = [...next].map((value) => Number(value)).sort((a, b) => a - b);
      renderEventSelector();
    });
    if (record.selectionReason) {
      const detail = row.querySelector(".banner-detail");
      if (detail) {
        detail.insertAdjacentHTML("beforeend", ` · ${record.selectionReason}`);
      }
    }
    els.eventsList.append(row);
  }
}

function renderUsers() {
  els.userList.innerHTML = "";
  els.usersEmpty.classList.toggle("hidden", state.users.length > 0);

  for (const user of state.users) {
    const button = document.createElement("button");
    button.className = "user-card";
    if (String(user.userId) === String(state.selectedUserId)) {
      button.classList.add("active");
    }
    const name = user.name || `User ${user.userId}`;
    button.innerHTML = `
      <div class="title">${name}</div>
      <div class="details">ID ${user.userId} · Lv ${user.level} · Gems ${formatNumber((user.paidGem || 0) + (user.freeGem || 0))}</div>
      <div class="details mono">${user.uuid || "No UUID"}</div>
    `;
    button.addEventListener("click", () => selectUser(user.userId));
    els.userList.append(button);
  }
}

function setView(view) {
  state.currentView = view;
  const isHome = view === "home";
  const isBundles = view === "bundles";
  const isBanner = view === "banners";
  const isEvents = view === "events";
  const isEditor = view === "editor";
  els.homeView.classList.toggle("hidden", !isHome);
  els.bundlesView.classList.toggle("hidden", !isBundles);
  els.bannerView.classList.toggle("hidden", !isBanner);
  els.eventsView.classList.toggle("hidden", !isEvents);
  els.editorView.classList.toggle("hidden", !isEditor);
  els.homeButton.classList.toggle("active", isHome);
  els.bundlesButton.classList.toggle("active", isBundles);
  els.bannerButton.classList.toggle("active", isBanner);
  els.eventsButton.classList.toggle("active", isEvents);
}

function deckThumbMarkup(slot) {
  const costume = slot?.costume;
  const weapon = slot?.weapon;
  return `
    <div class="homepage-thumb">
      <div class="homepage-thumb-portrait">
        ${costume?.imageUrl ? `<img src="${costume.imageUrl}" alt="${costume.name || "Character"}">` : ""}
        ${weapon?.imageUrl ? `<div class="homepage-thumb-weapon"><img src="${weapon.imageUrl}" alt="${weapon.name || "Weapon"}"></div>` : ""}
      </div>
      <div class="homepage-thumb-label">${costume?.characterName || costume?.name || "Empty"}</div>
    </div>
  `;
}

function renderHomepageUsers() {
  els.homepageUsers.innerHTML = "";
  if (!state.homepageUsers.length) {
    els.homepageUsers.innerHTML = `<div class="empty-state">No users found in the current database.</div>`;
    return;
  }

  for (const item of state.homepageUsers) {
    const user = item.user;
    const deckPayload = item.deck;
    const decks = deckPayload?.decks || [];
    const selectedKey = item.selectedDeckKey || deckPayload?.selectedDeckKey || "";
    const deck = deckPayload?.deck;
    const slots = deckPayload?.slots || [];

    const row = document.createElement("section");
    row.className = "homepage-user-row";
    row.innerHTML = `
      <button class="homepage-user-summary" type="button" data-user-id="${user.userId}">
        <div class="homepage-user-name">${user.name || `User ${user.userId}`}</div>
        <div class="homepage-user-note">${user.message || "No note saved."}</div>
        <div class="homepage-user-meta">Level ${user.level} · ${formatNumber(user.completedQuests || 0)} quests completed</div>
      </button>
      <div class="homepage-user-deck">
        <label class="hero-deck-label" for="homepage-deck-${user.userId}">Deck View</label>
        <select id="homepage-deck-${user.userId}" class="table-select homepage-deck-select" data-user-id="${user.userId}">
          ${decks.map((option) => `<option value="${option.key}"${option.key === selectedKey ? " selected" : ""}>${option.label || option.name}</option>`).join("")}
        </select>
      </div>
      <div class="homepage-user-slots">
        ${slots.map((slot) => deckThumbMarkup(slot)).join("")}
      </div>
    `;
    const summaryButton = row.querySelector(".homepage-user-summary");
    summaryButton?.addEventListener("click", () => selectUser(user.userId));

    const deckSelect = row.querySelector(".homepage-deck-select");
    deckSelect?.addEventListener("change", async (event) => {
      const nextDeck = await fetchActiveDeckForUser(user.userId, event.target.value);
      const target = state.homepageUsers.find((entry) => String(entry.user.userId) === String(user.userId));
      if (target) {
        target.deck = nextDeck;
        target.selectedDeckKey = nextDeck.selectedDeckKey;
      }
      renderHomepageUsers();
    });

    els.homepageUsers.append(row);
  }
}

function populateTableSelect() {
  const panel = els.tableSelectPanel;
  panel.innerHTML = "";
  const groups = state.overview.tableGroups || [];
  const search = state.tableSearch.trim().toLowerCase();
  let firstVisibleValue = "";
  let allValues = [];

  for (const group of groups) {
    const matchingTables = group.tables.filter((table) => {
      if (!state.overview.schema[table]) return false;
      if (!search) return true;
      const label = tableLabel(table);
      return (
        table.toLowerCase().includes(search) ||
        label.toLowerCase().includes(search) ||
        group.label.toLowerCase().includes(search)
      );
    });
    if (matchingTables.length === 0) continue;

    const groupLabel = document.createElement("div");
    groupLabel.className = "table-select-group-label";
    groupLabel.textContent = group.label;
    panel.append(groupLabel);

    for (const table of matchingTables) {
      const btn = document.createElement("button");
      btn.className = "table-select-option";
      btn.type = "button";
      btn.dataset.value = table;
      btn.textContent = tableLabel(table);
      if (table === state.selectedTable) btn.classList.add("active");
      btn.addEventListener("click", () => selectTableFromPicker(table));
      panel.append(btn);
      if (!firstVisibleValue) firstVisibleValue = table;
      allValues.push(table);
    }
  }

  if (!allValues.includes(state.selectedTable)) {
    state.selectedTable = firstVisibleValue || "";
  }
  // Sync trigger label
  els.tableSelectLabel.textContent = state.selectedTable ? tableLabel(state.selectedTable) : "Select table…";
}

function selectTableFromPicker(table) {
  state.selectedTable = table;
  els.tableSelectLabel.textContent = tableLabel(table);
  // Update active state in panel
  for (const btn of els.tableSelectPanel.querySelectorAll(".table-select-option")) {
    btn.classList.toggle("active", btn.dataset.value === table);
  }
  closeTableSelectPanel();
  if (state.selectedUserId || !currentTableIsUserScoped()) {
    loadTable(table);
  }
}

function openTableSelectPanel() {
  els.tableSelectPanel.classList.remove("hidden");
  els.tableSelectTrigger.setAttribute("aria-expanded", "true");
  // Scroll active item into view
  const active = els.tableSelectPanel.querySelector(".table-select-option.active");
  active?.scrollIntoView({ block: "nearest" });
}

function closeTableSelectPanel() {
  els.tableSelectPanel.classList.add("hidden");
  els.tableSelectTrigger.setAttribute("aria-expanded", "false");
}

function renderUserSummary(user) {
  state.selectedUserId = user.userId;
  els.heroTitle.textContent = user.name || `User ${user.userId}`;
  els.heroSubtitle.textContent = user.message || "No profile message saved.";
  els.userSummaryPanel.classList.remove("hidden");
  els.tablePanel.classList.remove("hidden");
  els.userSummaryGrid.innerHTML = "";

  const cards = [
    ["User ID", user.userId],
    ["Player ID", user.playerId],
    ["Level", user.level],
    ["Experience", user.exp],
    ["Paid Gems", user.paidGem],
    ["Free Gems", user.freeGem],
    ["Latest Version", user.latestVersion],
  ];

  for (const [label, value] of cards) {
    const card = document.createElement("div");
    card.className = "summary-card";
    card.innerHTML = `<span class="muted">${label}</span><span class="value">${formatNumber(value)}</span>`;
    els.userSummaryGrid.append(card);
  }
}

function renderActiveDeck() {
  const payload = state.activeDeck;
  const deck = payload?.deck;
  const decks = payload?.decks || [];
  const slots = payload?.slots || [];
  const hasDeckChoices = decks.length > 1;

  els.heroDeckToolbar.classList.toggle("hidden", !hasDeckChoices);
  if (hasDeckChoices) {
    els.deckSelect.innerHTML = "";
    for (const option of decks) {
      const el = document.createElement("option");
      el.value = option.key;
      el.textContent = option.label || option.name || option.key;
      if (option.key === state.selectedDeckKey) {
        el.selected = true;
      }
      els.deckSelect.append(el);
    }
    if (!state.selectedDeckKey && decks[0]) {
      state.selectedDeckKey = decks[0].key;
      els.deckSelect.value = state.selectedDeckKey;
    }
  } else {
    els.deckSelect.innerHTML = "";
  }

  els.deckVisual.innerHTML = "";
  const hasVisualSlot = slots.some((slot) => slot?.costume || slot?.weapon || slot?.companion);
  els.deckVisual.classList.toggle("hidden", !hasVisualSlot);
  els.heroFallbackImage.classList.toggle("hidden", hasVisualSlot);

  if (!hasVisualSlot) {
    return;
  }

  for (const slot of slots) {
    const card = document.createElement("article");
    card.className = "deck-slot-card";

    const costume = slot.costume;
    const weapon = slot.weapon;
    const companion = slot.companion;
    const subWeapons = slot.subWeapons || [];

    const art = document.createElement("div");
    art.className = "deck-slot-portrait-box";
    if (costume?.imageUrl) {
      const img = document.createElement("img");
      img.src = costume.imageUrl;
      img.alt = costume.name || "Deck character";
      img.className = "deck-slot-character";
      art.append(img);
    } else {
      art.classList.add("deck-slot-empty");
    }

    if (companion) {
      const companionChip = document.createElement("div");
      companionChip.className = "deck-companion-chip";
      if (companion.imageUrl) {
        const img = document.createElement("img");
        img.src = companion.imageUrl;
        img.alt = companion.name || "Companion";
        companionChip.append(img);
      }
      const label = document.createElement("span");
      label.textContent = companion.name || "Companion";
      companionChip.append(label);
      art.append(companionChip);
    }

    if (weapon) {
      const weaponWrap = document.createElement("div");
      weaponWrap.className = "deck-weapon-overlay";
      if (weapon.imageUrl) {
        const img = document.createElement("img");
        img.src = weapon.imageUrl;
        img.alt = weapon.name || "Weapon";
        weaponWrap.append(img);
      }
      if (subWeapons.length) {
        const stack = document.createElement("div");
        stack.className = "deck-subweapon-stack";
        for (const subWeapon of subWeapons.slice(0, 2)) {
          const badge = document.createElement("div");
          badge.className = "deck-subweapon-badge";
          if (subWeapon.imageUrl) {
            const img = document.createElement("img");
            img.src = subWeapon.imageUrl;
            img.alt = subWeapon.name || "Sub weapon";
            badge.append(img);
          }
          stack.append(badge);
        }
        weaponWrap.append(stack);
      }
      art.append(weaponWrap);
    }

    const meta = document.createElement("div");
    meta.className = "deck-slot-info-box";
    const subWeaponText = subWeapons.length
      ? `<div class="deck-slot-subdetail">Sub Weapons: ${subWeapons.map((subWeapon) => subWeapon.name).join(", ")}</div>`
      : "";
    const companionText = companion?.name
      ? `<div class="deck-slot-subdetail">Companion: ${companion.name}</div>`
      : "";
    meta.innerHTML = `
      <div class="deck-slot-kicker">Deck ${deck?.displayIndex || 1} · Slot ${slot.slot || "?"}</div>
      <div class="deck-slot-name">${costume?.name || "Empty Slot"}</div>
      <div class="deck-slot-detail">${costume?.characterName || ""}${weapon?.name ? ` · ${weapon.name}` : ""}</div>
      ${subWeaponText}
      ${companionText}
    `;

    card.append(art, meta);
    els.deckVisual.append(card);
  }
}

function currentTableIsUserScoped() {
  const schema = state.overview?.schema?.[state.selectedTable];
  return Boolean(schema?.columns?.some((column) => column.name === "user_id"));
}

function markEditorInteraction(holdMs = 6000) {
  state.editorInteractionUntil = Date.now() + holdMs;
}

function editorInteractionIsActive() {
  if (Date.now() < state.editorInteractionUntil) {
    return true;
  }
  const activeElement = document.activeElement;
  return Boolean(activeElement && els.editorTable.contains(activeElement));
}

function selectedDeckEntities() {
  const selected = {
    weaponUuids: new Set(),
    costumeUuids: new Set(),
    companionUuids: new Set(),
    partUuids: new Set(),
    thoughtUuids: new Set(),
    characterIds: new Set(),
    deckCharacterUuids: new Set(),
    deckType: String(state.activeDeck?.deck?.deckType ?? ""),
    deckNumber: String(state.activeDeck?.deck?.deckNumber ?? ""),
  };

  for (const slot of state.activeDeck?.slots || []) {
    const deckCharacterUuid = String(slot?.deckCharacterUuid || "").trim();
    if (deckCharacterUuid) {
      selected.deckCharacterUuids.add(deckCharacterUuid);
    }

    const costumeUuid = String(slot?.costume?.userCostumeUuid || "").trim();
    if (costumeUuid) {
      selected.costumeUuids.add(costumeUuid);
    }

    const characterId = String(slot?.costume?.characterId || "").trim();
    if (characterId && characterId !== "0") {
      selected.characterIds.add(characterId);
    }

    const companionUuid = String(slot?.companion?.userCompanionUuid || "").trim();
    if (companionUuid) {
      selected.companionUuids.add(companionUuid);
    }

    const thoughtUuid = String(slot?.thought?.userThoughtUuid || "").trim();
    if (thoughtUuid) {
      selected.thoughtUuids.add(thoughtUuid);
    }

    const mainWeaponUuid = String(slot?.weapon?.userWeaponUuid || "").trim();
    if (mainWeaponUuid) {
      selected.weaponUuids.add(mainWeaponUuid);
    }

    for (const subWeapon of slot?.subWeapons || []) {
      const subWeaponUuid = String(subWeapon?.userWeaponUuid || "").trim();
      if (subWeaponUuid) {
        selected.weaponUuids.add(subWeaponUuid);
      }
    }

    for (const part of slot?.parts || []) {
      const partUuid = String(part?.userPartsUuid || "").trim();
      if (partUuid) {
        selected.partUuids.add(partUuid);
      }
    }
  }

  return selected;
}

function deckTableFilterConfig(table = state.selectedTable) {
  const configs = {
    user_weapons: {
      rowKey: "user_weapon_uuid",
      noun: "weapons",
      placeholder: "Search weapons in this user's inventory",
      matchesEquipped: (row, selected) => selected.weaponUuids.has(String(row.user_weapon_uuid || "").trim()),
    },
    user_costumes: {
      rowKey: "user_costume_uuid",
      noun: "costumes",
      placeholder: "Search costumes in this user's inventory",
      matchesEquipped: (row, selected) => selected.costumeUuids.has(String(row.user_costume_uuid || "").trim()),
    },
    user_companions: {
      rowKey: "user_companion_uuid",
      noun: "companions",
      placeholder: "Search companions in this user's inventory",
      matchesEquipped: (row, selected) => selected.companionUuids.has(String(row.user_companion_uuid || "").trim()),
    },
    user_parts: {
      rowKey: "user_parts_uuid",
      noun: "parts",
      placeholder: "Search parts in this user's inventory",
      matchesEquipped: (row, selected) => selected.partUuids.has(String(row.user_parts_uuid || "").trim()),
    },
    user_thoughts: {
      rowKey: "user_thought_uuid",
      noun: "thoughts",
      placeholder: "Search thoughts in this user's inventory",
      matchesEquipped: (row, selected) => selected.thoughtUuids.has(String(row.user_thought_uuid || "").trim()),
    },
    user_characters: {
      rowKey: "character_id",
      noun: "characters",
      placeholder: "Search characters tied to this deck",
      matchesEquipped: (row, selected) => selected.characterIds.has(String(row.character_id || "").trim()),
    },
    user_deck_characters: {
      rowKey: "user_deck_character_uuid",
      noun: "deck characters",
      placeholder: "Search deck character rows",
      matchesEquipped: (row, selected) => selected.deckCharacterUuids.has(String(row.user_deck_character_uuid || "").trim()),
    },
    user_deck_sub_weapons: {
      rowKey: "user_weapon_uuid",
      noun: "deck sub weapons",
      placeholder: "Search equipped sub-weapon rows",
      matchesEquipped: (row, selected) => selected.weaponUuids.has(String(row.user_weapon_uuid || "").trim()),
    },
    user_deck_parts: {
      rowKey: "user_parts_uuid",
      noun: "deck parts",
      placeholder: "Search equipped parts rows",
      matchesEquipped: (row, selected) => selected.partUuids.has(String(row.user_parts_uuid || "").trim()),
    },
    user_decks: {
      rowKey: "deck_type",
      noun: "decks",
      placeholder: "Search deck rows",
      matchesEquipped: (row, selected) =>
        String(row.deck_type || "").trim() === selected.deckType &&
        String(row.user_deck_number || "").trim() === selected.deckNumber,
    },
  };
  return configs[table] || null;
}

function rowSearchText(row, annotations) {
  const rowValues = Object.values(row || {}).map((value) => String(value ?? ""));
  const annotationValues = Object.values(annotations || {}).flatMap((annotation) => {
    if (!annotation || typeof annotation !== "object") {
      return [];
    }
    return [
      annotation.label,
      annotation.detail,
      annotation.value,
      annotation.group,
    ].map((value) => String(value ?? ""));
  });
  return [...rowValues, ...annotationValues].join(" ").toLowerCase();
}

function filteredTableEntries() {
  const baseEntries = state.tableRows.map((row, index) => ({
    row,
    annotations: state.tableAnnotations[index] || {},
  }));
  const filterConfig = deckTableFilterConfig();
  if (!filterConfig) {
    return baseEntries;
  }

  const query = state.weaponInventorySearch.trim().toLowerCase();
  const selected = selectedDeckEntities();
  return baseEntries.filter(({ row, annotations }) => {
    if (state.weaponEquippedOnly && !filterConfig.matchesEquipped(row, selected)) {
      return false;
    }
    if (!query) {
      return true;
    }
    return rowSearchText(row, annotations).includes(query);
  });
}

function renderWeaponTableFilters(filteredCount) {
  const filterConfig = deckTableFilterConfig();
  const isVisible = Boolean(filterConfig);
  els.weaponTableFilters.classList.toggle("hidden", !isVisible);
  if (!isVisible) {
    return;
  }

  els.weaponTableSearch.value = state.weaponInventorySearch;
  els.weaponEquippedOnly.checked = state.weaponEquippedOnly;

  const noun = filterConfig.noun || "rows";
  const singularNoun = noun.endsWith("s") ? noun.slice(0, -1) : noun;
  const countLabel = filteredCount === 1 ? `${singularNoun} row` : `${noun} rows`;
  const placeholder = state.weaponEquippedOnly
    ? `Search equipped ${noun} in deck ${state.selectedDeckKey || "1:1"}`
    : filterConfig.placeholder;
  els.weaponTableSearch.placeholder = placeholder;
  els.weaponTableSearch.title = `${filteredCount} ${countLabel} currently shown`;
}

function defaultValueForColumn(column) {
  if (column.name === "user_id") return state.selectedUserId;
  if (column.name === "level") return "1";
  if (cachedLookupOptions(column.name).length > 0) return "";
  if (column.type.toUpperCase().includes("INT")) return "0";
  return "";
}

function generateScopedUuid() {
  const userId = Number.parseInt(String(state.selectedUserId || "0"), 10) || 0;
  const userHex = userId.toString(16).padStart(8, "0").slice(-8);
  let base = "";
  if (globalThis.crypto?.randomUUID) {
    base = globalThis.crypto.randomUUID();
  } else {
    const bytes = new Uint8Array(16);
    if (globalThis.crypto?.getRandomValues) {
      globalThis.crypto.getRandomValues(bytes);
    } else {
      for (let index = 0; index < bytes.length; index += 1) {
        bytes[index] = Math.floor(Math.random() * 256);
      }
    }
    bytes[6] = (bytes[6] & 0x0f) | 0x40;
    bytes[8] = (bytes[8] & 0x3f) | 0x80;
    const hex = [...bytes].map((value) => value.toString(16).padStart(2, "0")).join("");
    base = `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  }
  return `${userHex}${base.slice(8)}`;
}

function buildBlankRow() {
  const row = {};
  for (const column of state.tableSchema.columns) {
    row[column.name] = defaultValueForColumn(column);
  }
  if (currentTableIsUserScoped()) {
    for (const keyName of state.tableSchema.primaryKey) {
      if (keyName.endsWith("_uuid")) {
        row[keyName] = generateScopedUuid();
      }
    }
  }
  if (state.selectedTable === "user_costumes") {
    row.limit_break_count = "0";
    row.level = "1";
    row.exp = "0";
    row.headup_display_view_id = "1";
    row.acquisition_datetime = String(Date.now());
    row.awaken_count = "0";
    row.latest_version = "0";
  }
  if (state.selectedTable === "user_companions") {
    const companionOptions = cachedLookupOptions("companion_id");
    row.headup_display_view_id = "1";
    row.level = "1";
    row.acquisition_datetime = String(Date.now());
    row.latest_version = "0";
    if (!String(row.companion_id ?? "").trim() && companionOptions.length > 0) {
      row.companion_id = String(companionOptions[0].value);
    }
  }
  row.__isDraft = true;
  return row;
}

async function ensureLookupOptions(column) {
  const scope = state.selectedUserId || "global";
  const cacheKey = `${scope}:${column}`;
  if (state.lookupOptions[cacheKey]) {
    return state.lookupOptions[cacheKey];
  }
  const params = new URLSearchParams();
  if (state.selectedUserId) {
    params.set("user_id", state.selectedUserId);
  }
  const suffix = params.toString() ? `?${params.toString()}` : "";
  // API returns the options array directly (not wrapped in {options:[]})
  // Guard against null in case the server returns null for unknown columns
  const data = await fetchJSON(`/api/lookups/${column}${suffix}`);
  state.lookupOptions[cacheKey] = Array.isArray(data) ? data : [];
  return state.lookupOptions[cacheKey];
}

function cachedLookupOptions(column) {
  const scope = state.selectedUserId || "global";
  return state.lookupOptions[`${scope}:${column}`] || [];
}

function formatLookupOptionText(option) {
  const detail = option?.detail ? ` · ${option.detail}` : "";
  return option?.label ? `${option.label} (${option.value})${detail}` : String(option?.value ?? "");
}

function setControlValue(control, value) {
  if (!control) return;
  control.value = value;
  if (control.type === "hidden" && control.dataset.column) {
    const input = control.parentElement?.querySelector(".lookup-combobox-input");
    if (input) {
      const selected = cachedLookupOptions(control.dataset.column).find(
        (option) => String(option.value) === String(value),
      );
      input.value = selected ? formatLookupOptionText(selected) : String(value ?? "");
    }
    control.dispatchEvent(new Event("change", { bubbles: true }));
  }
}

function shouldUseLookupSelect(column, row) {
  if (row.__isDraft && state.tableSchema.primaryKey.includes(column.name) && column.name.endsWith("_uuid")) {
    return false;
  }
  return cachedLookupOptions(column.name).length > 0;
}

function applyUserCostumeDraftDefaults(tr) {
  if (state.selectedTable !== "user_costumes") return;

  const getControl = (name) => tr.querySelector(`[data-column="${name}"]`);
  const setIfBlankOrZero = (name, value) => {
    const control = getControl(name);
    if (!control) return;
    const current = String(control.value ?? "").trim();
    if (current === "" || current === "0") {
      control.value = value;
    }
  };

  const uuidControl = getControl("user_costume_uuid");
  if (uuidControl && !String(uuidControl.value || "").trim()) {
    uuidControl.value = generateScopedUuid();
  }

  const costumeControl = getControl("costume_id");
  if (costumeControl && !String(costumeControl.value || "").trim()) {
    const [firstCostume] = cachedLookupOptions("costume_id");
    if (firstCostume) {
      setControlValue(costumeControl, String(firstCostume.value));
    }
  }

  setIfBlankOrZero("limit_break_count", "0");
  setIfBlankOrZero("level", "1");
  setIfBlankOrZero("exp", "0");
  setIfBlankOrZero("headup_display_view_id", "1");
  setIfBlankOrZero("awaken_count", "0");
  setIfBlankOrZero("latest_version", "0");

  const acquisitionControl = getControl("acquisition_datetime");
  if (acquisitionControl) {
    const current = String(acquisitionControl.value ?? "").trim();
    if (current === "" || current === "0") {
      acquisitionControl.value = String(Date.now());
    }
  }
}

function applyUserCompanionDraftDefaults(tr) {
  if (state.selectedTable !== "user_companions") return;

  const getControl = (name) => tr.querySelector(`[data-column="${name}"]`);
  const setIfBlankOrZero = (name, value) => {
    const control = getControl(name);
    if (!control) return;
    const current = String(control.value ?? "").trim();
    if (current === "" || current === "0") {
      control.value = value;
    }
  };

  const uuidControl = getControl("user_companion_uuid");
  if (uuidControl && !String(uuidControl.value || "").trim()) {
    uuidControl.value = generateScopedUuid();
  }

  const companionControl = getControl("companion_id");
  if (companionControl && !String(companionControl.value || "").trim()) {
    const [firstCompanion] = cachedLookupOptions("companion_id");
    if (firstCompanion) {
      setControlValue(companionControl, String(firstCompanion.value));
    }
  }

  setIfBlankOrZero("headup_display_view_id", "1");
  setIfBlankOrZero("level", "1");
  setIfBlankOrZero("latest_version", "0");

  const acquisitionControl = getControl("acquisition_datetime");
  if (acquisitionControl) {
    const current = String(acquisitionControl.value ?? "").trim();
    if (current === "" || current === "0") {
      acquisitionControl.value = String(Date.now());
    }
  }
}

function buildEditorControl(column, row, annotation = null) {
  const options = cachedLookupOptions(column.name);
  if (shouldUseLookupSelect(column, row)) {
    const wrapper = document.createElement("div");
    wrapper.className = "lookup-combobox";

    const currentValue = String(row[column.name] ?? "");
    const hidden = document.createElement("input");
    hidden.type = "hidden";
    hidden.dataset.column = column.name;
    hidden.value = currentValue;

    const input = document.createElement("input");
    input.className = "lookup-combobox-input";
    input.type = "text";
    input.placeholder = `Search ${column.name}`;
    input.autocomplete = "off";
    input.disabled = column.name === "user_id";

    const menu = document.createElement("div");
    menu.className = "lookup-combobox-menu hidden";

    const currentText = annotation?.label
      ? `${annotation.label} (${currentValue})${annotation.detail ? ` · ${annotation.detail}` : ""}`
      : currentValue;
    input.value = currentValue ? currentText : "";

    const closeMenu = () => {
      menu.classList.add("hidden");
    };

    const renderMenu = (filterText = "") => {
      menu.innerHTML = "";
      const normalizedFilter = filterText.trim().toLowerCase();
      const groupedOptions = new Map();
      for (const option of options) {
        const haystack = `${option.label || ""} ${option.value || ""} ${option.detail || ""} ${option.group || ""}`.toLowerCase();
        if (normalizedFilter && !haystack.includes(normalizedFilter)) {
          continue;
        }
        const group = option.group || "";
        if (!groupedOptions.has(group)) {
          groupedOptions.set(group, []);
        }
        groupedOptions.get(group).push(option);
      }

      if (!groupedOptions.size) {
        const empty = document.createElement("div");
        empty.className = "lookup-combobox-empty";
        empty.textContent = "No matches";
        menu.append(empty);
        return;
      }

      for (const [group, groupOptions] of groupedOptions.entries()) {
        if (group) {
          const heading = document.createElement("div");
          heading.className = "lookup-combobox-group";
          heading.textContent = group;
          menu.append(heading);
        }
        for (const option of groupOptions) {
          const choice = document.createElement("button");
          choice.type = "button";
          choice.className = "lookup-combobox-option";
          if (option.value === hidden.value) {
            choice.classList.add("active");
          }
          choice.textContent = formatLookupOptionText(option);
          choice.addEventListener("mousedown", (event) => {
            event.preventDefault();
            setControlValue(hidden, option.value);
            closeMenu();
          });
          menu.append(choice);
        }
      }
    };

    input.addEventListener("focus", () => {
      renderMenu(input.value === currentText ? "" : input.value);
      menu.classList.remove("hidden");
    });
    input.addEventListener("input", () => {
      renderMenu(input.value);
      menu.classList.remove("hidden");
    });
    input.addEventListener("blur", () => {
      window.setTimeout(() => {
        closeMenu();
        if (!hidden.value) {
          input.value = "";
          return;
        }
        const selected = options.find((option) => option.value === hidden.value);
        if (selected) {
          input.value = formatLookupOptionText(selected);
        }
      }, 120);
    });

    wrapper.append(input, hidden, menu);
    return wrapper;
  }

  const input = document.createElement("input");
  input.className = "table-input";
  input.value = row[column.name] ?? "";
  input.dataset.column = column.name;
  input.disabled = column.name === "user_id";
  return input;
}

function renderTable() {
  const { columns, primaryKey } = state.tableSchema;
  const visibleEntries = filteredTableEntries();
  els.tableHeading.textContent = tableLabel(state.selectedTable);
  const scopeLabel = currentTableIsUserScoped()
    ? `filtered to user ${state.selectedUserId ?? "all"}`
    : "global table";
  const category = state.overview.tableGroups.find((group) => group.tables.includes(state.selectedTable))?.label || "Uncategorized";
  const rowCountLabel = visibleEntries.length === state.tableRows.length
    ? `${state.tableRows.length} row(s)`
    : `${visibleEntries.length} of ${state.tableRows.length} row(s)`;
  els.tableMeta.textContent = `Category: ${category} · Primary key: ${primaryKey.join(", ") || "none"} · ${rowCountLabel} · ${scopeLabel}`;
  renderWeaponTableFilters(visibleEntries.length);
  els.editorTable.innerHTML = "";
  els.tableEmpty.classList.toggle("hidden", visibleEntries.length > 0);
  els.tableEmpty.textContent = deckTableFilterConfig()
    ? "No rows match the current selected-deck filters."
    : "No rows in this table for the selected user.";

  const thead = document.createElement("thead");
  const headRow = document.createElement("tr");
  for (const column of columns) {
    const th = document.createElement("th");
    th.textContent = column.name;
    headRow.append(th);
  }
  const actionsTh = document.createElement("th");
  actionsTh.textContent = "Actions";
  headRow.append(actionsTh);
  thead.append(headRow);
  els.editorTable.append(thead);

  const tbody = document.createElement("tbody");
  for (const entry of visibleEntries) {
    tbody.append(buildTableRow(entry.row, entry.annotations));
  }
  els.editorTable.append(tbody);
}

function buildTableRow(row, annotations) {
  const tr = document.createElement("tr");
  if (row.__isDraft) {
    tr.classList.add("draft-row");
  }
  const previewColumns = {
    user_weapons: "weapon_id",
    user_costumes: "costume_id",
    user_characters: "character_id",
    user_companions: "companion_id",
    user_consumable_items: "consumable_item_id",
    user_materials: "material_id",
  };
  for (const column of state.tableSchema.columns) {
    const td = document.createElement("td");
    td.className = "cell";
    const stack = document.createElement("div");
    stack.className = "cell-stack";
    const annotation = annotations[column.name];
    const control = buildEditorControl(column, row, annotation);
    stack.append(control);

    if (annotation?.imageUrl && previewColumns[state.selectedTable] === column.name) {
      const preview = document.createElement("div");
      preview.className = "cell-asset-preview";
      const img = document.createElement("img");
      img.src = annotation.imageUrl;
      img.alt = annotation.label || "Asset preview";
      preview.append(img);
      stack.append(preview);
    }
    if (annotation) {
      const note = document.createElement("div");
      note.className = "cell-note";
      note.textContent = annotation.label;
      stack.append(note);

      if (annotation.detail) {
        const detail = document.createElement("div");
        detail.className = "cell-detail";
        detail.textContent = annotation.detail;
        stack.append(detail);
      }
    }

    td.append(stack);
    tr.append(td);
  }

  const actions = document.createElement("td");
  actions.className = "actions";

  const save = document.createElement("button");
  save.className = "primary-button";
  save.textContent = "Save";
  save.addEventListener("click", async () => {
    markEditorInteraction(10000);
    const originalKey = Object.fromEntries(
      state.tableSchema.primaryKey.map((name) => [name, String(row[name] ?? "")]),
    );
    const payload = {};
    for (const control of tr.querySelectorAll("input, select")) {
      if (!control.dataset.column) continue;
      payload[control.dataset.column] = control.value;
    }
    const params = new URLSearchParams();
    if (state.selectedUserId && currentTableIsUserScoped()) {
      params.set("user_id", state.selectedUserId);
    }
    const suffix = params.toString() ? `?${params.toString()}` : "";
    await fetchJSON(`/api/table/${state.selectedTable}${suffix}`, {
      method: "POST",
      body: JSON.stringify({ row: payload }),
    });
    const keyChanged =
      !row.__isDraft &&
      state.tableSchema.primaryKey.some((name) => String(payload[name] ?? "") !== originalKey[name]);
    if (keyChanged) {
      await fetchJSON(`/api/table/${state.selectedTable}${suffix}`, {
        method: "DELETE",
        body: JSON.stringify({ key: originalKey }),
      });
    }
    await loadTable(state.selectedTable);
  });

  const remove = document.createElement("button");
  remove.className = "danger-button";
  remove.textContent = "Delete";
  remove.addEventListener("click", async () => {
    markEditorInteraction(10000);
    const key = {};
    for (const name of state.tableSchema.primaryKey) {
      const control = tr.querySelector(`[data-column="${name}"]`);
      key[name] = control?.value ?? "";
    }
    const params = new URLSearchParams();
    if (state.selectedUserId && currentTableIsUserScoped()) {
      params.set("user_id", state.selectedUserId);
    }
    const suffix = params.toString() ? `?${params.toString()}` : "";
    await fetchJSON(`/api/table/${state.selectedTable}${suffix}`, {
      method: "DELETE",
      body: JSON.stringify({ key }),
    });
    await loadTable(state.selectedTable);
  });

  actions.append(save, remove);
  tr.append(actions);

  if (row.__isDraft && state.selectedTable === "user_costumes") {
    const costumeControl = tr.querySelector('[data-column="costume_id"]');
    costumeControl?.addEventListener("change", () => applyUserCostumeDraftDefaults(tr));
    applyUserCostumeDraftDefaults(tr);
  }

  if (row.__isDraft && state.selectedTable === "user_companions") {
    applyUserCompanionDraftDefaults(tr);
  }

  return tr;
}

async function loadTable(table) {
  state.selectedTable = table;
  const params = new URLSearchParams();
  if (state.selectedUserId && currentTableIsUserScoped()) {
    params.set("user_id", state.selectedUserId);
  }
  const suffix = params.toString() ? `?${params.toString()}` : "";
  const data = await fetchJSON(`/api/table/${table}${suffix}`);
  state.tableRows = data.rows;
  state.tableAnnotations = data.annotations || [];
  state.tableSchema = data.schema;
  await Promise.all(state.tableSchema.columns.map((column) => ensureLookupOptions(column.name)));
  renderTable();
}

async function loadActiveDeck(userId, deckKey = "") {
  const params = new URLSearchParams();
  if (deckKey) {
    const [deckType, deckNumber] = String(deckKey).split(":");
    if (deckType) params.set("deck_type", deckType);
    if (deckNumber) params.set("deck_number", deckNumber);
  }
  const suffix = params.toString() ? `?${params.toString()}` : "";
  state.activeDeck = await fetchJSON(`/api/user/${userId}/active-deck${suffix}`);
  state.selectedDeckKey = state.activeDeck?.selectedDeckKey || "";
  renderActiveDeck();
  if (state.tableSchema && deckTableFilterConfig()) {
    renderTable();
  }
}

async function fetchActiveDeckForUser(userId, deckKey = "") {
  const params = new URLSearchParams();
  if (deckKey) {
    const [deckType, deckNumber] = String(deckKey).split(":");
    if (deckType) params.set("deck_type", deckType);
    if (deckNumber) params.set("deck_number", deckNumber);
  }
  const suffix = params.toString() ? `?${params.toString()}` : "";
  return fetchJSON(`/api/user/${userId}/active-deck${suffix}`);
}

async function loadHomepageUsers() {
  state.homepageUsers = await Promise.all(
    state.users.map(async (user) => {
      const deck = await fetchActiveDeckForUser(user.userId);
      return {
        user,
        deck,
        selectedDeckKey: deck.selectedDeckKey || "",
      };
    }),
  );
  renderHomepageUsers();
}

async function pollDeckViews() {
  if (state.deckPollingBusy || !state.users.length) return;
  if (state.currentView === "editor" && editorInteractionIsActive()) return;
  state.deckPollingBusy = true;
  try {
    if (state.currentView === "home") {
      state.homepageUsers = await Promise.all(
        state.homepageUsers.map(async (entry) => {
          const deck = await fetchActiveDeckForUser(entry.user.userId, entry.selectedDeckKey || "");
          return {
            ...entry,
            deck,
            selectedDeckKey: deck.selectedDeckKey || entry.selectedDeckKey || "",
          };
        }),
      );
      renderHomepageUsers();
      return;
    }

    if (state.currentView === "editor" && state.selectedUserId) {
      await loadActiveDeck(state.selectedUserId, state.selectedDeckKey || "");
    }
  } catch (error) {
    console.error("Deck polling failed", error);
  } finally {
    state.deckPollingBusy = false;
  }
}

async function selectUser(userId) {
  const user = await fetchJSON(`/api/user/${userId}/summary`);
  state.selectedDeckKey = "";
  await loadActiveDeck(userId);
  renderUserSummary(user);
  renderUsers();
  setView("editor");
  await loadTable(state.selectedTable || "");
}

function goHome() {
  setView("home");
  renderUsers();
}

// === Bundles view ===

const MONTH_NAMES = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];

function formatBundleMonth(key) {
  // key format: "2021-07"
  const [year, month] = key.split("-");
  const idx = parseInt(month, 10) - 1;
  return `${MONTH_NAMES[idx] || month} ${year}`;
}

function renderBundles() {
  if (!state.bundleCatalog || !state.bundleSchedule) {
    els.bundlesStatus.textContent = "Bundle data not available.";
    els.bundlesGrid.innerHTML = "";
    els.bundlesSummaryStats.innerHTML = "";
    return;
  }

  const bundles = state.bundleCatalog.bundles || [];
  const activeBundles = new Set(state.bundleSchedule.active_bundles || []);
  const unreleasedEnabled = state.bundleSchedule.unreleased_enabled || false;
  els.bundlesUnreleased.checked = unreleasedEnabled;

  // Summary stats
  let totalEvents = 0, totalGacha = 0, totalLogin = 0, totalSideStories = 0, totalShops = 0;
  for (const b of bundles) {
    if (!activeBundles.has(b.month)) continue;
    totalEvents += b.event_count || 0;
    totalGacha += b.gacha_count || 0;
    totalLogin += b.login_count || 0;
    totalSideStories += b.side_story_count || 0;
    totalShops += b.shop_count || 0;
  }

  els.bundlesSummaryStats.innerHTML = `
    <div class="selection-stat"><span class="selection-stat-label">Selected</span><span class="selection-stat-value">${activeBundles.size}</span></div>
    <div class="selection-stat"><span class="selection-stat-label">Events</span><span class="selection-stat-value">${formatNumber(totalEvents)}</span></div>
    <div class="selection-stat"><span class="selection-stat-label">Gacha</span><span class="selection-stat-value">${formatNumber(totalGacha)}</span></div>
    <div class="selection-stat"><span class="selection-stat-label">Login</span><span class="selection-stat-value">${formatNumber(totalLogin)}</span></div>
    <div class="selection-stat"><span class="selection-stat-label">Shops</span><span class="selection-stat-value">${formatNumber(totalShops)}</span></div>
  `;

  els.bundlesStatus.textContent = `${bundles.length} monthly bundles available. Select months to enable their content in-game.`;

  // Group by year and render grid
  let lastYear = "";
  let html = "";
  for (const b of bundles) {
    const year = b.month.split("-")[0];
    if (year !== lastYear) {
      html += `<div class="bundle-year-label">${year}</div>`;
      lastYear = year;
    }
    const isSelected = activeBundles.has(b.month);
    const chips = [];
    if (b.event_count > 0) chips.push(`${b.event_count} event${b.event_count !== 1 ? "s" : ""}`);
    if (b.gacha_count > 0) chips.push(`${b.gacha_count} gacha`);
    if (b.login_count > 0) chips.push(`${b.login_count} login`);
    if (b.side_story_count > 0) chips.push(`${b.side_story_count} side`);
    if (b.shop_count > 0) chips.push(`${b.shop_count} shop${b.shop_count !== 1 ? "s" : ""}`);

    html += `
      <div class="bundle-card${isSelected ? " selected" : ""}" data-month="${b.month}">
        <div class="bundle-card-indicator"></div>
        <div class="bundle-card-month">${formatBundleMonth(b.month)}</div>
        <div class="bundle-card-stats">
          ${chips.map((c) => `<span class="bundle-stat-chip">${c}</span>`).join("")}
        </div>
      </div>
    `;
  }
  els.bundlesGrid.innerHTML = html;

  // Click handlers on cards
  els.bundlesGrid.querySelectorAll(".bundle-card").forEach((card) => {
    card.addEventListener("click", () => {
      const month = card.dataset.month;
      const active = state.bundleSchedule.active_bundles || [];
      const idx = active.indexOf(month);
      if (idx >= 0) {
        active.splice(idx, 1);
      } else {
        active.push(month);
        active.sort();
      }
      state.bundleSchedule.active_bundles = active;
      renderBundles();
    });
  });
}

async function loadBundles() {
  try {
    state.bundleCatalog = await fetchJSON("/api/bundles");
    state.bundleSchedule = await fetchJSON("/api/schedule");
  } catch {
    state.bundleCatalog = null;
    state.bundleSchedule = null;
  }
  renderBundles();
}

async function applyBundles() {
  if (!state.bundleSchedule) return;
  els.bundlesApply.disabled = true;
  els.bundlesApply.textContent = "Patching…";
  try {
    const result = await fetchJSON("/api/schedule", {
      method: "POST",
      body: JSON.stringify(state.bundleSchedule),
    });
    state.bundleSchedule = await fetchJSON("/api/schedule");
    renderBundles();
    const patchedCount = result?.results?.length || 0;
    window.alert(`Patch applied successfully. ${patchedCount} tables modified. The server will reload master data.`);
  } catch (error) {
    window.alert(`Patch failed: ${error.message}`);
  } finally {
    els.bundlesApply.disabled = false;
    els.bundlesApply.textContent = "Apply & Patch";
  }
}

// === Bundle event listeners ===

els.bundlesButton.addEventListener("click", () => {
  setView("bundles");
  if (!state.bundleCatalog) loadBundles();
});

els.bundlesApply.addEventListener("click", () => {
  applyBundles();
});

els.bundlesSelectAll.addEventListener("click", () => {
  if (!state.bundleCatalog || !state.bundleSchedule) return;
  state.bundleSchedule.active_bundles = (state.bundleCatalog.bundles || []).map((b) => b.month);
  renderBundles();
});

els.bundlesClearAll.addEventListener("click", () => {
  if (!state.bundleSchedule) return;
  state.bundleSchedule.active_bundles = [];
  renderBundles();
});

els.bundlesUnreleased.addEventListener("change", (event) => {
  if (!state.bundleSchedule) return;
  state.bundleSchedule.unreleased_enabled = event.target.checked;
  renderBundles();
});

els.bundlesRegenerate.addEventListener("click", async () => {
  els.bundlesRegenerate.disabled = true;
  els.bundlesRegenerate.textContent = "Regenerating…";
  try {
    await fetchJSON("/api/bundles/regenerate", { method: "POST" });
    await loadBundles();
    window.alert("Bundle index regenerated from master data.");
  } catch (error) {
    window.alert(`Regeneration failed: ${error.message}`);
  } finally {
    els.bundlesRegenerate.disabled = false;
    els.bundlesRegenerate.textContent = "Regenerate Index";
  }
});

// === End bundles ===

async function initialize() {
  state.overview = await fetchJSON("/api/overview");
  state.users = state.overview.users || [];
  state.gachaBannerCatalog = await fetchJSON("/api/master-data/gacha-banners");
  state.eventCatalog = await fetchJSON("/api/master-data/events");
  state.presetCatalog = await fetchJSON("/api/master-data/presets");
  renderOverview();
  renderPresetManager();
  renderGachaBannerManager();
  renderEventSelector();
  populateTableSelect();
  renderUsers();
  await loadHomepageUsers();

  // Load bundles in background
  loadBundles();

  setView("home");

  if (state.users.length > 0) {
    const firstUser = await fetchJSON(`/api/user/${state.users[0].userId}/summary`);
    renderUserSummary(firstUser);
    state.activeDeck = await fetchActiveDeckForUser(state.users[0].userId);
    state.selectedDeckKey = state.activeDeck?.selectedDeckKey || "";
    renderActiveDeck();
    state.selectedUserId = state.users[0].userId;
    if (state.selectedTable) {
      await loadTable(state.selectedTable);
    }
  } else if (state.selectedTable) {
    await loadTable(state.selectedTable);
  }
}

async function refreshOverviewAndSelection() {
  state.overview = await fetchJSON("/api/overview");
  state.users = state.overview.users || [];
  state.gachaBannerCatalog = await fetchJSON("/api/master-data/gacha-banners");
  state.eventCatalog = await fetchJSON("/api/master-data/events");
  state.presetCatalog = await fetchJSON("/api/master-data/presets");
  renderOverview();
  renderPresetManager();
  renderGachaBannerManager();
  renderEventSelector();
  populateTableSelect();
  renderUsers();
  await loadHomepageUsers();

  if (state.users.length === 0) {
    state.selectedUserId = null;
    els.userSummaryPanel.classList.add("hidden");
    els.tablePanel.classList.remove("hidden");
    els.heroTitle.textContent = "No users in this database";
    els.heroSubtitle.textContent = "Global and system tables are still available to inspect and edit.";
    setView("home");
    if (state.selectedTable) {
      await loadTable(state.selectedTable);
    }
    return;
  }

  const remaining = state.users.find((user) => String(user.userId) === String(state.selectedUserId));
  const fallback = remaining || state.users[0];
  if (state.currentView === "editor") {
    await selectUser(fallback.userId);
  } else {
    state.selectedUserId = fallback.userId;
    renderUsers();
  }
}

els.refreshUsers.addEventListener("click", async () => {
  await refreshOverviewAndSelection();
});

els.presetsClear.addEventListener("click", () => {
  state.selectedPresetIds = [];
  renderPresetManager();
});

els.presetsReload.addEventListener("click", async () => {
  state.presetCatalog = await fetchJSON("/api/master-data/presets");
  renderPresetManager();
});

els.presetsApplyAdd.addEventListener("click", () => {
  applySelectedPresets("add");
});

els.presetsApplyReplace.addEventListener("click", () => {
  applySelectedPresets("replace");
});

els.bannerSearch.addEventListener("input", (event) => {
  state.gachaBannerSearch = event.target.value;
  renderGachaBannerManager();
});

els.bannerUsableTab.addEventListener("click", () => {
  state.bannerSubtab = "usable";
  renderGachaBannerManager();
});

els.bannerUnusableTab.addEventListener("click", () => {
  state.bannerSubtab = "unusable";
  state.bannerShowSelectedOnly = false;
  renderGachaBannerManager();
});

els.bannerSelectedToggle.addEventListener("click", () => {
  if (state.bannerSubtab !== "usable") return;
  state.bannerShowSelectedOnly = !state.bannerShowSelectedOnly;
  renderGachaBannerManager();
});

els.bannerSelectAll.addEventListener("click", () => {
  if (state.bannerSubtab === "unusable") return;
  const records = recordsForCurrentBannerTab(state.gachaBannerCatalog?.records || []);
  state.gachaBannerCatalog.activeBannerIds = records.flatMap((record) => record.momBannerIds || []).sort((a, b) => a - b);
  renderGachaBannerManager();
});

els.bannerClearAll.addEventListener("click", () => {
  if (state.bannerSubtab === "unusable") return;
  state.gachaBannerCatalog.activeBannerIds = [];
  renderGachaBannerManager();
});

els.bannerReload.addEventListener("click", async () => {
  state.gachaBannerCatalog = await fetchJSON("/api/master-data/gacha-banners");
  state.presetCatalog = await fetchJSON("/api/master-data/presets");
  renderOverview();
  renderPresetManager();
  renderGachaBannerManager();
});

els.bannerSave.addEventListener("click", async () => {
  if (!state.gachaBannerCatalog?.enabled) return;
  const selected = [...bannerSelectionSet()].map((value) => Number(value)).sort((a, b) => a - b);
  const result = await fetchJSON("/api/master-data/gacha-banners", {
    method: "POST",
    body: JSON.stringify({ activeBannerIds: selected }),
  });
  state.gachaBannerCatalog = result;
  state.overview = await fetchJSON("/api/overview");
  state.presetCatalog = await fetchJSON("/api/master-data/presets");
  renderOverview();
  renderPresetManager();
  renderGachaBannerManager();
  const activeEntries = (state.gachaBannerCatalog.records || []).filter((record) => {
    const selectedIds = bannerSelectionSet();
    return isRecordFullySelected(record, selectedIds);
  }).length;
  window.alert(`Saved ${activeEntries} game-visible gacha entr${activeEntries === 1 ? "y" : "ies"} to the Mom banner table.`);
});

els.homeButton.addEventListener("click", () => {
  goHome();
});

els.bannerButton.addEventListener("click", () => {
  setView("banners");
});

els.eventsButton.addEventListener("click", () => {
  setView("events");
});

els.eventsSearch.addEventListener("input", (event) => {
  state.eventSearch = event.target.value;
  renderEventSelector();
});

els.eventsEventTab.addEventListener("click", () => {
  state.eventSubtab = "event_quests";
  state.eventCategoryFilter = "all";
  renderEventSelector();
});

els.eventsSideTab.addEventListener("click", () => {
  state.eventSubtab = "side_story_quests";
  state.eventCategoryFilter = "all";
  renderEventSelector();
});

els.eventsSelectedToggle.addEventListener("click", () => {
  state.eventShowSelectedOnly = !state.eventShowSelectedOnly;
  renderEventSelector();
});

els.eventsSelectAll.addEventListener("click", () => {
  const payload = currentEventGroupPayload();
  if (!payload) return;
  const next = eventSelectionSet();
  for (const record of currentFilteredEventRecords(payload, next)) {
    if (!isSelectableEventRecord(record)) continue;
    for (const value of record.ids || [record.id]) {
      next.add(String(value));
    }
  }
  payload.activeIds = [...next].map((value) => Number(value)).sort((a, b) => a - b);
  renderEventSelector();
});

els.eventsClearAll.addEventListener("click", () => {
  const payload = currentEventGroupPayload();
  if (!payload) return;
  const filteredIds = new Set();
  for (const record of currentFilteredEventRecords(payload, eventSelectionSet())) {
    if (!isSelectableEventRecord(record)) continue;
    for (const value of record.ids || [record.id]) {
      filteredIds.add(String(value));
    }
  }
  payload.activeIds = (payload.activeIds || []).filter((value) => !filteredIds.has(String(value)));
  renderEventSelector();
});

els.eventsReload.addEventListener("click", async () => {
  state.eventCatalog = await fetchJSON("/api/master-data/events");
  state.presetCatalog = await fetchJSON("/api/master-data/presets");
  renderPresetManager();
  renderEventSelector();
});

els.eventsSave.addEventListener("click", async () => {
  const payload = currentEventGroupPayload();
  if (!payload) return;
  const result = await fetchJSON("/api/master-data/events", {
    method: "POST",
    body: JSON.stringify({ group: state.eventSubtab, activeIds: payload.activeIds || [] }),
  });
  state.eventCatalog = result;
  state.presetCatalog = await fetchJSON("/api/master-data/presets");
  renderPresetManager();
  renderEventSelector();
  const count = (payload.activeIds || []).length;
  window.alert(`Saved ${count} ${payload.label.toLowerCase()}.`);
});

els.deckSelect.addEventListener("change", async (event) => {
  if (!state.selectedUserId) return;
  await loadActiveDeck(state.selectedUserId, event.target.value);
});

els.weaponTableSearch.addEventListener("input", (event) => {
  state.weaponInventorySearch = event.target.value;
  if (state.tableSchema && deckTableFilterConfig()) {
    renderTable();
  }
});

els.weaponEquippedOnly.addEventListener("change", (event) => {
  state.weaponEquippedOnly = event.target.checked;
  if (state.tableSchema && deckTableFilterConfig()) {
    renderTable();
  }
});

els.tableSearch.addEventListener("input", async (event) => {
  state.tableSearch = event.target.value;
  populateTableSelect();
  if (state.selectedTable) {
    await loadTable(state.selectedTable);
  } else {
    state.tableRows = [];
    state.tableAnnotations = [];
    state.tableSchema = null;
    els.editorTable.innerHTML = "";
    els.tableMeta.textContent = "No matching tables.";
    els.tableEmpty.classList.remove("hidden");
  }
});

// Custom table-select dropdown toggle
els.tableSelectTrigger.addEventListener("click", (event) => {
  event.stopPropagation();
  const isOpen = els.tableSelectTrigger.getAttribute("aria-expanded") === "true";
  if (isOpen) closeTableSelectPanel();
  else openTableSelectPanel();
});

// Close when clicking outside
document.addEventListener("click", (event) => {
  if (!els.tableSelectWrap?.contains(event.target)) {
    closeTableSelectPanel();
  }
});

// Keyboard: close on Escape
els.tableSelectPanel.addEventListener("keydown", (event) => {
  if (event.key === "Escape") closeTableSelectPanel();
});

els.addRow.addEventListener("click", () => {
  if (!state.tableSchema) return;
  markEditorInteraction(10000);
  state.tableRows = [buildBlankRow(), ...state.tableRows];
  state.tableAnnotations = [{}, ...state.tableAnnotations];
  renderTable();
  const draftRows = els.editorTable.querySelectorAll("tbody tr.draft-row");
  const newestDraft = draftRows[0];
  if (newestDraft) {
    newestDraft.scrollIntoView({ block: "nearest", behavior: "smooth" });
    const firstEditable = newestDraft.querySelector(
      'input:not([disabled]):not([type="hidden"]), select:not([disabled])',
    );
    firstEditable?.focus();
  }
});

els.editorTable.addEventListener("focusin", () => {
  markEditorInteraction(8000);
});

els.editorTable.addEventListener("input", () => {
  markEditorInteraction(8000);
});

els.editorTable.addEventListener("change", () => {
  markEditorInteraction(8000);
});

els.deleteUser.addEventListener("click", async () => {
  if (!state.selectedUserId) return;
  const confirmed = window.confirm(
    `Delete user ${state.selectedUserId} and all linked rows from the database? This cannot be undone.`
  );
  if (!confirmed) return;

  await fetchJSON(`/api/user/${state.selectedUserId}`, {
    method: "DELETE",
  });
  await refreshOverviewAndSelection();
});

els.invalidateSessions.addEventListener("click", async () => {
  if (!state.selectedUserId) return;
  const confirmed = window.confirm(
    `Invalidate active sessions for user ${state.selectedUserId}? The client should reconnect and reload fresh data from the database.`
  );
  if (!confirmed) return;

  const result = await fetchJSON(`/api/user/${state.selectedUserId}/sessions`, {
    method: "DELETE",
  });
  const deleted = Number(result.deletedSessions || 0);
  window.alert(
    deleted > 0
      ? `Cleared ${deleted} active session(s). Reopen or reconnect the client to refresh its data.`
      : "No active sessions were found for that user. If the client is open, reconnect it to fetch fresh data."
  );
});

initialize().catch((error) => {
  els.heroTitle.textContent = "Failed to load editor";
  els.heroSubtitle.textContent = error.message;
  console.error(error);
});

window.setInterval(() => {
  pollDeckViews();
}, 3000);
