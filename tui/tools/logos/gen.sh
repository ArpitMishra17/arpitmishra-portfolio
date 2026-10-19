#!/usr/bin/env bash
# Regenerates ../../logos_traced.go: downloads each technology's icon,
# renders it with transparency, and traces it with ascii.rest's kit at the
# size the Stack page shows logos.
#
#   ./gen.sh /path/to/ascii.rest/checkout
#
# Needs macOS (AppKit renders the SVGs), swiftc and Node 23.6+ (runs the
# kit's TypeScript directly).
set -euo pipefail
kit="${1:?path to an ascii.rest checkout}/src/kit/image.ts"
here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# Name in the stack list = devicon icon, or si:<slug>:<colour> for Simple Icons
logos=(
	"Solidity=solidity-original" "React=react-original" "Node.js=nodejs-original"
	"Express=express-original" "Next.js=nextjs-original" "FastAPI=fastapi-original"
	"Django=django-plain" "Tailwind CSS=tailwindcss-original" "Git=git-original"
	"GitHub=github-original" "VSCode=vscode-original" "Jupyter=jupyter-original"
	"Docker=docker-original" "Hardhat=hardhat-original" "LangChain=si:langchain:#1C3C3C"
	"PostgreSQL=postgresql-original" "Redis=redis-original" "MongoDB=mongodb-original"
)

swiftc -O "$here/svg2rgba.swift" -o "$work/svg2rgba"
args=()
for entry in "${logos[@]}"; do
	name="${entry%%=*}" src="${entry#*=}" svg="$work/${#args[@]}.svg"
	if [[ $src == si:* ]]; then
		IFS=: read -r _ slug colour <<<"$src"
		curl -fsS -m 20 "https://cdn.jsdelivr.net/npm/simple-icons@latest/icons/$slug.svg" | sed "s/<svg /<svg fill=\"$colour\" /" >"$svg"
	else
		curl -fsS -m 20 -o "$svg" "https://cdn.jsdelivr.net/gh/devicons/devicon/icons/${src%-*}/$src.svg"
	fi
	read -r w h < <("$work/svg2rgba" "$svg" "$svg.rgba" 512)
	args+=("$name=$svg.rgba:${w}x$h")
done
node "$here/trace.mjs" "$kit" 30 16 "${args[@]}" >"$here/../../logos_traced.go"
gofmt -w "$here/../../logos_traced.go"
echo "wrote logos_traced.go (${#args[@]} logos)"
