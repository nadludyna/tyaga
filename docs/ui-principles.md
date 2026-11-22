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
