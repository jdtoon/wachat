# DESIGN.md — UI & UX

> Companion to `CLAUDE.md` (architecture + rules) and `ROADMAP.md` (features +
> budgets). This file defines **how it looks and feels**. The goal: instantly
> familiar to a WhatsApp user, but calmer, faster, and more refined — and every
> visual choice must respect the performance north star. Beauty here comes from
> precision and restraint, not weight.

---

## 0. Direction

**"Quiet & Instant."** Refined minimalism, content-first. The app is something
you stare at all day, so it stays calm: a near-neutral canvas, one confident
accent, generous breathing room, type that's a pleasure at small sizes. The one
thing a user should remember is that it feels *impossibly fast and unbothered*
compared to the official client — scrolling is glassy, sending is instant,
search is immediate, nothing churns.

**Familiar, not cloned.** Keep the patterns WhatsApp users have in muscle memory
(two-pane layout, message bubbles, bottom composer, swipe/hover to reply, the
chat-list metaphor). Give it your **own name, icon, and palette** — do not copy
WhatsApp's logo, wordmark, or exact brand green. Familiar ergonomics are fair
game; brand assets are not.

---

## 1. Design tokens

Tokens are the single source of truth. In Gio there's no CSS — define these once
as a Go `Theme` struct (colors as `color.NRGBA`, sizes as `unit.Dp`, text as
`unit.Sp`) and pass it down. Swapping the struct swaps the theme instantly with
zero reflow cost.

### Color — Light

| Token | Hex | Use |
|---|---|---|
| `canvas` | `#F4F5F3` | App background |
| `surface` | `#FFFFFF` | Cards, composer, list |
| `surfaceRaised` | `#FFFFFF` + shadow | Menus, dialogs |
| `textPrimary` | `#11140F` | Body text |
| `textSecondary` | `#6B7167` | Timestamps, previews |
| `accent` | `#1F8F6B` | Actions, sent bubble, focus |
| `accentText` | `#FFFFFF` | Text on accent |
| `bubbleSent` | `#D6F3E4` | Outgoing message |
| `bubbleRecv` | `#FFFFFF` | Incoming message |
| `divider` | `#E4E6E1` | Hairlines |
| `unread` | `#1F8F6B` | Badges |

### Color — Dark (true OLED)

| Token | Hex | Use |
|---|---|---|
| `canvas` | `#000000` | App background (OLED-black) |
| `surface` | `#0E120F` | Cards, composer, list |
| `surfaceRaised` | `#171C18` | Menus, dialogs |
| `textPrimary` | `#ECEFEA` | Body text |
| `textSecondary` | `#8A9187` | Timestamps, previews |
| `accent` | `#3FB68A` | Actions, focus |
| `accentText` | `#04130C` | Text on accent |
| `bubbleSent` | `#13402F` | Outgoing message |
| `bubbleRecv` | `#171C18` | Incoming message |
| `divider` | `#23291F` | Hairlines |

Treat the accent hue as *your* signature — pick a green that's clearly not
WhatsApp's. Keep exactly one saturated accent; everything else is neutral.

### Type

Avoid generic system fonts. Pick a characterful-but-highly-legible grotesque
that stays crisp at 13–15px, embed the TTF in the binary (Gio loads custom faces
via the shaper), and pair it with a good emoji font for fallback.

Open-licensed candidates (pick one for UI/body): **Hanken Grotesk**, **Public
Sans**, or **IBM Plex Sans**. Use a mono (**IBM Plex Mono**) only for code spans
and optionally timestamps. Confirm the license permits redistribution in an
open-source binary.

| Token | Size (sp) | Weight | Use |
|---|---|---|---|
| `display` | 22 | 600 | Empty-state / onboarding |
| `title` | 16 | 600 | Header name, dialog titles |
| `body` | 15 | 400 | Message text |
| `meta` | 12 | 400 | Timestamps, receipts, previews |
| `label` | 13 | 500 | Buttons, chips |

Line height ~1.35 for body. Never justify; left-align (RTL-aware).

### Spacing, radius, elevation

