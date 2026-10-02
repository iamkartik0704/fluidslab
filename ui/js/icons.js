// DAMBREAK — inline SVG icons (Lucide-style: 24px viewBox, 2px stroke,
// round caps/joins). No icon font, no library, no remote assets.

const I = (inner) =>
  `<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${inner}</svg>`;

export const icons = {
  play:    I('<polygon points="6 4 20 12 6 20 6 4" fill="currentColor" stroke="none"/>'),
  pause:   I('<rect x="5" y="4" width="5" height="16" fill="currentColor" stroke="none"/><rect x="14" y="4" width="5" height="16" fill="currentColor" stroke="none"/>'),
  step:    I('<polygon points="5 4 15 12 5 20 5 4" fill="currentColor" stroke="none"/><line x1="19" y1="5" x2="19" y2="19"/>'),
  reset:   I('<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/>'),
  wifi:    I('<path d="M5 12.5a11 11 0 0 1 14 0"/><path d="M8.5 15.5a6.5 6.5 0 0 1 7 0"/><circle cx="12" cy="19" r="1" fill="currentColor"/>'),
  wifiOff: I('<path d="M5 12.5a11 11 0 0 1 14 0"/><path d="M8.5 15.5a6.5 6.5 0 0 1 7 0"/><line x1="3" y1="3" x2="21" y2="21"/>'),
  gauge:   I('<path d="M12 14 8 8"/><circle cx="12" cy="14" r="1" fill="currentColor"/><path d="M4 18a9 9 0 1 1 16 0"/>'),
  clock:   I('<circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 14"/>'),
  layers:  I('<polygon points="12 2 22 8.5 12 15 2 8.5 12 2"/><polyline points="2 15.5 12 22 22 15.5"/>'),
  activity:I('<polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/>'),
  flask:   I('<path d="M10 2v7L4.5 19a2 2 0 0 0 1.7 3h11.6a2 2 0 0 0 1.7-3L14 9V2"/><line x1="8.5" y1="2" x2="15.5" y2="2"/><line x1="7" y1="15" x2="17" y2="15"/>'),
  grid:    I('<rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/>'),
  wave:    I('<path d="M2 12c2-3 4-3 6 0s4 3 6 0 4-3 6 0"/><path d="M2 18c2-3 4-3 6 0s4 3 6 0 4-3 6 0"/>'),
  zap:     I('<polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" fill="currentColor" stroke="none"/>'),
  download:I('<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>'),
  settings:I('<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h.01a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h.01a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v.01a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>'),
  chart:   I('<line x1="4" y1="20" x2="20" y2="20"/><polyline points="4 16 9 10 13 13 20 5"/>'),
  info:    I('<circle cx="12" cy="12" r="9"/><line x1="12" y1="11" x2="12" y2="16"/><circle cx="12" cy="8" r="0.5" fill="currentColor"/>'),
  chevron: I('<polyline points="6 9 12 15 18 9"/>'),
  camera:  I('<path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/><circle cx="12" cy="13" r="4"/>'),
};

export function icon(name, cls) {
  const svg = icons[name] || icons.info;
  return cls ? svg.replace('class="icon"', `class="icon ${cls}"`) : svg;
}
