/*!
 * Vendored from lucide-static v0.460.0 (ISC license, https://lucide.dev)
 * — only the icons used by the "Docker" screens (SVG icons instead of
 * emoji). Vendored locally (same convention as ../xterm/xterm.js) instead
 * of loaded from a CDN at runtime — a desktop app shouldn't depend on
 * network access just to draw its own UI.
 *
 * Each entry is just the original <svg>'s inner content (paths/shapes);
 * LUCIDE.svg() builds the full <svg> wrapper around it, with
 * stroke="currentColor" (follows the text/theme color) and a configurable
 * size.
 */
(function () {
  "use strict";

  var ICONS = {
    play: '<polygon points="6 3 20 12 6 21 6 3" />',
    square: '<rect width="18" height="18" x="3" y="3" rx="2" />',
    rotateCw:
      '<path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8" />' +
      '<path d="M21 3v5h-5" />',
    refreshCw:
      '<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" />' +
      '<path d="M21 3v5h-5" />' +
      '<path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" />' +
      '<path d="M8 16H3v5" />',
    scrollText:
      '<path d="M15 12h-5" />' +
      '<path d="M15 8h-5" />' +
      '<path d="M19 17V5a2 2 0 0 0-2-2H4" />' +
      '<path d="M8 21h12a2 2 0 0 0 2-2v-1a1 1 0 0 0-1-1H11a1 1 0 0 0-1 1v1a2 2 0 1 1-4 0V5a2 2 0 1 0-4 0v2a1 1 0 0 0 1 1h3" />',
    trash2:
      '<path d="M3 6h18" />' +
      '<path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />' +
      '<path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />' +
      '<line x1="10" x2="10" y1="11" y2="17" />' +
      '<line x1="14" x2="14" y1="11" y2="17" />',
    pencil:
      '<path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z" />' +
      '<path d="m15 5 4 4" />',
    download:
      '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />' +
      '<polyline points="7 10 12 15 17 10" />' +
      '<line x1="12" x2="12" y1="15" y2="3" />',
    upload:
      '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />' +
      '<polyline points="17 8 12 3 7 8" />' +
      '<line x1="12" x2="12" y1="3" y2="15" />',
    folderOpen:
      '<path d="m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2" />',
    triangleAlert:
      '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3" />' +
      '<path d="M12 9v4" />' +
      '<path d="M12 17h.01" />',
    x: '<path d="M18 6 6 18" /><path d="m6 6 12 12" />',
  };

  // LUCIDE.svg(name, opts) returns the full <svg>...</svg> markup string for
  // an icon. opts.size defaults to 16 (px, both width/height); opts.class
  // adds extra classes alongside the base "lucide-icon".
  function svg(name, opts) {
    opts = opts || {};
    var inner = ICONS[name];
    if (!inner) {
      return "";
    }
    var size = opts.size || 16;
    var cls = "lucide-icon" + (opts.class ? " " + opts.class : "");
    return (
      '<svg class="' + cls + '" width="' + size + '" height="' + size + '" ' +
      'viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" ' +
      'stroke-linecap="round" stroke-linejoin="round">' + inner + "</svg>"
    );
  }

  window.LUCIDE = { svg: svg };
})();
