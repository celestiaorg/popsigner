# BanhBaoRing Design Documentation

> 🔔 **"Ring ring!"** - The sound of secure keys being delivered.

---

## Overview

This directory contains all design-related documentation for the BanhBaoRing platform, including visual identity, component library, and page wireframes.

---

## Documents

| Document | Description |
|----------|-------------|
| [DESIGN_SYSTEM.md](./DESIGN_SYSTEM.md) | Complete design system with brand identity, colors, typography, components, and page layouts |

---

## Brand Summary

### Product Name
**BanhBaoRing** - Named after the distinctive "ring ring!" of Vietnamese bánh bao street vendors. That familiar sound means something trusted and good is coming.

### Tagline
> **"Ring ring! Sign where your infra lives."**

### Value Proposition (3-Second Pitch)

**The Pain:**
- 🔒 **Vendor lock-in** - Stuck with AWS KMS or expensive enterprise vaults
- 🧩 **No customizability** - Need secp256k1? "Not supported."
- 🌍 **Remote signers are... remote** - Your vault is across the world from your nodes
- 😫 **Tedious local setup** - Config files, passphrases, backup stress

**BanhBaoRing:** Point of Presence deployment. Your keys, next to your nodes. Same datacenter. Built on OpenBao.

---

## Visual Identity Quick Reference

### Colors

```css
/* Primary: Celestia-inspired purple */
--primary-500: #d946ef;
--primary-600: #a855f7;

/* Accent: Warm coral/orange (bánh bao warmth) */
--accent-500: #f97316;

/* Background: Deep purple-black */
--bg-primary: #0c0a14;
--bg-secondary: #1a1625;
```

### Typography

| Usage | Font |
|-------|------|
| Headings | Outfit |
| Body | Plus Jakarta Sans |
| Code | JetBrains Mono |

### Key UI Patterns

- **Dark theme primary** - Deep purple-black backgrounds
- **Gradient CTAs** - Purple → Orange gradients for primary actions
- **Glow effects** - Subtle purple glow on hover
- **Glass morphism** - Blur backdrops for nav and modals

---

## Page Overview

| Page | Purpose | Key Sections |
|------|---------|--------------|
| **Landing** | Convert visitors → signups | Hero, Problem, Solution, Features, Pricing, CTA |
| **Login** | Authentication | OAuth buttons (GitHub, Google), email form |
| **Signup** | Registration | OAuth, email, org name |
| **Dashboard** | Overview | Stats cards, recent activity, quick actions |
| **Keys** | Key management | Key list, search/filter, create modal |
| **Key Detail** | Individual key | Signature chart, test sign, danger zone |
| **Settings** | Configuration | Profile, team, API keys, billing |
| **Audit** | Compliance | Event log, filters, export |

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Templates | templ |
| Styling | Tailwind CSS + DaisyUI |
| Interactivity | HTMX 2.0 |
| Reactivity | Alpine.js 3 |
| Charts | Chart.js 4 |
| Syntax | Highlight.js 11 |

---

## Related Documents

- **Product Requirements:** [`../product/PRD_DASHBOARD.md`](../product/PRD_DASHBOARD.md)
- **Implementation:** [`../implementation/IMPL_11_DASHBOARD.md`](../implementation/IMPL_11_DASHBOARD.md)
- **Product Overview:** [`../product/README.md`](../product/README.md)

