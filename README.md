# Deathchase — Go Replication

A faithful Go replication of Deathchase (1983, Mervyn Estcourt) for the ZX Spectrum, built from the [Ritchie333/deathchase](https://github.com/Ritchie333/deathchase) Z80 SkoolKit disassembly.

## Status

**Pre-development.** Source analysis and implementation planning complete.

## Architecture

- **Headless engine** with Gym-like `Step(Action) → StepResult` API for AI training
- **Ebitengine** wrapper for human play
- **oto** direct audio for low-latency sound (~12ms)
- **Persistent settings** via JSON config

## Documentation

- `docs/deathchase-initial-analysis.md` — Z80 source analysis (pseudo-3D rendering, enemy AI, photon bolt, tree system)
- `docs/implementation-plan.md` — 8-phase Go implementation plan
- `docs/lessons-from-manic-miner.md` — Lessons from prior projects

## Source Reference

- `src/deathchase.skool` — Complete annotated Z80 disassembly (SkoolKit format, 3,188 lines)
- `src/deathchase_loader.skool` — BASIC loader
- `src/deathchase.ref` — SkoolKit reference with POKEs

## Credits

- **Original game:** Mervyn Estcourt (1983)
- **Disassembly:** Ritchie Swann (2018)
- **Go implementation:** Seamus Waldron with Claude AI
