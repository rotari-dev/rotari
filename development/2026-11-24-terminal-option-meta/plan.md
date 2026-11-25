# Plan: Option as Meta in the Web terminal

Created: 2026-11-24
Status: Implemented on 2026-11-25; see [work-log.md](work-log.md). Not done: the checks by hand, and the Keys item.
Related: [Web terminal](../2026-11-05-web-terminal/plan.md), and the two plans proposed with this one:
[font](../2026-11-24-terminal-font/plan.md) and [OSC 52 copy](../2026-11-24-terminal-osc52-copy/plan.md)

## Purpose

On a Mac, Option types characters such as `å`, so the `Alt` and `Meta` keys of `tmux`, Neovim, bash, and zsh (`M-b`,
`M-f`, `M-.`) cannot be used. Terminal applications have a setting for this; the panel has none.

## Today

Panes are made without `macOptionIsMeta`, which is off by default in xterm.js 5.5.0. Other systems are not affected.
The panel has no keyboard shortcuts of its own for the terminal, so nothing of rotari's clashes with `tmux`.

## Scope

A per-browser setting that sends Option as Meta on a Mac, and the documentation of which keys a browser keeps.

## Non-goals

- Left and right Option apart. xterm.js has one setting for both.
- Other key mappings, or a file of them.
- Taking Ctrl-W, Ctrl-T, and Ctrl-N from the browser. A page cannot, except with Chrome's Keyboard Lock in full screen,
  which would be a plan of its own.

## Decisions

1. **Off by default.** With it on, Option no longer types characters, and some layouts need that: on a Japanese Mac
   keyboard Option-¥ types a backslash, and on many European ones `@`, `[`, `{`, or `|` need Option.
2. **Where:** **Use Option as Meta** in the bar's Options menu of the [font plan](../2026-11-24-terminal-font/plan.md),
   marked when on, only on a Mac, as xterm.js decides it (the browser's platform). The pane's right-click menu does not
   open while a program takes the mouse.
3. **Kept by the browser**, under `rotari-terminal-option-meta`. `"1"` is on and anything else is off. When storage is
   refused, it lasts for the page, and a note says so.
4. **It applies at once** to every pane, and to panes opened later.
5. **Option with the mouse does not change.** Alt+click keeps xterm.js's own cursor move.

## Design

The option is set on each `pane.term.options`, which xterm.js reads at each key, so no pane is made again. One function
reads the setting and is used both where a pane opens and where it changes.

## Documentation

- [docs/WEB.md](../../docs/WEB.md): the setting, and the **Keys** item, which names only Ctrl-W today. Chrome on Windows
  and Linux also keeps Ctrl-T and Ctrl-N, which Neovim uses; a Mac keeps the Cmd ones, so Ctrl-W reaches the shell
  there. What each browser keeps is checked by hand before it is written, which has not been done, so the item is as it
  was.
- WEB-33's note, and no ID of its own, as for the [font](../2026-11-24-terminal-font/plan.md).

## Tests and validation

- `TestTerminalPanel` (JSDOM): off by default, on from storage, a bad stored value, every pane and later ones, the menu
  item only on a Mac and while a program takes the mouse, storage refused.
- By hand on a Mac: `M-b` and `M-f` in bash and zsh, a `tmux` `M-` binding, Neovim `<A-j>`, Japanese input, and
  Option-¥ with the setting off.
