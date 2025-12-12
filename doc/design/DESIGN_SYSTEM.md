# BanhBaoRing Design System

> 🔔 **"Ring ring!"** - Secure keys, delivered.

---

## 1. Brand Identity

### 1.1 Product Name & Origin

**BanhBaoRing** - Named after the distinctive "ring ring!" of Vietnamese bánh bao street vendors cycling through neighborhoods. That familiar sound means something good is coming - warm, fresh bánh bao delivered right to you.

BanhBaoRing brings that same trusted, convenient experience to key management: **secure signing delivered right to your application.**

### 1.2 Logo Concept

```
   ┌─────────────────────────┐
   │                         │
   │      🔔                 │
   │   BanhBaoRing          │
   │                         │
   └─────────────────────────┘
```

- **Icon:** Bell (🔔) - The vendor's bell
- **Primary Mark:** Bell with subtle key/lock integrated
- **Wordmark:** "BanhBaoRing" in Outfit font

### 1.3 Taglines

| Context | Tagline |
|---------|---------|
| **Hero** | Ring ring! Sign where your infra lives. |
| **Sub-hero** | Point of Presence deployment. Your keys, next to your nodes. Built on OpenBao. |
| **Technical** | Edge-deployed signing for Celestia & Cosmos. 100+ signs/sec. Open source. |
| **One-liner** | The remote signer that deploys where you need it. |
| **POP-focused** | Point of Presence signing. Zero network hops. Your vault, your region. |

---

## 2. Value Proposition - TL;DR

### 2.1 The Pain (3 seconds)

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  🔒 VENDOR LOCK-IN                                              │
│     Trapped with AWS KMS or HashiCorp's pricing?                │
│                                                                 │
│  🧩 NO CUSTOMIZABILITY                                          │
│     Need secp256k1? Sorry, not supported.                       │
│                                                                 │
│  🐢 LOW PERFORMANCE                                             │
│     100+ signs/sec? Good luck with that latency.                │
│                                                                 │
│  😫 TEDIOUS LOCAL SETUP                                         │
│     Another keyring config. Another passphrase to remember.     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 The Solution (5 seconds)

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  🔔 BANHBAORING                                                 │
│                                                                 │
│  Open source. No lock-in.                                       │
│  Plugin system. Your algorithm, supported.                      │
│  100+ signs/sec. Parallel workers included.                     │
│  5-minute setup. One API call to sign.                          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2.3 USP Grid

| USP | Description | Icon |
|-----|-------------|------|
| **Point of Presence** | Deploy next to your nodes. Same region, same datacenter. Zero network hops. | 📍 |
| **Deploy in Minutes** | Sign up → Create key → First signature in under 5 minutes. | 🚀 |
| **No Vendor Lock-in** | 100% open source. Built on OpenBao. Self-host or use our cloud. | 🔓 |
| **Plugin Architecture** | secp256k1 today, your custom algorithm tomorrow. | 🧩 |
| **Vault-Grade Security** | Keys never leave OpenBao. Full audit trail. | 🔐 |
| **Drop-in SDK** | One line of Go/Rust. Works with Cosmos SDK keyring interface. | 🔗 |

---

## 3. Color Palette

### 3.1 Primary Colors

```css
:root {
  /* === PRIMARY: Celestia-inspired purple === */
  --primary-50: #fdf4ff;
  --primary-100: #fae8ff;
  --primary-200: #f5d0fe;
  --primary-300: #f0abfc;
  --primary-400: #e879f9;
  --primary-500: #d946ef;    /* Main purple */
  --primary-600: #a855f7;    /* Celestia purple */
  --primary-700: #7e22ce;
  --primary-800: #6b21a8;
  --primary-900: #581c87;
  
  /* === ACCENT: Warm coral/orange (bánh bao warmth) === */
  --accent-50: #fff7ed;
  --accent-100: #ffedd5;
  --accent-200: #fed7aa;
  --accent-300: #fdba74;
  --accent-400: #fb923c;
  --accent-500: #f97316;     /* Main orange */
  --accent-600: #ea580c;
  --accent-700: #c2410c;
  
  /* === SECONDARY: Celestia cyan === */
  --secondary-400: #22d3ee;
  --secondary-500: #06b6d4;
  --secondary-600: #0891b2;
}
```

