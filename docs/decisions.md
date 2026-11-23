# Decisions

A couple of lines per decision: what was chosen, why, what was rejected.
Updated at the end of every work session.

## 2026-11-18

- **Windows only.** ТЯГА is for the owner's own use, on Windows. Rejected: keeping macOS and
  Linux releases (nobody uses them, and a desktop window needs CGO and per-OS builds there).
  Linux stays only as browser mode for the test stand.
- **Wails v3 for the window.** Pure Go on Windows (no CGO), a tray icon out of the box, the
  same `index.html`; the exe grew from 7.5 to about 13 MB. Rejected: bare WebView2 (window, tray and menus by hand),
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

## 2026-11-19

- **A downloader, not a show.** The window answers four things: where to paste, what I'll
  get, is it coming and how long, where is it. One compact two-line row per download, the way
  Seal and 4K Video Downloader do it. Rejected after trying them this session: falling notes
  (everywhere), the link flying into its row (with notes and a ripple), the game, the step
  log. Each was fine alone; together they were noise and the window got as tall as the screen
  (feature creep). The owner's verdict, and he was right.
- **Nothing moves while nothing happens.** The window is transparent, so every moving pixel
  makes Windows redraw under it (DWM): 5–17 % CPU idle on the owner's PC. Motion only while
  downloading and in answer to a click.
- **Progress on the taskbar button** (ITaskbarList3) and in the tray tooltip: watch it with
  the window out of the way. Rejected: an always-on-top mini window (it covers the view).
- **Copy the file to the clipboard** (CF_HDROP) next to Show file: the owner sends most files
  to Telegram, Ctrl+V attaches it. Not a "Send to Telegram" button: he doesn't always.
- **Audio first**, M4A/MP3 next to it: he downloads music more often.
- **Fragment and "for editing" stay**, shown only where they apply. Options stay visible, not
  under a gear or in a dropdown: seen, not remembered.
- **"Whole playlist" greyed out** unless the link is a video inside a playlist, in its place,
  so nothing moves when you paste.
- **A playlist with skipped videos is done**, with a note and Retry for what's missing, and is
  named after the playlist. Rejected: failing the whole thing over one private video.
- **Hover, press and keyboard focus are clearly visible** on every control (a tint, a ripple
  from the click, an orange ring).
- **Hints only where the label doesn't say it all**, as one solid bubble placed to fit the
  window. Rejected: a hint on everything (they repeated the labels and were cut off).
- **A sun/moon theme button**, Windows' theme until clicked. One snapshot and a growing
  circle (View Transitions), cheap.
- **Selected switches are tonal**, solid orange only for Download and the bar.
- **Show file opens Explorer without a fake Alt key** (it broke Alt+Tab; confirmed fixed).
- **The window grows ahead** when a link is in the box, and never resizes during an animation:
  resizing a transparent window on Windows is slow and was the stutter the stand didn't show.
- **Test builds by one PowerShell line** that clones the branch fresh and builds into the
  owner's builds folder: building from his own clone picked up `main` by mistake.

## 2026-11-22/23

- **The options row is icons of one size.** Every button 32 px tall, icon buttons 56 px wide,
  8 px between all of them, and the window (653) as wide as the row: no hole anywhere.
  Quality (HD in a frame), format, fragment, "for editing" and the folder are icons with a hint.
- **A word holds its place in both languages.** A button carries its label in both languages,
  the other one invisible: switching the language moves and resizes nothing.
- **A drop-down grows out of its button as one shape**, a neutral outline, unrolling in 0.12 s,
  opening upwards when the list is folded. Rejected: an orange outline (too loud), a floating
  menu (looked unattached). Hover and press are about twice as fast as before ("snappy").
- **Fragment is a slider and the chapters, not typed times.** Knowing timestamps was the chore.
  A link's own `?t=` is the start, as a chip that switches it off. Greyed out until there is
  exactly one video link. Rejected after trying: frame previews (storyboards), heavy and slow.
- **Quality lists only the heights the video has**; with no link or several links, all of them.
- **Downloads go to `Downloads\ТЯГА\Видео|Аудио`** (folder names in the page's language; an
  existing one wins). The old default `Downloads\YouTube` moves over; its files stay.
- **A playlist's line is the current track**, 0 → 100 % each: at each new track the fish swims
  on, out over the right edge, and comes back in from the left (seamless); how far the whole
  list is stays in words. A single download: the fish swims out and that's it. Rejected: one
  line for the whole list (the fish crept a pixel per track), a leap at each track (it went
  over the row above and vanished before the line).
- **CI checks the page too**: its script compiles (`stand/syntax.js`), and the stand's layout
  checks run in a browser without a window in every state, both themes, both languages, both
  widths (`stand/ci.js`). Before, a typo that killed the page passed CI green.
- **The line and the fish go away as one**: the picture fades, then its place folds; nothing
  over the fish (a leap, "?", splashes) reaches the row above, and the water scene under the
  list ends where the fish ends. The sky over the header got clouds.
- **The tray icon's place is left to Windows.** Windows keys "show on the taskbar" by the
  exe's path; forcing it means the registry, which ТЯГА doesn't touch (portable).
- **Pin on top is "always on top", not "survives Win+D".** Show desktop hides every window,
  pinned ones too, as in Windows itself; keeping one over it needs hacks.