- **Spacing scale (dp):** 2, 4, 8, 12, 16, 24, 32. Compose from these only.
- **Radius:** bubbles 14, cards 12, chips/avatars full, buttons 10.
- **Elevation:** prefer flat surfaces + a single soft shadow for true overlays
  (menus/dialogs) only. **Avoid backdrop-blur and large multi-layer shadows** —
  they're per-frame GPU cost and fight the perf budget. A 1px divider beats a
  shadow for in-flow separation.

### Motion

Short, cheap, purposeful. Honor a **reduced-motion** setting (disables all
non-essential animation).

| Token | Duration | Easing | Use |
|---|---|---|---|
| `fast` | 120ms | ease-out | Hover, press, toggle |
| `base` | 180ms | ease-out | Panel/menu open, send |
| `enter` | 220ms | ease-out-back (subtle) | New message slide-in |

Frame-cap all animation; never animate off-screen rows; release any animation
state when a row scrolls away.

---

## 2. Layout

Two-pane, with a responsive collapse.

```
┌───────────────┬───────────────────────────────┐
│  SIDEBAR      │  CONVERSATION                  │
│  ┌─────────┐  │  ┌──────────────────────────┐  │
│  │ search/ │  │  │ header: name · presence  │  │
│  │ ⌘K bar  │  │  │         · actions        │  │
│  ├─────────┤  │  ├──────────────────────────┤  │
│  │ chat    │  │  │                          │  │
│  │ list    │  │  │   message scroll area    │  │
│  │ (virt.) │  │  │   (virtualized)          │  │
│  │         │  │  │                          │  │
│  ├─────────┤  │  ├──────────────────────────┤  │
│  │ account │  │  │ composer                 │  │
│  └─────────┘  │  └──────────────────────────┘  │
└───────────────┴───────────────────────────────┘
```

- **Sidebar width:** ~320–360dp, user-resizable, remembered.
- **Narrow window (< ~760dp):** collapse to single pane (list ⇄ conversation),
  with back navigation. This is also the lightest mode.
- **Density toggle:** `comfortable` (default) and `compact` (tighter list rows +
  bubbles) — a power-user nicety the official app lacks.

---

## 3. Components

### Message bubble
- Sent: right-aligned, `bubbleSent`. Received: left-aligned, `bubbleRecv`.
- **Grouping:** consecutive messages from the same sender within ~5 min collapse
  the avatar/name; only the group's last bubble carries the timestamp + receipt.
- **Tail-less, rounded** bubbles (a clean modern refinement over hard tails);
  group corners soften toward the sender.
- **Meta row:** time + receipt state (sent · delivered · read) bottom-inner,
  `meta` token, low contrast so it never competes with text.
- **Reply quote:** a thin accent-bordered block above the text showing the
  quoted snippet; tap to jump to the original.
- **Reactions:** a small chip cluster overlapping the bubble's bottom edge.
- **Link preview:** compact card (title, host, small thumbnail) — thumbnail
  decoded on-visible, never blocking.

### Media in bubbles
- Images/video: rounded thumbnail with fixed aspect box; **lazy decode/downscale
  on visible, release on scroll-away** (ROADMAP `[HEAVY]` rule). Tap → full
  viewer.
- Voice notes: waveform + play head; waveform computed once and cached.
- Documents: icon chip with name/size; download-on-demand.

### Chat-list row
- Avatar · name (title) · last-message preview (one line, `textSecondary`) ·
  time · unread badge. Pin and mute glyphs inline. Fixed/predictable height (see
  §5). Selected row uses a subtle `surface` tint, not a heavy fill.

### Composer
- Auto-growing multiline input (caps at ~6 lines, then scrolls), attach button,
  emoji button, send button (accent). Enter sends, Shift+Enter newline (user-
  configurable). **Optimistic send:** the bubble appears instantly in a
  "sending" state, then resolves to "sent" — this is core to the "instant" feel.

### Command palette (⌘K / Ctrl+K) — signature feature
- Center-screen overlay: fuzzy-search chats, jump to actions, global message
  search. Keyboard-first, no mouse required. This is the headline "better than
  WhatsApp" interaction; make it fast and beautiful.