### 3.2 Semantic Colors

```css
:root {
  /* Success */
  --success-400: #4ade80;
  --success-500: #22c55e;
  --success-600: #16a34a;
  
  /* Warning */
  --warning-400: #facc15;
  --warning-500: #eab308;
  --warning-600: #ca8a04;
  
  /* Error */
  --error-400: #f87171;
  --error-500: #ef4444;
  --error-600: #dc2626;
}
```

### 3.3 Dark Theme (Primary)

```css
:root {
  /* Dark mode - default */
  --bg-primary: #0c0a14;     /* Deep purple-black */
  --bg-secondary: #1a1625;   /* Card backgrounds */
  --bg-tertiary: #2d2640;    /* Elevated surfaces */
  --bg-hover: #3d3555;       /* Hover states */
  
  --text-primary: #faf5ff;   /* Main text */
  --text-secondary: #c4b5d6; /* Muted text */
  --text-tertiary: #8b7fa3;  /* Disabled text */
  
  --border: #4a3f5c;         /* Borders */
  --border-hover: #6b5b8a;   /* Hover borders */
}
```

### 3.4 Light Theme (Secondary)

```css
[data-theme="light"] {
  --bg-primary: #faf5ff;
  --bg-secondary: #ffffff;
  --bg-tertiary: #f3e8ff;
  
  --text-primary: #1a1625;
  --text-secondary: #4a3f5c;
  
  --border: #e9d5ff;
}
```

---

## 4. Typography

### 4.1 Font Stack

```css
:root {
  /* Display - headings, hero text */
  --font-display: "Outfit", "Sora", system-ui, sans-serif;
  
  /* Body - paragraphs, UI text */
  --font-body: "Plus Jakarta Sans", "Inter", system-ui, sans-serif;
  
  /* Monospace - code, addresses, keys */
  --font-mono: "JetBrains Mono", "Fira Code", "SF Mono", monospace;
}
```

### 4.2 Font Sizes (Tailwind scale)

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
| `text-6xl` | 60px | 60px | Landing hero |
| `text-7xl` | 72px | 72px | Statement |

### 4.3 Font Loading

```html
<!-- Google Fonts CDN -->
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500&family=Outfit:wght@400;500;600;700&family=Plus+Jakarta+Sans:wght@400;500;600;700&display=swap" rel="stylesheet">
```

---

## 5. Landing Page Design

