# How ТЯГА's window behaves

The basics every change to the page is checked against. They come from Jakob Nielsen's ten
usability heuristics (1994), which most UI guidelines still build on, plus two rules of our
own. The visual rules (colours, motion, Material 3) are in `CLAUDE.md` under "Design rules";
this file is about behaviour.

## Ours first

**1. Nothing jumps.** The layout is stable and predictable. Things don't grow, shrink or move
by themselves while you look at them, and nothing moves when you click.
- A part that shows changing content has a fixed size: the step feed under a download is
  always four lines tall, the newest step on the bottom line, old ones leave over the top.
- Numbers that change (percent, track, speed) sit in fixed-width places.
- The scrollbar's place is always kept, so a long list doesn't push the page sideways.
- What appears because you asked for it (the fragment times, an error under the box) may
  push things down; what appears by itself must not.

**2. The window never freezes and nothing is lost.** Every click answers at once (at least a
visible change), the work goes on in the background, and closing the window or the program
doesn't throw away what was done: unfinished files resume, finished ones stay.

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
