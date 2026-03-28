# Deathchase Go Implementation Plan

## Technology Stack
Same as previous projects:
- **Language:** Go
- **Graphics:** Ebitengine (ebiten/v2)
- **Audio:** oto/v3 directly
- **Persistence:** JSON config (~/.deathchase/config.json)

## Package Structure

```
deathchase/
├── main.go
├── Makefile
├── action/
│   └── action.go            # Action{Left, Right, Accelerate, Brake, Fire, Enter, Escape}
├── engine/
│   ├── engine.go            # Headless GameEnv
│   ├── observation.go
│   ├── constants.go
│   └── engine_test.go
├── entity/
│   ├── player.go            # Player bike: steering, speed, collision
│   ├── enemy.go             # Enemy bikes: distance, position, AI
│   ├── photon.go            # Photon bolt: firing, travel, hit detection
│   ├── bonus.go             # Bonus enemies (planes, tanks)
│   └── explosion.go         # Explosion animation
├── world/
│   ├── trees.go             # Tree buffer + perspective scrolling
│   ├── perspective.go       # 3D perspective calculations
│   └── sector.go            # Sector management, day/night
├── screen/
│   ├── buffer.go            # ZX Spectrum buffer system
│   ├── renderer.go          # Buffer → Ebitengine image
│   ├── sprites.go           # Sprite drawing
│   └── text.go              # ZX Spectrum ROM font
├── audio/
│   └── audio.go             # oto direct
├── data/
│   ├── sprites.go           # Bike, tree, bolt, explosion graphics
│   └── sfx.go               # Sound parameters
├── config/
│   └── config.go
├── input/
│   └── input.go
├── game/
│   ├── game.go              # Ebitengine wrapper
│   ├── hud.go               # Score, lives, sector, range indicator
│   ├── settings.go
│   └── help.go
└── docs/
```

## Implementation Phases

### Phase 1: Core Rendering + Trees
- ZX Spectrum buffer system (reuse)
- Tree buffer ($5B00, 256 bytes)
- Tree scrolling (perspective shift per frame)
- Playing area rendering
- Day/night colour switching

### Phase 2: Player Bike
- Handlebar steering (left/centre/right)
- Speed control (fast/slow/stopped)
- Bike display sprite
- Tree collision detection

### Phase 3: Enemy Bikes
- Two enemy bike slots
- Distance system (4 states: far→medium→near→in range)
- Enemy AI (direction changes, position tracking)
- Enemy bike sprites at each distance
- Sector completion (both bikes destroyed → next sector)

### Phase 4: Photon Bolt
- Firing mechanic (single bolt)
- Bolt perspective travel (into the screen)
- Bolt trail rendering
- Hit detection (enemy bikes, bonus enemies)

### Phase 5: Bonus Enemies
- Planes and tanks
- Appearance timing
- Movement patterns
- Scoring

### Phase 6: Game Progression
- 8 sectors per level
- Day/night patrol alternation
- Lives system
- Score and high score (ASCII digits)
- Difficulty progression
- Explosion animations

### Phase 7: Audio
- Engine/movement sound
- Photon bolt sound
- Explosion sounds
- Level complete sounds

### Phase 8: Menu & Polish
- Title screen
- Keyboard/Joystick selection
- Settings, high scores, help

## Key Technical Challenge: Pseudo-3D Perspective

The tree scrolling creates a convincing 3D effect. Understanding this is critical:

1. **Tree buffer** ($5B00, 256 bytes): Represents tree positions at different depths
2. **Scrolling**: Each frame, rows shift forward (towards the player). New random trees placed at the back (horizon)
3. **Rendering**: Each depth row is drawn at a different Y position on screen, with different scaling
4. **Playing area size**: $01A0 (small) or $02E0 (large) determines the scroll depth

The enemy bikes also use perspective — they have 4 sprite sizes for different distances. The photon bolt shrinks as it travels "into" the screen.

## Risk Areas

1. **Perspective rendering**: Getting the 3D effect right requires exact replication of the tree buffer manipulation and screen mapping. Don't guess — trace the `$5E00` routine byte by byte.
2. **Collision detection**: Tree collision uses the actual pixel data on screen, not abstract coordinates.
3. **Frame timing**: The main loop uses EI/DI for brief interrupt windows, suggesting 50 FPS PAL timing.
4. **Audio**: Use oto from day one.
