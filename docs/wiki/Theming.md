# Theming

`wachat`'s design system lives in [`internal/ui/theme.go`](https://github.com/jdtoon/wachat/blob/main/internal/ui/theme.go). It is the single source of truth — both the runtime widgets and the visual specification in [`docs/design.md`](https://github.com/jdtoon/wachat/blob/main/docs/design.md) refer to the same tokens.

## Tokens

A `Theme` is a value type holding five token groups:

| Group | Type | Examples |
|---|---|---|
| `Palette` | `color.NRGBA` | `Canvas`, `Surface`, `Accent`, `BubbleSent`, `Divider` |
| `Type` | `unit.Sp` | `Display` (22), `Title` (16), `Body` (15), `Meta` (12), `Label` (13) |
| `Spacing` | `unit.Dp` | `XXS` (2), `XS` (4), `S` (8), `M` (12), `L` (16), `XL` (24), `XXL` (32) |
| `Radius` | `unit.Dp` | `Bubble` (14), `Card` (12), `Button` (10) |
| `Motion` | `time.Duration` | `Fast` (120ms), `Base` (180ms), `Enter` (220ms) + `Reduced` bool |

Plus a `Density` enum (`Comfortable` / `Compact`) that scales row paddings.

## Palettes

- [`LightPalette`](https://github.com/jdtoon/wachat/blob/main/internal/ui/theme_light.go) — refined minimalism, content-first. Near-neutral canvas with a single confident green accent.
- [`DarkPalette`](https://github.com/jdtoon/wachat/blob/main/internal/ui/theme_dark.go) — **true-OLED** black canvas. Saves power on OLED displays; floats the surfaces on `#0E120F`.

Both palettes are tested for WCAG AA contrast — body text on canvas/surface ≥ 4.5:1, button labels on accent ≥ 3:1 (AA Large).

## Typography

[Public Sans](https://github.com/uswds/public-sans) Regular / Medium / SemiBold is embedded under [`internal/ui/fonts/`](https://github.com/jdtoon/wachat/tree/main/internal/ui/fonts) (~250 KB total). It is OFL-licensed; the licence text ships in the same directory.

The Go font collection is registered as a fallback so emoji and scripts that Public Sans doesn't include still render.

## Reduced motion

`Theme.Duration(d)` returns `0` when `Motion.Reduced` is true. Widget code should always go through this helper rather than reading `Motion.Fast` directly — that way the OS's reduce-motion setting is honored in one place.

## Density

`Theme.RowPad()` returns vertical padding for list rows: `Spacing.S` in comfortable mode, `Spacing.XS` in compact mode. Future v0.0.7 work will plumb the toggle into the UI.

## Adding to the theme

When `docs/design.md` adds a token (e.g. a new radius or color), the change must land in:

1. The struct definition in `theme.go`
2. Both `theme_light.go` and `theme_dark.go`
3. A test in `theme_test.go` if it has a contrast or monotonicity invariant
4. This wiki page