### 5.1 Hero Section

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  ┌─── NAV ───────────────────────────────────────────────────────────────┐  │
│  │  🔔 BanhBaoRing     Features  Pricing  Docs         [Login] [Sign Up] │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│                                                                             │
│                      🔔 Ring ring!                                          │
│                                                                             │
│            Sign where your infra lives.                                     │
│                                                                             │
│      Point of Presence key management for sovereign rollups.                │
│      Deploy next to your nodes. Built on OpenBao. Open source.              │
│                                                                             │
│          ┌─────────────────────────────────────────────┐                    │
│          │  ▶  Get Started Free                        │                    │
│          └─────────────────────────────────────────────┘                    │
│                                                                             │
│           "Your keys, in your region, next to your nodes"                   │
│                                                                             │
│         [Rollup Logo 1]  [Rollup Logo 2]  [Rollup Logo 3]                  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.2 Problem Section (The Pain)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│                   Current key management solutions suck.                    │
│                                                                             │
│   ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐              │
│   │                 │ │                 │ │                 │              │
│   │  🔒 Vendor      │ │  🧩 No Custom   │ │  🐢 Slow        │              │
│   │     Lock-in    │ │     Algorithms  │ │                 │              │
│   │                 │ │                 │ │  Need 100+      │              │
│   │  AWS KMS? Vault │ │  Need secp256k1?│ │  signs/sec?     │              │
│   │  enterprise?    │ │  "Not supported"│ │  "Good luck"    │              │
│   │  Good luck      │ │                 │ │                 │              │
│   │  leaving.       │ │                 │ │                 │              │
│   └─────────────────┘ └─────────────────┘ └─────────────────┘              │
│                                                                             │
│                        ┌─────────────────┐                                  │
│                        │  😫 Tedious     │                                  │
│                        │     Setup       │                                  │
│                        │                 │                                  │
│                        │  Local keyring? │                                  │
│                        │  Config files,  │                                  │
│                        │  passphrases,   │                                  │
│                        │  backup stress. │                                  │
│                        └─────────────────┘                                  │
│                                                                             │
│                         Sound familiar?                                     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.3 Solution Section (The Fix)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│                         BanhBaoRing fixes all of it.                        │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                                                                     │   │
│   │     client := banhbaoring.NewClient("bbr_xxx")                      │   │
│   │                                                                     │   │
│   │     sig, _ := client.Keys.Sign(ctx, "sequencer", txBytes)           │   │
│   │                                                                     │   │
│   │     // Same region. Zero hops. Keys never touched.                  │   │
│   │                                                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│   ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐              │
│   │  📍 POP Deploy  │ │  🚀 Deploy Fast │ │  🔓 Open Source │              │
│   │                 │ │                 │ │                 │              │
│   │  Next to your   │ │  5 min to first │ │  Built on       │              │
│   │  nodes. Same    │ │  signature      │ │  OpenBao        │              │
│   │  datacenter.    │ │                 │ │                 │              │
│   └─────────────────┘ └─────────────────┘ └─────────────────┘              │
│                                                                             │
│                        ┌─────────────────┐                                  │
│                        │  🧩 Plugins     │                                  │
│                        │                 │                                  │
│                        │  secp256k1 now  │                                  │
│                        │  Your algo next │                                  │
│                        │                 │                                  │
│                        └─────────────────┘                                  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.4 How It Works Section

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│                         From signup to signature                            │
│                              in 5 minutes                                   │
│                                                                             │
│   ┌───────────┐           ┌───────────┐           ┌───────────┐            │
│   │     1     │    →      │     2     │    →      │     3     │            │
│   │           │           │           │           │           │            │
│   │  Sign up  │           │  Create   │           │ Integrate │            │
│   │  (OAuth)  │           │   key     │           │   SDK     │            │
│   │           │           │           │           │           │            │
│   │  30 sec   │           │  1 min    │           │  2 min    │            │
│   └───────────┘           └───────────┘           └───────────┘            │
│                                                                             │
│                              Done! 🎉                                       │
│                                                                             │
│                   Your sequencer is now secure.                             │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.5 Features Grid

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│                         Everything you need                                 │
│                                                                             │
│   ┌─────────────────────────┐    ┌─────────────────────────┐               │
│   │ 📍 Point of Presence    │    │ 🚀 Deploy in Minutes    │               │
│   │                         │    │                         │               │
│   │ Deploy next to your     │    │ 5 min to first sig.     │               │
│   │ nodes. Same datacenter. │    │ No local config pain.   │               │
│   └─────────────────────────┘    └─────────────────────────┘               │
│                                                                             │
│   ┌─────────────────────────┐    ┌─────────────────────────┐               │
│   │ 🔓 100% Open Source     │    │ 🧩 Plugin Architecture  │               │
│   │                         │    │                         │               │
│   │ Built on OpenBao.       │    │ secp256k1 built-in.     │               │
│   │ Self-host or use cloud. │    │ Add your own algorithms.│               │
│   └─────────────────────────┘    └─────────────────────────┘               │
│                                                                             │
│   ┌─────────────────────────┐    ┌─────────────────────────┐               │
│   │ 🔐 Vault-Grade Security │    │ 📊 Full Audit Trail     │               │
│   │                         │    │                         │               │
│   │ Keys never leave vault. │    │ Every signature logged. │               │
│   │ Powered by OpenBao.     │    │ Compliance ready.       │               │
│   └─────────────────────────┘    └─────────────────────────┘               │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.6 Pricing Section

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│                           Simple pricing                                    │
│                     Pay as your rollup grows                                │
│                                                                             │
│   ┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐  │
│   │                     │ │                     │ │                     │  │
│   │        FREE         │ │        PRO          │ │     ENTERPRISE      │  │
│   │                     │ │                     │ │                     │  │
│   │         $0          │ │     $49/month       │ │      Custom         │  │
│   │                     │ │                     │ │                     │  │
│   │   • 3 keys          │ │   • 25 keys         │ │   • Unlimited keys  │  │
│   │   • 10K signs/mo    │ │   • 500K signs/mo   │ │   • Unlimited signs │  │
│   │   • 1 namespace     │ │   • 5 namespaces    │ │   • Dedicated vault │  │
│   │   • 7 day audit     │ │   • 90 day audit    │ │   • 99.99% SLA      │  │
│   │                     │ │   • 99.9% SLA       │ │   • SSO / SAML      │  │
│   │   [Get Started]     │ │   [Start Trial]     │ │   [Contact Us]      │  │
│   │                     │ │                     │ │                     │  │
│   └─────────────────────┘ └─────────────────────┘ └─────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.7 Social Proof / Testimonials

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│                   Trusted by leading rollup teams                           │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                                                                     │   │
│   │  "We migrated our sequencer keys in 10 minutes. No more .env       │   │
│   │   nightmares. BanhBaoRing just works."                              │   │
│   │                                                                     │   │
│   │                     — CTO, [Rollup Name]                            │   │
│   │                                                                     │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│                                                                             │
│         45,000+              500+                50+                        │
│       Signatures/day      Keys managed       Rollups served                 │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.8 CTA Section

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│                                                                             │
│                     🔔 Ready to secure your keys?                           │
│                                                                             │
│              Sign up free. First signature in 5 minutes.                    │
│                                                                             │
│                  ┌───────────────────────────────┐                          │
│                  │     Get Started Free →        │                          │
│                  └───────────────────────────────┘                          │
│                                                                             │
│                  No credit card required.                                   │
│                                                                             │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.9 Footer

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  🔔 BanhBaoRing             Product        Company        Developers        │
│                             --------       --------       -----------       │
│  Secure keys for           Features       About          Documentation     │
│  sovereign rollups.        Pricing        Blog           API Reference     │
│                             Changelog      Careers        SDK (Go, TS)      │
│                             Status         Contact        GitHub            │
│                                                                             │
│  ─────────────────────────────────────────────────────────────────────────  │
│                                                                             │
│  © 2025 BanhBaoRing. All rights reserved.      [Twitter] [GitHub] [Discord]│
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 6. Component Library

