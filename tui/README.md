# tui — terminal portfolio

SSH-served terminal portfolio, in the style of `ssh superlogical.jobs`.
Go + [Wish](https://github.com/charmbracelet/wish) (SSH server) + [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Run

```sh
go run .            # SSH server on :23234  →  ssh -p 23234 localhost
go run . -local     # render directly in your terminal, no SSH
go run . -render 120 40 Contact   # print one fixed-size frame (layout testing)
```

`PORT=... go run .` changes the listen port. The host key is auto-generated at `tui/.ssh/host_ed25519` (gitignored).

## Keys

- `←`/`→` or `tab`/`shift+tab` — switch section (also `1`–`5`)
- `↑`/`↓` or `k`/`j` — select item
- `a`/`d` — walk the promenade (Overview); everyone connected is on it
- `n` — leave a note (Contact) — appended to `GUESTBOOK_PATH` (default `./guestbook.jsonl`), never shown to others
- `r` — toggle rain · `m` — pause all motion
- `t` — theme picker (opencode-style dialog, type to filter)
- `?` — help dialog
- `z` — 🤫 (press again to cycle; also starts after ~90s idle)
- `q` — quit

## Scenes

The animated bits are drawn into a small character framebuffer (`canvas.go`)
and rendered once per frame. Scenes are pure functions of time and position
(`scenes.go`, `fx.go`), coloured from the active theme, lit for the real hour
in Mumbai with the real moon phase. Shared state is just the visitor list
(`hub.go`). Preview knobs:

```sh
TUI_HOUR=18.5 go run . -local        # pin the time of day (dusk here)
TUI_RAIN=1 go run . -local           # force the monsoon
TUI_COLOR=1 TUI_VISITORS=3 TUI_T=7 go run . -render 120 36   # coloured still
TUI_SINCE=0.2 go run . -render 120 36 Experience   # freeze transitions 0.2s in
go run . -render 120 36 ss1          # a screensaver effect (ss0 is pipes, ss0–ss5), or `boot`, `note`
```

Pieces of [ascii.rest](https://ascii.rest) by [@bas3line](https://github.com/bas3line)
are adapted throughout, MIT licensed (see `LICENSE-ascii.rest`): the text effects
in `typefx.go` (title decode, typewriter tagline, split-flap headers, ticker,
wave), the Contact lighthouse in `scenes.go`, and the logos on Stack
(`logos.go`; the library's own in `logos_data.go`, the rest in `logos_traced.go`,
traced from devicon / Simple Icons artwork with ascii.rest's kit by
`tools/logos/gen.sh <ascii.rest checkout>` — add a line there to add a logo).
Logos are their owners' trademarks.

## Graphics

In terminals that speak the kitty graphics protocol with Unicode placeholders
(kitty, Ghostty, Rex, probably others), the TUI shows real company logos
(`assets/logos/<slug>.png`, with a monogram tile for a company without one) and
shaded 3D pipes in the screensaver. Detection is a probe sent when you connect,
so nothing needs setting over SSH. Locally, `TUI_KITTY=1` forces it on and
`TUI_KITTY=0` forces it off; without that, `-local` and `-render` read `TERM` and
`TERM_PROGRAM`. Everything else gets the text version. The images are sent once
per visit, about 64 KB. The sidebar's black hole is a pixel render (`bhpixels.go`,
16 transparent frames of 288x216 px, about 265 KB) sent a moment after the first
screen; the terminal's own background shows round it.

## Layout notes

Omarchy-inspired: fixed left nav pane, hairline separator, generous padding,
monochrome with one soft accent. Content mirrors the website (see `data.go`).

## Limits

The server is public, so `limits.go` and `model.go` cap what one visitor can use:

- terminal size is clamped to 600x200, well above a full-screen 4K terminal (bigger sizes cost memory on every frame)
- a session ends after 20 minutes with no key, and after 2 hours in total
- a connection is dropped if its client stops reading for a minute, or sends no input for 21 minutes
- 40 sessions at once, 4 per IP, and 10 new sessions a minute per IP
- guestbook file stops at 2 MB, 100 notes an hour overall, 3 per IP an hour

## Hosting

Runs on an Oracle Cloud Always Free VM (Ubuntu 24.04). The TUI owns port 22;
admin sshd lives on 2222. Vercel/most PaaS free tiers can't do raw TCP.

One-time server setup (after opening TCP 22 + 2222 in the VCN security list):

```sh
scp deploy/setup-server.sh deploy/tui.service ubuntu@<ip>:
ssh ubuntu@<ip> sudo bash setup-server.sh stage1
ssh -p 2222 ubuntu@<ip> true          # must succeed before stage2
ssh -p 2222 ubuntu@<ip> sudo bash setup-server.sh stage2
```

Every deploy after that:

```sh
TUI_HOST=ubuntu@<ip> ./deploy/deploy.sh   # GOARCH=amd64 for the E2.1.Micro shape
```

Logs: `ssh -p 2222 ubuntu@<ip> journalctl -u tui -f`. DNS: A record
`tui.arpitmishra.dev → <ip>`, DNS-only (no proxy).