### Search
- Inline results with the matched term highlighted in the accent; show chat +
  snippet + date; Enter jumps to the message in context. Backed by FTS5, so it's
  instant — lean into that.

### Context / message menu
- Right-click or hover-affordance: reply, react, copy, forward, edit (own),
  delete-for-everyone (own), star. Swipe-to-reply gesture also supported.

### States
- **Pairing:** clean QR screen with status ("waiting", "syncing…", "ready").
- **Connection banner:** unobtrusive top strip for reconnecting/offline.
- **Empty conversation / no chat selected:** quiet `display`-type illustration
  or wordmark, not clutter.

---

## 4. The "better than WhatsApp" UX list

Concrete frictions to fix (tie to ROADMAP Phase 3):
- **Keyboard-first everything** — palette, chat switching, message navigation;
  the official app is mouse-bound.
- **Instant search with highlighting + jump-to-date** — theirs is slow and shallow.
- **Density modes** and resizable sidebar — theirs is fixed.
- **Frictionless multi-account** in one window — theirs is clumsy.
- **Optimistic, never-janky send** and glassy scrollback to any point in history.
- **Tasteful, low-noise notifications** with per-chat rules.
- **Calls:** show incoming-call notifications + a call log only (whatsmeow can't
  place/answer calls — see ROADMAP decisions). Design the log; do not design a
  dialer.

---

## 5. Performance-aware design rules (non-negotiable)

The UI must serve the architecture, not strain it:
- **Predictable row heights.** Design list/message rows so height is computable
  without rendering (estimate, then cache measured heights). Avoid layouts that
  force measuring every row — that breaks virtualization's flat cost.
- **No off-screen work.** No effect that needs rendering hidden content. Media,
  waveforms, link thumbnails, animations: load-on-visible, release-on-hide.
- **Cheap pixels.** No backdrop blur, minimal/soft shadows, flat fills. Effects
  that cost per frame are banned by default; justify any exception.
- **Theme = data.** Light/dark/density are token structs swapped at runtime; no
  layout recomputation, no asset reload.
- **Motion is frame-capped and reduced-motion-aware**, and never runs for
  off-screen elements.

If a desired visual can't be done within these rules, raise it — don't smuggle in
the heavy version (CLAUDE.md §11).

---

## 6. Mapping to Gio

- **Tokens → types:** colors `color.NRGBA`; spacing/sizing `unit.Dp`; text
  `unit.Sp`. Hold them in one `Theme` struct; derive a `material.Theme` from it
  but override colors/typography to match these tokens.
- **Fonts:** embed TTFs and register a custom `text.Shaper` face collection; set
  an emoji fallback face. Do not rely on the default Go fonts for body — they're
  fine but generic.
- **Immediate-mode reminder:** styles are computed each frame from the Theme +
  state; there is no retained widget tree to restyle. Build small style helpers
  (`bubble(th, kind)`, `chatRow(th, density)`) that read tokens.
- **Rounded surfaces:** use `clip.RRect` for bubbles/cards; keep corner radii on
  the radius tokens.
- **Accessibility:** set Gio semantics (labels/roles) on interactive widgets;
  ensure focus order is logical for keyboard-first use.

---

## 7. Accessibility & polish checklist

- [ ] Contrast meets WCAG AA (body ≥ 4.5:1, large text ≥ 3:1) in both themes
- [ ] Visible focus ring (accent) on every focusable element
- [ ] Full keyboard navigation; nothing mouse-only
- [ ] Respects OS font-scale and a reduced-motion setting
- [ ] Screen-reader labels on icons/avatars/actions
- [ ] Hit targets ≥ 32dp; comfortable spacing in `comfortable` density
- [ ] RTL-aware layout and text alignment

---

## 8. Quick "do / don't"

**Do:** one accent, neutral canvas, generous space, instant feedback,
keyboard-first, flat and fast, predictable row heights.

**Don't:** copy WhatsApp's brand assets, use generic system fonts, add backdrop
blur or heavy shadows, animate off-screen, measure-all-rows layouts, multiple
competing accent colors, anything that makes it feel heavier than it is.