### 6.1 Buttons

```html
<!-- Primary - gradient with glow -->
<button class="
  bg-gradient-to-r from-purple-500 to-orange-500
  text-white font-semibold 
  px-6 py-3 rounded-lg
  shadow-lg shadow-purple-500/25
  hover:shadow-purple-500/40 hover:scale-[1.02]
  active:scale-[0.98]
  transition-all duration-200
">
  Get Started Free →
</button>

<!-- Secondary - outline -->
<button class="
  border border-purple-500/50 
  text-purple-300
  px-5 py-2.5 rounded-lg
  hover:bg-purple-500/10 hover:border-purple-500
  transition-all duration-200
">
  View Documentation
</button>

<!-- Ghost -->
<button class="
  text-gray-400 
  px-4 py-2 rounded-lg
  hover:text-white hover:bg-white/5
  transition-all duration-200
">
  Cancel
</button>

<!-- Icon button -->
<button class="
  p-2 rounded-lg
  text-gray-400 hover:text-white
  hover:bg-white/5
  transition-all duration-200
">
  <svg>...</svg>
</button>
```

### 6.2 Cards

```html
<!-- Feature card -->
<div class="
  bg-[#1a1625]/80 backdrop-blur-lg
  border border-[#4a3f5c] rounded-xl p-6
  hover:border-purple-500/50
  hover:shadow-lg hover:shadow-purple-500/10
  transition-all duration-300
">
  <div class="text-3xl mb-4">📍</div>
  <h3 class="text-xl font-semibold text-white mb-2">Point of Presence</h3>
  <p class="text-gray-400">Deploy next to your nodes. Same region. Zero hops.</p>
</div>

<!-- Key card (dashboard) -->
<div class="
  bg-[#1a1625] 
  border-l-4 border-l-emerald-500
  border border-[#4a3f5c] rounded-lg p-4
  hover:bg-[#1a1625]/80 
  transition-all duration-200
">
  <div class="flex justify-between items-start">
    <div>
      <h3 class="text-white font-semibold flex items-center gap-2">
        🔑 sequencer-mainnet
      </h3>
      <p class="text-gray-400 font-mono text-sm mt-1">
        celestia1abc...xyz
      </p>
    </div>
    <span class="text-xs text-purple-400 bg-purple-500/10 px-2 py-1 rounded">
      production
    </span>
  </div>
</div>

<!-- Stat card -->
<div class="
  bg-gradient-to-br from-purple-500/10 to-orange-500/10
  border border-purple-500/20 rounded-xl p-6
">
  <p class="text-gray-400 text-sm mb-1">Signatures Today</p>
  <p class="text-3xl font-bold text-white">45,231</p>
  <p class="text-emerald-400 text-sm mt-2">↑ 12% from yesterday</p>
</div>
```

