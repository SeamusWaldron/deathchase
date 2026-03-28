# Deathchase — Initial Source Analysis

## Source Format

SkoolKit format (`.skool`), 3,188 lines. Disassembled by Ritchie Swann (2018). Relatively compact — the entire game fits in ~16KB ($3F48 bytes). Well annotated with comments on most routines.

**Source files:**
- `src/deathchase.skool` — Main game disassembly (3,188 lines)
- `src/deathchase_loader.skool` — BASIC loader (43 lines)
- `src/deathchase.ref` — SkoolKit reference with POKEs

## Game Overview

Deathchase (1983) by Mervyn Estcourt for the ZX Spectrum. Often cited as one of the best ZX Spectrum games ever made — remarkable for fitting a pseudo-3D chase game into 16KB.

**Gameplay:** You ride a motorcycle through a forest, chasing two enemy bikes. Shoot them with a photon bolt to advance to the next sector. Trees scroll towards you in pseudo-3D perspective. Avoid crashing into trees. Bonus enemies (planes/tanks) appear periodically. Day and night cycles alternate between sectors.

**Key mechanics:**
- Pseudo-3D perspective scrolling (trees approach from horizon)
- Two enemy bikes at varying distances (far → medium → near → in range)
- Photon bolt firing (single shot, travels ahead)
- Handlebar steering (left/centre/right)
- Speed control (fast/slow)
- 8 sectors per level
- Day/night patrol cycles
- Bonus enemies (planes, tanks)
- Collision detection with trees
- Score system (ASCII digits, 6 characters)

## Memory Layout ($5DC0-$5DEC: Game Variables)

### Enemy Bikes
| Address | Label | Description |
|---|---|---|
| $5DC0 | - | Enemy bike 1 direction (-1=left, 0=centre, 1=right) |
| $5DC1 | - | Enemy bike 2 direction |
| $5DC2 | - | Frames before bike 1 changes direction |
| $5DC3 | - | Frames before bike 2 changes direction |
| $5DC4 | - | Units to move bike 1 next frame (negative=left) |
| $5DC5 | - | Units to move bike 2 next frame |
| $5DC6 | - | Enemy bike 1 position |
| $5DC7 | - | Enemy bike 2 position (default $20) |
| $5DC8 | - | Active bikes (bit 0=bike 1, bit 1=bike 2) |
| $5DD2 | - | Number of enemy bikes on screen |
| $5DD5 | - | Distance to next enemy frame state (0-$1E) |
| $5DD6 | - | Enemy bike frame (0=far, 1=medium, 2=near, 3=in range) |

### Player/Controls
| Address | Label | Description |
|---|---|---|
| $5DC9 | - | Control method (0=keyboard, 1=kempston) |
| $5DDF | - | Handlebar position (-1=left, 0=centre, 1=right) |
| $5DE8 | - | Bike moving (0=no, 1=yes) |
| $5DEC | - | Current speed (0=fast, 2=slow) |

### Photon Bolt
| Address | Label | Description |
|---|---|---|
| $5DD7 | - | Firing (1=yes, 0=no) |
| $5DD8 | - | X position of bolt |
| $5DD9 | - | Frames of bolt left to draw |
| $5DDA | - | Pointer to next screen offset |
| $5DDC | - | Pointer to current bolt graphic |
| $5DDE | - | Bit 0: bolt hit an enemy |

### Bonus Enemy
| Address | Label | Description |
|---|---|---|
| $5DCA | - | X offset for bonus enemy |
| $5DCB | - | Status flags (bit 1=on screen, bit 5=should appear, bit 7=bolt hit) |
| $5DCC | - | Time before bonus enemy drawn |
| $5DCD | - | Address to invalidate/redraw bonus enemy |

### Game State
| Address | Label | Description |
|---|---|---|
| $5DCF | - | Random number counter |
| $5DD1 | - | Current sector (1-8) |
| $5DD3 | - | Day/night (bit 1: night patrol) |
| $5DE0 | - | Frames before shot bike explodes |
| $5DE1 | - | Position of exploding bike |
| $5DE6 | - | Size of playing area ($01A0 or $02E0) |

