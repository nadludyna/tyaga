# How ТЯГА's window behaves

The basics every change to the page is checked against. They come from Jakob Nielsen's ten
usability heuristics (1994), which most UI guidelines still build on, plus rules of our
own. The visual rules (colours, motion, Material 3) are in the notes for AI agents (`CLAUDE.md`,
kept on the owner's computer, not in the repo) under "Design rules";
this file is about behaviour.

## How to check a change

Walk through it as the owner would, start to finish, not state by state: paste a link, sign in,
quit. Count what he has to do. Any step the program could have done itself, any moment where
he has to guess what's going on or wait without knowing why, is a bug to fix before showing.

### Nothing that worked may break

The owner's words, after fixing the same things twice: a build that breaks what worked costs
him more than one that adds nothing. So, before **every** build, however small the change:

- `tyagaCheck()`, `tyagaFlyCheck()` and `tyagaModeCheck()` all come back empty, in the empty
  window, with a link pasted, and with a list (queued, downloading, done); English and Russian,
  dark and light, 680 px and ~440 px. Run them once the theme has finished switching: its
  circle covers the page for a moment and the corner check then reports false alarms. In a
  hidden browser pane that circle never finishes: set the theme with
  `localStorage.setItem('tyaga-theme', 'light')` and reload instead of clicking.
- The flows in step 3 below, walked on the stand.
- Anything that fails is fixed before the build, not listed as "known" in the report.

Things the owner had to find himself, each now caught by a check (keep them in it):
a drop-down cut off under the list; the options row moving when Video/Audio switch; something
moving inside an opening drop-down; the link count off-centre in its badge; text cut at its
bottom edge; a button that doesn't fit its card in a narrow window.

### Before every build, measured, not by eye

The owner kept finding things the eye skips: a label 1–2 pixels above its icon, a button in
the wrong font, a window that blinked. So these are measured, every time, before a build:

1. **The layout check** (`stand/check.js`): load it in the stand's page and run
   `tyagaCheck()` in every state the change touches (empty, a link pasted and looked at, More
   open, the folder menu open, downloading, done, failed; English and Russian; dark and light;
   wide and ~440 px). It must return an empty list: labels in the middle of their icons and
   buttons by the letters themselves (±0.5 px), words in one row on one line (±1 px), Onest
   everywhere, nothing overlapping, nothing sticking out, nothing left in English in Russian.
2. **The real window** (`stand/run-window.sh`, `stand/record-window.ps1`): ТЯГА's own
   window over a bright magenta screen, a download started, the screen recorded at 20–30
   frames a second. Look at every frame of the list card for a blink (magenta showing
   through) and at the fish from start to the catch. The browser can't show this: it has no
   see-through window.
3. **Old behaviour still there**: walk the flows that worked before (paste several links,
   Retry, Undo, cancel, fold the list) — a change must not take anything away unasked.
4. **The flows, in the real window, start to finish, recorded**: paste a video link →
   preview → change the quality → fragment → Download → done → Show file → Copy; an audio
   link (album with chapters); a playlist link and a video inside a playlist; a failed one →
   Retry; cancel; clear finished → Undo; sign in → sign out. At every step, the window's
   height must match rule 1b, and nothing may vanish that is still needed.

Layout itself (grid, spacing, grouping, what goes in a flyout) follows the owner's `ui-layout`
skill (kept with his Claude settings, for all his projects): mockup first, then build, then
this check.

The rules behind the check are in "Ours first" below; a new kind of mistake found by eye gets
a new line in the check, so it's caught by itself next time.

## Ours first

**1. Nothing jumps.** The layout is stable and predictable. Things don't grow, shrink or move
by themselves while you look at them, and nothing moves when you click.
- A part that shows changing content has a fixed size: a download's row is two lines in
  every state, and what it's doing right now replaces the numbers in place.
- Numbers that change (percent, track, speed) sit in fixed-width places.
- The scrollbar's place is always kept, so a long list doesn't push the page sideways.
- What appears because you asked for it (the fragment times, an error under the box) may
  push things down; what appears by itself must not.

**1a. Every change of state moves, briefly.** Nothing pops in or vanishes: what appears fades
or slides in, what goes away folds up or fades out, a switch between two views cross-fades,
and the neighbours glide to their new place. Short (0.15–0.35 s), the same easing everywhere,
never a bounce or a show. A missing or abrupt transition is a bug, the same as a wrong number.
Before a build, list every state change the work touches (appear, disappear, expand,
collapse, switch views, clear, hover, press, the mouse leaving and coming back) and watch each
one frame by frame on the stand (screenshots every ~100 ms). The owner should never have to
point one out.

**1b. The window changes its height only on these events, and on nothing else** (the owner's
call: compact, but predictable):
1. Nothing pasted, no downloads: header and the links box only, the smallest window.
2. A link is pasted and looked at: the window grows once by the preview card. The link is
   removed: it shrinks back.
3. A download is added: the preview turns into a row of the list; the list grows by one row
   per download, up to about five rows, then scrolls inside.
4. Finished downloads are cleared, or the list is folded or unfolded: it shrinks or grows.

Opening the quality or fragment picker, More, the folder menu, a tooltip, hovering or
switching an option never changes the window's size. The pickers are flyouts over the preview
card (they only exist while there is one, and it is taller than them). Every grow is the exact
reverse of its shrink: same duration, same easing. The see-through part of the window never
catches clicks.

**2. The window never freezes and nothing is lost.** Every click answers at once (at least a
visible change), the work goes on in the background, and closing the window or the program
doesn't throw away what was done: unfinished files resume, finished ones stay.

**3. Don't make the person do what the program can do.** If ТЯГА can tell that a step is
done, it takes the next one itself. The sign-in window closes by itself once you're signed in;
"close this window when you're done" is a bug. Every step left to the person needs a reason.

**4. Good defaults, asked once.** The settings most people want are already on; whatever you
change is remembered. Nothing asks the same question twice.

**5. Answer at once, say how long.** A click shows something within 0.1 s; anything over a
second shows that it's working; anything over ten says how far along it is (Nielsen's
response-time limits).

**6. Windows habits.** Enter does the main thing, Esc closes or cancels, the cursor is already
where you type, a window opens in the middle of the screen. Nothing disappears silently: when
× hides ТЯГА to the tray, a notification says it's there and the downloads go on.

## Nielsen's ten, the way they apply here

1. **Show what's happening.** Queued, waiting (YouTube's pause), downloading, converting,
   done, failed: each looks different and says so in words. If ТЯГА stops answering, the
   page says that too, instead of doing nothing.
2. **Speak the person's language.** "Open folder", not "output dir". No paths, ids or
   `[youtube]` tags.
3. **A way out.** Cancel stops a download; Retry undoes a failure or a cancel; nothing that
   can't be undone happens without asking.
4. **Consistent.** One kind of button looks and works the same everywhere; Windows habits
   hold (Enter downloads, Ctrl+A selects the links, the title bar buttons work like Windows').
5. **Prevent mistakes.** Options that make no sense together are hidden or greyed out
   ("as one file" only for a playlist). A bad link is caught when it's added, not after.
6. **Seen, not remembered.** All options are on screen; the chosen ones are remembered.
7. **Shortcuts for the regulars.** Enter, drag a link anywhere onto the window, Ctrl+A.
8. **Only what's needed.** Every extra thing on screen takes attention from the important one.
9. **Errors in plain words.** What happened and what to do; the technical text only behind
   "Copy error".
10. **Help where it's needed.** Hints on hover, next to the thing they explain.
