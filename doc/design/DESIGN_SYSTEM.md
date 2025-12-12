# POPSigner Design System

> **POPSigner** — Point-of-Presence signing infrastructure.

---

## 1. Brand Identity

### 1.1 Product Name & Positioning

**POPSigner** — Point-of-Presence signing infrastructure. A distributed signing layer designed to live inline with execution, not behind an API queue.

POPSigner (formerly BanhBaoRing) reflects a clearer articulation of what the system is. The rename signals maturation from playful internal naming to category-defining infrastructure positioning.

### 1.2 Logo Concept

```
   ┌─────────────────────────┐
   │                         │
   │      ◇ POPSigner        │
   │                         │
   └─────────────────────────┘
```

- **Icon:** Geometric mark (diamond/node)—no emoji
- **Wordmark:** "POPSigner" in IBM Plex Sans or similar
- **Avoid:** Bell emoji, playful elements, crypto aesthetics

### 1.3 Taglines

| Context | Tagline |
|---------|---------|
| **Hero** | Point-of-Presence Signing Infrastructure |
| **Sub-hero** | Deploy inline with execution. Keys remain remote. You remain sovereign. |
| **Technical** | Distributed signing for rollups, bots, and infrastructure teams. |
| **One-liner** | Signing at the point of execution. |
| **Positioning** | We sell placement, not speed. Speed is a consequence. |

---

## 2. Value Proposition

### 2.1 Core Principles

| Principle | Description |
|-----------|-------------|
| **Inline Signing** | Signing happens on the execution path, not behind a queue |
| **Sovereignty by Default** | Keys are remote, but you control them. Export anytime. Exit anytime. |
| **Neutral Anchor** | Recovery data anchored to neutral data availability |

### 2.2 What POPSigner Is

- Point-of-Presence signing infrastructure
- A distributed signing layer
- Designed to live next to execution, not behind an API queue

### 2.3 What POPSigner Is Not

- A wallet
- MPC custody
- A consumer crypto product
- A compliance-first enterprise tool

### 2.4 Target Audience

- Senior backend engineers
- Infrastructure teams
- Rollup teams
- Execution bots / market makers

---

## 3. Language Constraints

### 3.1 Forbidden Words

The following words must **NEVER** appear in marketing copy:

- low-latency
- fast / faster
- high-performance
- speed
- throughput
- milliseconds / ms
- zero hops / zero network hops

### 3.2 Approved Replacements

| Instead of | Use |
|------------|-----|
| speed | proximity, inline, on the execution path |
| edge | Point-of-Presence, where systems already run |
| performance | deterministic, predictable, non-blocking |
| scale | parallel, worker-native, burst-ready |

### 3.3 Tone Guidelines

**Sound like:**
- Cloudflare
- Fastly
- Datadog

**Do not sound like:**
- Wallets
- Custody vendors
- Crypto dashboards
- VC pitch decks

---

## 4. Color Palette

> **Aesthetic:** Bloomberg Terminal / HFT Trading Systems
> 
> Think: data-dense, utilitarian, professional. Orange amber accents on near-black.
> No purple. No gradients. No crypto wallet vibes.

### 4.1 Primary Colors (Bloomberg Orange)

```css
:root {
  /* === PRIMARY: Bloomberg Orange/Amber === */
  --primary-50: #fffbeb;
  --primary-100: #fef3c7;
  --primary-200: #fde68a;
  --primary-300: #fcd34d;
  --primary-400: #fbbf24;
  --primary-500: #f59e0b;    /* Main - Amber */
  --primary-600: #d97706;    /* Bloomberg Orange */
  --primary-700: #b45309;
  --primary-800: #92400e;
  --primary-900: #78350f;
  
  /* === ACCENT: Terminal Cyan === */
  --accent-400: #22d3ee;
  --accent-500: #06b6d4;     /* Cyan for data highlights */
  --accent-600: #0891b2;
}
```

### 4.2 Semantic Colors

```css
:root {
  /* Success - Terminal Green */
  --success-400: #4ade80;
  --success-500: #22c55e;
  --success-600: #16a34a;
  
  /* Warning - Amber (same as primary) */
  --warning-400: #fbbf24;
  --warning-500: #f59e0b;
  --warning-600: #d97706;
  
  /* Error - Red */
  --error-400: #f87171;
  --error-500: #ef4444;
  --error-600: #dc2626;
  
  /* Info - Cyan */
  --info-400: #22d3ee;
  --info-500: #06b6d4;
}
```

