# Decisions

A couple of lines per decision: what was chosen, why, what was rejected.
Updated at the end of every work session.

## 2026-11-18

- **Windows only.** ТЯГА is for the owner's own use, on Windows. Rejected: keeping macOS and
  Linux releases (nobody uses them, and a desktop window needs CGO and per-OS builds there).
  Linux stays only as browser mode for the test stand.
- **Wails v3 for the window.** Pure Go on Windows (no CGO), a tray icon out of the box, the
  same `index.html`; about +4 MB. Rejected: bare WebView2 (window, tray and menus by hand),
  Wails v2 (no tray), Electron (100+ MB), Tauri (Rust).
- **A frameless, transparent window: the cards are the window.** It looks like the app, not
  like a browser in a frame. The window height follows the content (no empty space, nothing
  cut off at 125 %).
- **× hides to the tray; Quit lives in the tray menu.** Downloads keep going with the window
  closed. The tray menu is ТЯГА's own little window in the app's style, not Windows' grey
  menu. Rejected: a Quit button on the page.
- **Portable: everything in a hidden `tyaga-data` next to the exe.** Download, run, delete
  the folder and nothing is left. WebView2's data goes there too, with caches trimmed.
  Rejected: AppData (it piled up 519 MB the owner didn't know about).
- **YouTube sign-in from Brave's cookies, read on every download.** Always fresh, nothing to
  copy by hand. Rejected: a one-off cookie export (goes stale), an in-app sign-in window as
  the main way (kept only for Chrome and Edge, whose cookies Windows locks).
- **Covers: the album the track is really from.** When YouTube Music gives the album, only
  that album's Apple Music cover; else its own thumbnail. A plain video gets the album where
  the song first came out (by the album's release date). Rejected: the first catalogue match
  (gave compilation covers), searching YouTube Music for every video (no public API, slow,
  can find a cover version).
- **Release CI builds only Windows x64 and ARM.**
- **Numbers say what they count** ("video 2 of 12", "40 % of all", "3 MB of 4 MB"). Rejected:
  bare "2 / 3" and "63 %".
- **A game (bricks and a ball) while downloading,** never interrupted by a finished
  download; the news shows above it. Rejected: confetti.
- **Every state change moves, briefly** (docs/ui-principles.md, rule 1a), checked frame by
  frame before every build.
- **One work session = one branch = one PR.** Fewer PRs, one place per session.
