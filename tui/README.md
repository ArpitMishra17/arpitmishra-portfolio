# tui — terminal portfolio

SSH-served terminal portfolio, in the style of `ssh superlogical.jobs`.
Go + [Wish](https://github.com/charmbracelet/wish) (SSH server) + [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Glamour](https://github.com/charmbracelet/glamour) (markdown).

## Run

```sh
go run .            # SSH server on :23234  →  ssh -p 23234 localhost
go run . -local     # render directly in your terminal, no SSH
go run . -render 120 40 Blog   # print one fixed-size frame (layout testing)
```

`PORT=... go run .` changes the listen port. The host key is auto-generated at `tui/.ssh/host_ed25519` (gitignored).

## Keys

- `←`/`→` or `tab`/`shift+tab` — switch section (also `1`–`6`)
- `↑`/`↓` or `k`/`j` — select item / scroll
- `g`/`G` — top/bottom (blog)
- `t` — theme picker (opencode-style dialog, type to filter)
- `?` — help dialog
- `z` — 🤫
- `q` — quit

## Layout notes

Omarchy-inspired: fixed left nav pane, hairline separator, generous padding,
monochrome with one soft accent. Content mirrors the website (see `data.go`);
blog posts live in `content/blog/` and are embedded into the binary — keep them
in sync with `../content/blog/`.

## Hosting (later)

Single static binary — deploy to Fly.io / a small VPS exposing TCP 22 (or any
port) and point `tui.arpitmishra.dev` at it. Vercel can't do raw TCP.