### Score
Stored at $6BCD as 6 ASCII digits. High score at $6BD7.

## Main Loop ($663D)

```
MainLoop:
  1. EI/DI (brief interrupt window)
  2. Check for input (keyboard/kempston)
  3. Check if photon bolt hit something
  4. Respond to bike movement
  5. Move the trees (scroll towards player)
  6. Adjust photon bolt position (if firing)
  7. Print objects on playing field
  8. Process hit bikes
  9. Toggle range indicator
  10. Recalculate enemy bike positions
  11. Check tree collision
  12. Move bikes nearer/further (based on speed)
  13. Move bonus enemy (plane/tank)
  14. Draw photon bolt trail + check hits
  15. Jump back to step 1
```

## Pseudo-3D Rendering System

This is the most unique aspect — the game creates a convincing 3D perspective using 2D techniques:

### Tree Scrolling ($5E00)
- Trees stored in a 256-byte buffer at $5B00
- Each row represents a "depth" line from horizon to foreground
- When the player moves, a random tree pattern is placed at the back row
- Trees shift forward each frame, creating the perspective scroll
- The playing area size ($5DE6) determines how far trees extend

### Enemy Bike Distance
Bikes progress through 4 frame states:
- 0 = Far (small sprite near horizon)
- 1 = Medium
- 2 = Near
- 3 = In range (can be shot)

Distance counter ($5DD5) ranges 0-$1E, gradually moving bikes closer.

### Photon Bolt Perspective
The bolt travels "into" the screen with decreasing screen offset and appropriate graphics for each distance level.

## Key Routines

| Address | Description |
|---|---|
| $5E00 | Move trees (scroll perspective) |
| $5EFE | Respond to bike movement direction |
| $5F89 | Print objects on playing field |
| $60C1 | Bike graphic drawing |
| $6113 | Alternative bike drawing |
| $61F3 | Recalculate enemy bike positions |
| $62CB | Switch to new sector |
| $633A | Move bonus enemy (plane/tank) |
| $6491 | Random number generator |
| $657F | Main entry point |
| $65B3 | Start new game |
| $663D | Main loop |
| $669D | Check input (keyboard/kempston) |
| $66FD | Check tree collision |
| $6711 | Handle bike death / level completion |
| $67C6 | Move bikes nearer/further |
| $6813 | Check if photon bolt hit something |
| $6859 | Draw photon bolt trail |
| $6918 | Display the player's bike |
| $6969 | Print score |
| $69E3 | Process hit bike |
| $6A28 | Adjust photon bolt position |
| $6AC2 | Handle player bike explosion |
| $6BDD | Update high score |

## POKEs (from deathchase.ref)

| POKE | Effect |
|---|---|
| 26463,0 | Infinite lives |
| 26379,62 | Invulnerability |
| 25792,201 | No trees |
| 25796,171 | Default difficulty |
| 25796,172 | Hurt Me Plenty |
| 25796,173 | Ultra Violence |
| 25796,174 | Nightmare! |

## Key Differences from Previous Games

| Aspect | Manic Miner | Jetpac | Atic Atac | Deathchase |
|---|---|---|---|---|
| View | Side-on | Side-on | Top-down | Pseudo-3D perspective |
| Movement | Platform | Flying | 8-dir walk | Forward + steer |
| Rendering | Tile-based | Sprite-based | Room-based | Perspective scroll |
| Size | ~12K lines | ~6K lines | ~14K lines | ~3.2K lines |
| Complexity | Medium | Low-Medium | High | Medium-Low |
| Unique challenge | Physics | Inertia | Room map | 3D perspective |

## Complexity Assessment

Deathchase is the **most compact** of the four games but has a unique technical challenge: the pseudo-3D perspective rendering. The game logic itself is simpler (no inventory, no rooms, no complex AI), but recreating the convincing 3D scrolling effect accurately will require careful analysis of the tree buffer and rendering routines.

Estimated implementation effort: 0.5-0.75x Manic Miner (less code, but the 3D perspective is tricky).