### 6.3 Navigation

```html
<!-- Top nav -->
<nav class="
  fixed top-0 left-0 right-0 z-50
  bg-[#0c0a14]/80 backdrop-blur-lg
  border-b border-[#4a3f5c]/50
">
  <div class="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
    <!-- Logo -->
    <a href="/" class="flex items-center gap-2 text-xl font-semibold text-white">
      🔔 BanhBaoRing
    </a>
    
    <!-- Links -->
    <div class="hidden md:flex items-center gap-8">
      <a href="/features" class="text-gray-300 hover:text-white transition">Features</a>
      <a href="/pricing" class="text-gray-300 hover:text-white transition">Pricing</a>
      <a href="/docs" class="text-gray-300 hover:text-white transition">Docs</a>
    </div>
    
    <!-- CTAs -->
    <div class="flex items-center gap-4">
      <a href="/login" class="text-gray-300 hover:text-white transition">Log in</a>
      <a href="/signup" class="
        bg-gradient-to-r from-purple-500 to-orange-500
        text-white font-medium px-4 py-2 rounded-lg
        hover:shadow-lg hover:shadow-purple-500/25
        transition-all duration-200
      ">
        Sign up free
      </a>
    </div>
  </div>
</nav>
```

### 6.4 Code Blocks

```html
<!-- Code block with copy button -->
<div class="relative group">
  <div class="absolute top-3 right-3 flex items-center gap-2">
    <span class="text-xs text-gray-500 uppercase font-mono">Go</span>
    <button 
      class="opacity-0 group-hover:opacity-100 transition-opacity
             text-gray-400 hover:text-white p-1.5 rounded-md bg-white/5"
      onclick="copyCode(this)"
    >
      📋
    </button>
  </div>
  <pre class="
    bg-[#0c0a14] border border-[#4a3f5c] rounded-xl p-6 
    overflow-x-auto text-sm
  ">
    <code class="language-go text-gray-300">
client := banhbaoring.NewClient("bbr_xxx")
sig, _ := client.Keys.Sign(ctx, "sequencer", txBytes)
    </code>
  </pre>
</div>
```

### 6.5 Form Inputs