### 4.3 Dark Theme (Terminal Black - Default)

```css
:root {
  /* Bloomberg Terminal Dark */
  --bg-primary: #000000;     /* True black */
  --bg-secondary: #0a0a0a;   /* Card backgrounds */
  --bg-tertiary: #141414;    /* Elevated surfaces */
  --bg-hover: #1f1f1f;       /* Hover states */
  
  --text-primary: #e5e5e5;   /* Main text - slightly warm */
  --text-secondary: #a3a3a3; /* Muted text */
  --text-tertiary: #737373;  /* Disabled text */
  --text-accent: #f59e0b;    /* Orange accent text */
  
  --border: #262626;         /* Borders - subtle */
  --border-hover: #404040;   /* Hover borders */
  
  /* Data colors */
  --data-positive: #22c55e;  /* Green - up/success */
  --data-negative: #ef4444;  /* Red - down/error */
  --data-neutral: #f59e0b;   /* Orange - highlight */
}
```

### 4.4 Light Theme (Disabled)

POPSigner is dark-mode only. No light theme.

```css
/* Light theme intentionally omitted.
   Terminal systems are dark by default. */
```

---

## 5. Typography

> **Aesthetic:** Terminal-first. Monospace prominence. Data-dense.

### 5.1 Font Stack

```css
:root {
  /* Display - Sharp, utilitarian */
  --font-display: "IBM Plex Sans", "Inter", system-ui, sans-serif;
  
  /* Body - Clean, readable */
  --font-body: "IBM Plex Sans", "Inter", system-ui, sans-serif;
  
  /* Mono - PRIMARY for data, keys, addresses */
  --font-mono: "IBM Plex Mono", "JetBrains Mono", "SF Mono", monospace;
}
```

### 5.2 Typography Rules

- **Headlines:** Sans-serif, but lean toward monospace for technical pages
- **Body:** Sans-serif for readability
- **Data/Keys/Addresses:** ALWAYS monospace
- **Numbers:** Monospace (tabular figures)
- **Code blocks:** Monospace with terminal background

### 5.3 Avoid

- Outfit, Space Grotesk (crypto clichés)
- Playful or decorative fonts
- Rounded, friendly fonts
- Any font with personality

### 5.3 Font Sizes (Tailwind scale)

| Name | Size | Line Height | Use Case |
|------|------|-------------|----------|
| `text-xs` | 12px | 16px | Labels, badges |
| `text-sm` | 14px | 20px | Secondary text |
| `text-base` | 16px | 24px | Body text |
| `text-lg` | 18px | 28px | Lead text |
| `text-xl` | 20px | 28px | Section headers |
| `text-2xl` | 24px | 32px | Card titles |
| `text-3xl` | 30px | 36px | Page headers |
| `text-4xl` | 36px | 40px | Hero subtitle |
| `text-5xl` | 48px | 48px | Hero headline |

---

## 6. Landing Page Design

