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