```html
<!-- Text input -->
<div class="space-y-2">
  <label class="text-sm font-medium text-gray-300">Key Name</label>
  <input 
    type="text" 
    placeholder="e.g., sequencer-mainnet"
    class="
      w-full px-4 py-3 rounded-lg
      bg-[#0c0a14] border border-[#4a3f5c]
      text-white placeholder:text-gray-500
      focus:border-purple-500 focus:ring-2 focus:ring-purple-500/20
      transition-all duration-200
    "
  />
</div>

<!-- Select -->
<div class="space-y-2">
  <label class="text-sm font-medium text-gray-300">Namespace</label>
  <select class="
    w-full px-4 py-3 rounded-lg
    bg-[#0c0a14] border border-[#4a3f5c]
    text-white
    focus:border-purple-500 focus:ring-2 focus:ring-purple-500/20
    transition-all duration-200
  ">
    <option value="production">production</option>
    <option value="staging">staging</option>
  </select>
</div>
```

### 6.6 Badges & Pills

```html
<!-- Status badge -->
<span class="inline-flex items-center gap-1 px-2 py-1 rounded text-xs font-medium
             bg-emerald-500/10 text-emerald-400">
  <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
  Active
</span>

<!-- Namespace pill -->
<span class="px-2 py-1 rounded text-xs font-medium
             bg-purple-500/10 text-purple-400">
  production
</span>

<!-- Plan badge -->
<span class="px-2.5 py-1 rounded-full text-xs font-semibold
             bg-gradient-to-r from-purple-500 to-orange-500 text-white">
  PRO
</span>
```

### 6.7 Modal

```html
<!-- Modal (Alpine.js) -->
<div 
  x-data="{ open: false }"
  @keydown.escape.window="open = false"
>
  <!-- Trigger -->
  <button @click="open = true">Open Modal</button>
  
  <!-- Modal -->
  <div 
    x-show="open" 
    x-transition:enter="transition ease-out duration-200"
    x-transition:enter-start="opacity-0"
    x-transition:enter-end="opacity-100"
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
  >
    <!-- Backdrop -->
    <div class="absolute inset-0 bg-black/60" @click="open = false"></div>
    
    <!-- Content -->
    <div class="
      relative bg-[#1a1625] border border-[#4a3f5c] rounded-xl
      max-w-md w-full p-6 shadow-2xl
    ">
      <h2 class="text-xl font-semibold text-white mb-4">Create Key</h2>
      <!-- Form content -->
      <button @click="open = false" class="absolute top-4 right-4 text-gray-400">✕</button>
    </div>
  </div>
</div>
```

### 6.8 Toast Notifications

```html
<!-- Toast (HTMX + Alpine.js) -->
<div 
  id="toast"
  x-data="{ show: false, message: '', type: 'success' }"
  @toast.window="show = true; message = $event.detail.message; type = $event.detail.type; setTimeout(() => show = false, 5000)"
  x-show="show"
  x-transition
  class="fixed bottom-6 right-6 z-50"
>
  <div :class="{
    'bg-emerald-500/90': type === 'success',
    'bg-red-500/90': type === 'error',
    'bg-yellow-500/90': type === 'warning'
  }" class="px-4 py-3 rounded-lg shadow-lg text-white font-medium flex items-center gap-3">
    <span x-text="message"></span>
    <button @click="show = false" class="opacity-60 hover:opacity-100">✕</button>
  </div>
</div>
```

---

## 7. Page Templates

### 7.1 Landing Page Layout

```
┌───────────────────────────────────────────────────────────────┐
│ Nav (fixed, blur backdrop)                                    │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│ Hero (full viewport height, centered content)                 │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│ Problem Section (dark bg, icon cards)                         │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│ Solution Section (gradient bg, code example)                  │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│ How It Works (timeline/steps)                                 │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│ Features Grid (6 cards)                                       │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│ Pricing (3 columns)                                           │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│ Testimonials / Social Proof                                   │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│ Final CTA (gradient bg)                                       │
│                                                               │
├───────────────────────────────────────────────────────────────┤
│ Footer                                                        │
└───────────────────────────────────────────────────────────────┘
```

### 7.2 Dashboard Layout