### 6.1 Hero Section

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  ┌─── NAV ───────────────────────────────────────────────────────────────┐  │
│  │  ◇ POPSigner        Docs  Pricing  GitHub         [Sign In] [Deploy]  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│                                                                             │
│         Point-of-Presence Signing Infrastructure                            │
│                                                                             │
│      A distributed signing layer designed to live inline with               │
│      execution—not behind an API queue.                                     │
│                                                                             │
│      Deploy next to your systems. Keys remain remote.                       │
│      You remain sovereign.                                                  │
│                                                                             │
│          ┌─────────────────────────────────────────────┐                    │
│          │  Deploy POPSigner →                         │                    │
│          └─────────────────────────────────────────────┘                    │
│                                                                             │
│          [Read the Architecture →]                                          │
│                                                                             │
│      (formerly BanhBaoRing)                                                 │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.2 What It Is Section

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│                         A signing layer, not a service.                     │
│                                                                             │
│   POPSigner is Point-of-Presence signing infrastructure. It deploys        │
│   where your systems already run—the same region, the same rack,            │
│   the same execution path.                                                  │
│                                                                             │
│   This isn't custody. This isn't MPC. This is signing at the               │
│   point of execution.                                                       │
│                                                                             │
│   ┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐  │
│   │                     │ │                     │ │                     │  │
│   │   Inline Signing    │ │   Sovereignty       │ │   Neutral Anchor    │  │
│   │                     │ │                     │ │                     │  │
│   │   On the execution  │ │   Export anytime.   │ │   Recovery data     │  │
│   │   path, not behind  │ │   Exit anytime.     │ │   anchored to       │  │
│   │   a queue.          │ │   No lock-in.       │ │   neutral DA.       │  │
│   │                     │ │                     │ │                     │  │
│   └─────────────────────┘ └─────────────────────┘ └─────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.3 Pricing Section

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│      Three deployment models. Choose your isolation level.                  │
│                                                                             │
│   ┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐  │
│   │                     │ │                     │ │                     │  │
│   │  SHARED POPSIGNER   │ │ PRIORITY POPSIGNER  │ │ DEDICATED POPSIGNER │  │
│   │                     │ │  ★ MOST POPULAR     │ │                     │  │
│   │      €49/month      │ │     €499/month      │ │   €19,999/month     │  │
│   │                     │ │                     │ │                     │  │
│   │   • Shared POP      │ │   • Priority lanes  │ │   • Region-pinned   │  │
│   │   • No SLA          │ │   • Region select   │ │   • CPU isolation   │  │
│   │   • Plugins         │ │   • 99.9% SLA       │ │   • 99.99% SLA      │  │
│   │   • Escape hatch    │ │   • Self-serve      │ │   • Manual onboard  │  │
│   │                     │ │                     │ │                     │  │
│   │  [Start with Shared]│ │  [Deploy Priority]  │ │  [Contact Us]       │  │
│   │                     │ │                     │ │                     │  │
│   └─────────────────────┘ └─────────────────────┘ └─────────────────────┘  │
│                                                                             │
│   Self-host option is always free. 100% open source.                        │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.4 CTA Section

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│       Deploy signing infrastructure that lives where you do.                │
│                                                                             │
│                  ┌───────────────────────────────┐                          │
│                  │     Deploy POPSigner →        │                          │
│                  └───────────────────────────────┘                          │
│                                                                             │
│                  [Read Documentation →]                                     │
│                                                                             │
│   ┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐  │
│   │   Open Source       │ │   Built on OpenBao  │ │   Exit by Default   │  │
│   └─────────────────────┘ └─────────────────────┘ └─────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.5 Footer

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  ◇ POPSigner              Product        Developers       Company          │
│                           --------       -----------      --------         │
│  Point-of-Presence        Pricing        Documentation    About            │
│  Signing Infrastructure   Docs           SDK (Go)         Contact          │
│                           GitHub         SDK (Rust)                        │
│                           Status         API Reference                     │
│                                                                             │
│  ─────────────────────────────────────────────────────────────────────────  │
│                                                                             │
│  © 2025 POPSigner. Open source under Apache 2.0.      [GitHub] [Discord]   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 7. Component Library

### 7.1 Buttons

```html
<!-- Primary -->
<button class="
  bg-primary-600 hover:bg-primary-700
  text-white font-medium 
  px-6 py-3 rounded-lg
  transition-colors duration-200
">
  Deploy POPSigner →
</button>

<!-- Secondary - outline -->
<button class="
  border border-zinc-600 
  text-zinc-300
  px-5 py-2.5 rounded-lg
  hover:bg-zinc-800 hover:border-zinc-500
  transition-colors duration-200
">
  Read Documentation
</button>

<!-- Ghost -->
<button class="
  text-zinc-400 
  px-4 py-2 rounded-lg
  hover:text-white hover:bg-zinc-800
  transition-colors duration-200
">
  Cancel
</button>
```

### 7.2 Cards

```html
<!-- Feature card -->
<div class="
  bg-zinc-900
  border border-zinc-800 rounded-xl p-6
  hover:border-zinc-700
  transition-colors duration-200
">
  <h3 class="text-lg font-medium text-white mb-2">Inline Signing</h3>
  <p class="text-zinc-400 text-sm">On the execution path, not behind a queue.</p>
</div>
```

### 7.3 Code Blocks

