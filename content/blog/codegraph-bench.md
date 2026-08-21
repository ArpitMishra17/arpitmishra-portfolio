---
title: "I built a benchmark to find out which code graph actually understands your codebase"
description: "While working on the code graph at Vyazen, I built CodeGraph-Bench on the side as an open source way to score code indexing tools against the TypeScript compiler. Here is what I found on BabylonJS and VS Code."
publishedAt: "2026-08-21"
image: "/images/blog/codegraph-bench.webp"
author: "Arpit Mishra"
---

If you have watched a coding agent confidently point at the wrong function or burn thirty tool calls grepping before landing on the wrong one, you have felt the cost of how most agents navigate code.

Most agents search code. They do not understand it. Claude Code reads files and runs grep. Copilot works from open tabs. Cursor chunks code with tree-sitter and searches by embedding similarity. None of them build an explicit, queryable map of how code connects.

While I was working on the code graph at Vyazen, which is core to what we are building, I kept wondering whether our approach was actually better. I needed a neutral way to check. So I built CodeGraph-Bench on the side as an open source framework that scores code indexing tools against ground truth from the compiler. Here is what I found.

---

## Why a graph, and why size changes everything

A **code knowledge graph** is a map of a codebase: **nodes** are symbols like classes, functions, properties and **edges** are relationships between them like calls, imports, inheritance, implementations.

On a twenty file service, none of this matters. Grep is fine. The agent reads the whole thing. The problem starts where enterprise code actually lives: eight thousand files, eighty thousand symbols, a dozen modules that each define their own `initialize` and `update` and `process`.

At that size, text search returns forty matches for one name and no way to rank them. Embedding search returns the code that looks most similar to the query, which is not the same as the code that is actually wired to it. Both degrade in the same direction: more code means more near duplicates, and neither method knows which one is connected to the thing you asked about.

A graph does not degrade that way, because the connection is resolved once at index time and stored. "Who calls `processBatch`" stops being a search over the repo and becomes a lookup over edges, one hop, bounded cost, same answer every session. The agent stops rebuilding a mental model from scratch and starts querying one.

That is the promise, anyway. Whether a given graph delivers it depends entirely on whether its edges are correct, and until now nothing measured that.

## The tools I tested

I took four indexers that build graphs from TypeScript, spanning very different architectures: one with full type resolution, one heuristic plus clustering, and two deterministic tree sitter taggers.

| Tool | Approach |
| --- | --- |
| Vyazen | Vyazen's internal code graph |
| GitNexus | Heuristic extraction plus Leiden clustering |
| Graphify | Deterministic tree sitter. No LLM, no type resolution |
| Potpie | Rust and rayon tree sitter tagging. No scope or type resolution |

## What I measured

Four things, in three families. The split that matters most is *coverage* versus *trustworthiness*, how much of the real codebase a tool finds versus how much of what it claims is actually real. They are independent. A tool can be excellent at one and poor at the other, and each failure hurts an agent differently.

| Metric | What it asks |
| --- | --- |
| Symbols found, Symbols correct | Of the real symbols, how many did the tool find? Of the symbols it claims, how many are real? |
| Connections found, Connections correct | The same split, applied to relationships between symbols. |
| Call precision, Call recall | Of the `A calls B` claims, how many are real? Of the real calls, how many were found? |
| Target accuracy | Of the correct call edges, how many point to the exact file and line the compiler resolves to? |

Target accuracy is the one people skip, and it is the one that decides whether a graph is usable. Every score below runs 0 to 100.

## How I scored it

I did not want any tool's output to be the answer key. The oracle is the **TypeScript compiler with full type checking**. It knows which specific function a call resolves to, through overloading, inheritance and module resolution.

Every tool ran against the exact same commit of each repo. Each output was normalized to a common schema and scored against the same oracle. Where the compiler itself cannot resolve a target, like external packages or dynamic dispatch, those edges were excluded from scoring, so no tool is penalized for what is unknowable at compile time. And a heuristic tool that lands on the same target the compiler resolves to gets full credit, however it got there.

---

## The benchmark

### The repos

Two real codebases, deliberately unalike:

* **BabylonJS**, a game engine. Roughly 8,400 files and 80,000 symbols in a single monorepo. The compiler saw **138,575 call sites**, resolving 83.6% directly and excluding 15.8%.
* **VS Code**, an editor. 5,108 TS/JS files spread across 84 projects. **393,675 call sites**, 87.0% directly resolved, 11.9% excluded.

One is deep and monolithic, the other wide and heavily cross project. Every chart below is toggleable between the two.

### Symbols: does the tool know what is in there?

Symbols are the vocabulary. Miss a class and every question about it comes back empty. Invent one and the agent goes looking for code that does not exist. Nothing downstream can be right if this layer is not correct. An edge to a symbol you never found is an edge you cannot emit.

:::chart-symbols

Vyazen finds essentially the whole codebase on both repos, 99.4% and 99.2%, while the tree sitter taggers see roughly half. Note Potpie's shape on VS Code: 90.9% correct on only 53.7% found. What it reports is largely real, it just is not reporting most of the codebase. That is a coverage failure wearing a good precision number, and it is why these two columns have to be read together.

### Connections: does it know how the pieces relate?

Connections are where a graph earns its name. This is the layer that answers change impact questions, what breaks if I touch this, what inherits from that, what pulls this module in. A missing edge makes a caller invisible during a refactor. A phantom edge sends the agent to code that has nothing to do with the change.