```
┌───────────────────────────────────────────────────────────────┐
│ Top Bar (logo, search, user menu)                             │
├───────────────┬───────────────────────────────────────────────┤
│               │                                               │
│   Sidebar     │   Main Content                                │
│               │                                               │
│   Overview    │   ┌─────────────────────────────────────────┐ │
│   Keys        │   │  Page Header                            │ │
│   Usage       │   └─────────────────────────────────────────┘ │
│   Audit       │                                               │
│   Settings    │   ┌─────────────────────────────────────────┐ │
│               │   │                                         │ │
│               │   │  Content Area                           │ │
│               │   │                                         │ │
│               │   │                                         │ │
│               │   │                                         │ │
│               │   └─────────────────────────────────────────┘ │
│               │                                               │
└───────────────┴───────────────────────────────────────────────┘
```

### 7.3 Auth Page Layout

```
┌───────────────────────────────────────────────────────────────┐
│                                                               │
│                                                               │
│                    🔔 BanhBaoRing                              │
│                                                               │
│              ┌─────────────────────────────┐                  │
│              │                             │                  │
│              │  Sign in / Sign up form    │                  │
│              │                             │                  │
│              │  [GitHub] [Google]          │                  │
│              │                             │                  │
│              │  ─── or ───                 │                  │
│              │                             │                  │
│              │  Email: [___________]       │                  │
│              │  Password: [___________]    │                  │
│              │                             │                  │
│              │  [Submit Button]            │                  │
│              │                             │                  │
│              └─────────────────────────────┘                  │
│                                                               │
│                  Don't have an account? Sign up               │
│                                                               │
└───────────────────────────────────────────────────────────────┘
```

---

## 8. Animation & Motion

### 8.1 Transition Defaults

```css
/* Default transitions */
.transition-fast { transition-duration: 150ms; }
.transition-normal { transition-duration: 200ms; }
.transition-slow { transition-duration: 300ms; }

/* Easing */
.ease-smooth { transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1); }
.ease-bounce { transition-timing-function: cubic-bezier(0.68, -0.55, 0.265, 1.55); }
```

### 8.2 Hover Effects

```css
/* Button hover */
.btn:hover {
  transform: scale(1.02);
  box-shadow: 0 10px 25px -5px rgba(168, 85, 247, 0.4);
}
.btn:active {
  transform: scale(0.98);
}

/* Card hover */
.card:hover {
  border-color: rgba(168, 85, 247, 0.5);
  box-shadow: 0 0 30px rgba(168, 85, 247, 0.1);
}

/* Link hover */
.nav-link {
  position: relative;
}
.nav-link::after {
  content: '';
  position: absolute;
  bottom: -4px;
  left: 0;
  width: 0;
  height: 2px;
  background: linear-gradient(to right, #a855f7, #f97316);
  transition: width 0.2s;
}
.nav-link:hover::after {
  width: 100%;
}
```

### 8.3 Page Load Animations

```css
/* Staggered fade in */
.animate-fade-in {
  animation: fadeIn 0.5s ease-out forwards;
  opacity: 0;
}

@keyframes fadeIn {
  to { opacity: 1; }
}

/* Stagger children */
.stagger > *:nth-child(1) { animation-delay: 0ms; }
.stagger > *:nth-child(2) { animation-delay: 100ms; }
.stagger > *:nth-child(3) { animation-delay: 200ms; }
.stagger > *:nth-child(4) { animation-delay: 300ms; }
```

### 8.4 HTMX Transitions

```html
<!-- Fade swap -->
<div hx-get="/keys" hx-swap="innerHTML transition:true">
  <!-- Content swapped with fade -->
</div>

<!-- CSS for HTMX transitions -->
<style>
  .htmx-swapping {
    opacity: 0;
    transition: opacity 200ms ease-out;
  }
</style>
```

---

## 9. Responsive Breakpoints

```css
/* Tailwind defaults */
sm: 640px   /* Mobile landscape */
md: 768px   /* Tablet */
lg: 1024px  /* Desktop */
xl: 1280px  /* Large desktop */
2xl: 1536px /* Extra large */
```

### Mobile Adaptations