```html
<!-- Code block -->
<div class="relative">
  <div class="absolute top-3 right-3 flex items-center gap-2">
    <span class="text-xs text-zinc-500 uppercase font-mono">Go</span>
    <button class="text-zinc-400 hover:text-white p-1.5 rounded">📋</button>
  </div>
  <pre class="bg-zinc-950 border border-zinc-800 rounded-lg p-6 overflow-x-auto">
    <code class="text-sm text-zinc-300">
client := popsigner.NewClient("psk_xxx")
sig, _ := client.Sign.Sign(ctx, keyID, txBytes, false)
    </code>
  </pre>
</div>
```

---

## 8. Page Layouts

### 8.1 Landing Page Layout

```
┌───────────────────────────────────────────────────────────────┐
│ Nav (fixed, minimal)                                          │
├───────────────────────────────────────────────────────────────┤
│ Hero (centered, text-focused)                                 │
├───────────────────────────────────────────────────────────────┤
│ What It Is (principles)                                       │
├───────────────────────────────────────────────────────────────┤
│ Architecture (diagram + code)                                 │
├───────────────────────────────────────────────────────────────┤
│ Exit Guarantee                                                │
├───────────────────────────────────────────────────────────────┤
│ Features (streamlined)                                        │
├───────────────────────────────────────────────────────────────┤
│ Pricing (3 tiers)                                             │
├───────────────────────────────────────────────────────────────┤
│ Final CTA                                                     │
├───────────────────────────────────────────────────────────────┤
│ Footer                                                        │
└───────────────────────────────────────────────────────────────┘
```

### 8.2 Dashboard Layout

```
┌───────────────────────────────────────────────────────────────┐
│ Top Bar (logo, search, user menu)                             │
├───────────────┬───────────────────────────────────────────────┤
│               │                                               │
│   Sidebar     │   Main Content                                │
│               │                                               │
│   Overview    │   ┌─────────────────────────────────────────┐ │
│   Keys        │   │  Page Content                           │ │
│   Usage       │   │                                         │ │
│   Audit       │   │                                         │ │
│   Settings    │   │                                         │ │
│               │   └─────────────────────────────────────────┘ │
│               │                                               │
└───────────────┴───────────────────────────────────────────────┘
```

---

## 9. Animation & Motion

### 9.1 Transition Defaults

```css
/* Keep animations subtle and professional */
.transition-fast { transition-duration: 150ms; }
.transition-normal { transition-duration: 200ms; }

/* Easing */
.ease-smooth { transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1); }
```

### 9.2 Hover Effects

```css
/* Button hover - subtle */
.btn:hover {
  background-color: var(--primary-700);
}

/* Card hover - border only */
.card:hover {
  border-color: var(--border-hover);
}
```

---

## 10. Accessibility

### 10.1 Requirements

- WCAG 2.1 AA compliance
- Color contrast ratio ≥ 4.5:1
- Full keyboard navigation
- Focus indicators on all interactive elements
- Screen reader support (ARIA labels)
- Reduced motion support (`prefers-reduced-motion`)

### 10.2 Focus Styles

```css
*:focus-visible {
  outline: 2px solid var(--primary-500);
  outline-offset: 2px;
}
```

---

## 11. Implementation Checklist

### Phase 1: Foundation
- [ ] Update branding from BanhBaoRing to POPSigner
- [ ] Remove bell emoji from all components
- [ ] Update color scheme to professional palette
- [ ] Update typography to IBM Plex Sans

### Phase 2: Landing Page
- [ ] Update hero copy (remove time claims)
- [ ] Add "What It Is" section
- [ ] Add "Exit Guarantee" section
- [ ] Update pricing to €49/€499/€19,999
- [ ] Update footer

### Phase 3: Dashboard
- [ ] Update branding throughout
- [ ] Add "Export Key" functionality visibility
- [ ] Update billing page with new tiers

### Phase 4: Documentation
- [ ] Update all docs with POPSigner naming
- [ ] Remove forbidden language throughout
- [ ] Update code examples with new API prefix (psk_)

---

## 12. References

- [Tailwind CSS](https://tailwindcss.com/docs)
- [HTMX](https://htmx.org/docs/)
- [Alpine.js](https://alpinejs.dev/start-here)
- [templ](https://templ.guide/)