:::chart-connections

GitNexus emits the most relationships of any tool, 259,345 on BabylonJS and 522,198 on VS Code, and still finds the fewest real ones. Roughly half of what it claims does not hold up. Volume is not coverage. Potpie's zeros are not a low score but an absence: it emits no `CALLS`, `IMPORTS`, `EXTENDS` or `IMPLEMENTS` edges for TypeScript, by design.

### Calls: does the edge point at the right code?

Calls are the edges agents actually traverse, and they are where name matching quietly falls apart. In a large repo, detecting that "A calls something named `update`" gets existence right and the target wrong. There are dozens of `update`s. Target accuracy asks the harder question: of the edges that are correct, how many land on the exact file and line the compiler resolves to?

:::chart-calls

> Put it in an agent's hands. You ask: *who calls `processBatch`?* in an 80,000 symbol codebase. GitNexus, at 48% call precision, hands back a list where about half are phantom `processBatch` functions elsewhere. Graphify, at 33.7% recall, misses two thirds of the real callers. Vyazen returns a short list where nearly every entry is a real caller, at the exact file and line the compiler resolves to. That is the difference between opening the right file first try and chasing references that were never there.

Cost is comparable, and in my run it was lower. A cold build of VS Code took Vyazen 69.9s and 2.37 GiB, against GitNexus at 146.3s and 5.57 GiB and Graphify at 140.3s and 3.93 GiB. The BabylonJS figures were a live read out rather than a cold build.

### Full scorecards

#### BabylonJS

| Tool | Symbols | Rel. | Sym found | Sym correct | Conn found | Conn correct | Call prec | Call rec | Target |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Vyazen | 79,324 | 152,639 | 99.4 | 98.2 | 79.7 | 90.4 | 85.1 | 74.5 | 99.9 |
| GitNexus | 80,773 | 259,345 | 83.5 | 77.8 | 39.9 | 49.3 | 48.2 | 62.2 | 97.2 |
| Graphify | 56,412 | 133,845 | 52.7 | 69.4 | 58.5 | 78.8 | 67.8 | 33.7 | 98.4 |
| Potpie | 54,034 | 134,740 | 48.1 | 70.2 | 0.0 | 0.0 | n/a | 0.0 | n/a |

#### VS Code

| Tool | Symbols | Rel. | Sym found | Sym correct | Conn found | Conn correct | Call prec | Call rec | Target |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Vyazen | 155,531 | 400,063 | 99.2 | 90.0 | 84.3 | 89.1 | 92.2 | 79.1 | 99.9 |
| GitNexus | 132,925 | 522,198 | 69.4 | 77.1 | 31.4 | 47.5 | 45.7 | 46.4 | 94.8 |
| Graphify | 94,260 | 338,945 | 57.5 | 85.1 | 57.6 | 79.9 | 84.7 | 36.4 | 97.2 |
| Potpie | 85,658 | 320,001 | 53.7 | 90.9 | 0.0 | 0.0 | n/a | 0.0 | n/a |

---

## What the scorecards add up to

Read across all three layers and the tools separate cleanly. GitNexus is crowded but noisy, most edges, fewest real ones. Graphify is trustworthy where it speaks and silent most of the time. It is dependable on inheritance and wins outright on import recall, 97.6% on BabylonJS and 93.4% on VS Code against Vyazen's 88.2% and 94.5% at module level. Potpie builds a symbol index, not a graph.

Vyazen is the only tool that holds breadth and trust together at the same time: 99% plus of symbols found, 80 to 84% of connections found, 89 to 90% of claimed connections real, and target accuracy of **99.9% on both repos**. When it says `Scene.ts:247` calls `processBatch`, it means that function, not a name that matches.

### What to keep in mind

* **Graphs are still emerging.** Most agents today run on grep and file reads, not graphs. This benchmarks a fast growing category, not the default.
* **TypeScript only, for now.** I am extending the oracle to other languages.
* **Graphify and Potpie ran LLM free.** Deterministic tree sitter only, not their optional LLM modes. The comparison is about architecture.
* **Open and reproducible.** Every scorecard, methodology note and adapter contract is public.

## Why I built it

When I started working on Vyazen's code graph and saw what correctly resolved edges make possible, I noticed that every conversation about indexing was qualitative. Faster. Finds more. Uses a graph. There was no shared way to say *more accurate, on real code, against neutral ground truth*.

Code graphs are moving from curiosity to practical tooling. Without a standard evaluation, an accurate graph and an impressive looking but imprecise one are indistinguishable, right up until an agent acts on the wrong edge in production.

CodeGraph-Bench is my attempt to fix that as a side project while working on the graph at Vyazen: a fair, transparent, reproducible way to ask **does your code graph understand your code?** It is open source. Bring your own tool, write an adapter, and get a scorecard against the same oracle and the same repos. The field is better off when tools compete on measured quality instead of marketing.

---

**Glossary.** *Code knowledge graph*, a map of a codebase where symbols are nodes and relationships are edges. *Oracle*, the ground truth, here the TypeScript compiler with full type checking. *Target accuracy*, the share of correct edges pointing to the exact file and line the compiler resolves to. *Call precision and recall*, of the claimed calls, how many are real, of the real calls, how many were found.

Full methodology and the bring your own tool guide: [github.com/Vyazen/CodeGraph-Bench](https://github.com/Vyazen/CodeGraph-Bench)