| Component | Desktop | Mobile |
|-----------|---------|--------|
| Nav | Horizontal links | Hamburger menu |
| Sidebar | Fixed left | Bottom sheet / Drawer |
| Cards grid | 3 columns | 1 column |
| Hero text | `text-6xl` | `text-4xl` |
| Tables | Full table | Card list |
| Modal | Centered | Full width, bottom |

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
/* Focus ring */
*:focus-visible {
  outline: 2px solid #a855f7;
  outline-offset: 2px;
}

/* Skip link */
.skip-link {
  position: absolute;
  top: -100px;
  left: 0;
  z-index: 100;
}
.skip-link:focus {
  top: 0;
}
```

---

## 11. File Structure

```
control-plane/
├── templates/
│   ├── layouts/
│   │   ├── base.templ           # HTML head, scripts
│   │   ├── landing.templ        # Landing page layout
│   │   ├── auth.templ           # Auth pages layout
│   │   └── dashboard.templ      # Dashboard layout
│   │
│   ├── pages/
│   │   ├── landing/
│   │   │   ├── index.templ      # Home/landing page
│   │   │   ├── features.templ
│   │   │   ├── pricing.templ
│   │   │   └── docs.templ
│   │   ├── auth/
│   │   │   ├── login.templ
│   │   │   ├── signup.templ
│   │   │   └── forgot.templ
│   │   └── dashboard/
│   │       ├── overview.templ
│   │       ├── keys.templ
│   │       ├── key_detail.templ
│   │       ├── audit.templ
│   │       ├── usage.templ
│   │       └── settings/
│   │           ├── profile.templ
│   │           ├── team.templ
│   │           ├── api_keys.templ
│   │           └── billing.templ
│   │
│   ├── partials/                # HTMX partial responses
│   │   ├── keys_list.templ
│   │   ├── activity_feed.templ
│   │   ├── sign_result.templ
│   │   └── toast.templ
│   │
│   └── components/
│       ├── button.templ
│       ├── card.templ
│       ├── input.templ
│       ├── modal.templ
│       ├── nav.templ
│       ├── sidebar.templ
│       ├── table.templ
│       ├── code_block.templ
│       └── chart.templ
│
├── static/
│   ├── css/
│   │   ├── input.css            # Tailwind input
│   │   └── output.css           # Compiled
│   ├── js/
│   │   └── app.js               # Alpine init, copy utils
│   └── img/
│       ├── logo.svg
│       ├── logo-dark.svg
│       └── og-image.png
│
└── tailwind.config.js
```

---

## 12. Implementation Checklist

### Phase 1: Foundation
- [ ] Set up templ + Tailwind
- [ ] Create base layout
- [ ] Implement color scheme CSS variables
- [ ] Add fonts (Outfit, Plus Jakarta Sans, JetBrains Mono)

### Phase 2: Landing Page
- [ ] Hero section with animation
- [ ] Problem section
- [ ] Solution section with code example
- [ ] How it works steps
- [ ] Features grid
- [ ] Pricing cards
- [ ] Testimonials
- [ ] Footer
- [ ] Mobile responsive

### Phase 3: Auth Pages
- [ ] Login page with OAuth buttons
- [ ] Signup page
- [ ] Password reset flow

### Phase 4: Dashboard
- [ ] Dashboard layout (sidebar + main)
- [ ] Overview page with stats
- [ ] Keys list with HTMX
- [ ] Key detail page
- [ ] Create key modal
- [ ] Sign test functionality

### Phase 5: Settings & Polish
- [ ] Settings pages
- [ ] Billing page
- [ ] Audit log
- [ ] Toast notifications
- [ ] Loading states
- [ ] Error states
- [ ] Mobile responsive dashboard

---

## 13. References

- [Tailwind CSS](https://tailwindcss.com/docs)
- [HTMX](https://htmx.org/docs/)
- [Alpine.js](https://alpinejs.dev/start-here)
- [templ](https://templ.guide/)
- [DaisyUI](https://daisyui.com/)
- [Chart.js](https://www.chartjs.org/docs/)

