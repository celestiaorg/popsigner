# Agent Task: Rebrand Web App - Landing Page

> **Parallel Execution:** ✅ Can run independently
> **Dependencies:** None
> **Estimated Time:** 2-3 hours

---

## Objective

Update all landing page templates with POPSigner branding, new copy, and **Bloomberg Terminal / HFT aesthetic**.

---

## Design Aesthetic

**IMPORTANT:** The design must reflect Point-of-Presence / HFT / Trading Terminal vibes.

### Visual Direction
- **Bloomberg Terminal aesthetic** - professional, data-dense, utilitarian
- **True black background** (`#000000`) - not gray, not dark gray, BLACK
- **Bloomberg Orange/Amber accents** (`#f59e0b`, `#d97706`) - signature color
- **Cyan for data highlights** (`#06b6d4`) - terminal feel
- **No violet/purple** - that's crypto wallet aesthetic, we're infrastructure
- **Dark mode ONLY** - no light theme

### Typography
- **IBM Plex Sans** for headings and body
- **IBM Plex Mono** for data, keys, code blocks
- Tabular numbers for any numeric data

### UI Elements
- Flat colors, no gradients
- Subtle borders (`#262626`)
- Orange hover states
- Monospace for technical content

---

## Scope

### Files to Modify

| File | Changes |
|------|---------|
| `control-plane/templates/components/landing/nav.templ` | Logo, brand name |
| `control-plane/templates/components/landing/hero.templ` | Headline, copy, CTAs |
| `control-plane/templates/components/landing/problems.templ` | Reframe or remove |
| `control-plane/templates/components/landing/solution.templ` | New positioning |
| `control-plane/templates/components/landing/how_it_works.templ` | Remove time claims |
| `control-plane/templates/components/landing/features.templ` | Update features |
| `control-plane/templates/components/landing/pricing.templ` | New tiers €49/€499/€19,999 |
| `control-plane/templates/components/landing/cta.templ` | New CTA |
| `control-plane/templates/components/landing/footer.templ` | Brand, links |

---

## Copy Reference

Refer to `doc/design/DESIGN_SYSTEM.md` for approved copy.

### Forbidden Words (NEVER use)

- low-latency, fast, faster, high-performance
- speed, throughput, milliseconds, ms
- zero hops, zero network hops
- "Ring ring!", bell emoji (🔔)

### Approved Replacements

| Instead of | Use |
|------------|-----|
| speed | proximity, inline, on the execution path |
| edge | Point-of-Presence, where systems already run |
| performance | deterministic, predictable, non-blocking |
| scale | parallel, worker-native, burst-ready |

---

## Implementation

### nav.templ

```go
// Before
<span class="text-2xl">🔔</span>
<span class="...">BanhBaoRing</span>

// After - Terminal aesthetic nav
<nav class="bg-black border-b border-neutral-900 sticky top-0 z-50">
  <div class="max-w-6xl mx-auto px-6 py-4 flex justify-between items-center">
    <a href="/" class="flex items-center gap-2">
      <span class="text-amber-500 text-xl">◇</span>
      <span class="font-mono text-white font-semibold">POPSigner</span>
    </a>
    
    <div class="flex items-center gap-8">
      <a href="/docs" class="text-neutral-400 hover:text-white text-sm">Docs</a>
      <a href="/pricing" class="text-neutral-400 hover:text-white text-sm">Pricing</a>
      <a href="https://github.com/..." class="text-neutral-400 hover:text-white text-sm">GitHub</a>
      <a href="/login" class="text-neutral-400 hover:text-white text-sm">Login</a>
      <a href="/signup" 
         class="bg-amber-600 text-black text-sm font-semibold px-4 py-2 hover:bg-amber-500">
        Deploy →
      </a>
    </div>
  </div>
</nav>
```

### hero.templ

```go
// Before
<div class="text-6xl mb-4 animate-bounce">🔔</div>
<h1>Ring ring!<br/>Sign where your infra lives.</h1>
<p>📍 Point of Presence key management for sovereign rollups.</p>
<p>Deploy next to your nodes. Same region. Same datacenter.</p>

// After - Bloomberg Terminal aesthetic
<section class="bg-black min-h-screen flex items-center">
  <div class="max-w-6xl mx-auto px-6">
    // Headline in monospace for terminal feel
    <h1 class="font-mono text-5xl md:text-6xl font-bold text-white tracking-tight">
      Point-of-Presence<br/>
      <span class="text-amber-500">Signing Infrastructure</span>
    </h1>
    
    // Subhead
    <p class="text-xl text-neutral-400 mt-6 max-w-2xl">
      A distributed signing layer designed to live inline with 
      execution—not behind an API queue.
    </p>
    
    // Secondary
    <p class="text-lg text-neutral-500 mt-4">
      Deploy next to your systems. Keys remain remote. You remain sovereign.
    </p>
    
    // CTAs - Orange primary
    <div class="mt-10 flex gap-4">
      <a href="/signup" 
         class="bg-amber-600 text-black font-semibold px-6 py-3 hover:bg-amber-500">
        Deploy POPSigner →
      </a>
      <a href="/docs" 
         class="border border-neutral-700 text-neutral-300 px-6 py-3 hover:border-amber-600">
        Documentation
      </a>
    </div>
  </div>
</section>
```

### pricing.templ

```go
// Before
PricingTier{Name: "Free", Price: "$0", ...}
PricingTier{Name: "Pro", Price: "$49", ...}
PricingTier{Name: "Enterprise", Price: "Custom", ...}

// After - Terminal aesthetic pricing grid
<section class="bg-black py-24">
  <div class="max-w-6xl mx-auto px-6">
    <h2 class="font-mono text-3xl text-white mb-4">
      <span class="text-amber-500">_</span>pricing
    </h2>
    <p class="text-neutral-500 mb-12">We sell placement, not transactions.</p>
    
    <div class="grid md:grid-cols-3 gap-6">
      // Tier 1 - Shared
      <div class="bg-neutral-950 border border-neutral-800 p-8">
        <h3 class="font-mono text-lg text-neutral-400 mb-4">shared</h3>
        <div class="font-mono text-4xl text-white mb-2">
          €49<span class="text-lg text-neutral-500">/mo</span>
        </div>
        <p class="text-neutral-500 text-sm mb-6">Shared POP infrastructure</p>
        <ul class="text-neutral-400 text-sm space-y-2 mb-8">
          <li>▸ Shared Point-of-Presence</li>
          <li>▸ No SLA</li>
          <li>▸ Plugins included</li>
          <li>▸ Exit guarantee</li>
        </ul>
        <a href="/signup?plan=shared" 
           class="block text-center border border-neutral-700 py-2 text-neutral-300 hover:border-amber-600">
          Start with Shared
        </a>
      </div>
      
      // Tier 2 - Priority (highlighted)
      <div class="bg-neutral-950 border-2 border-amber-600 p-8 relative">
        <div class="absolute -top-3 left-6 bg-amber-600 text-black text-xs font-mono px-2 py-1">
          RECOMMENDED
        </div>
        <h3 class="font-mono text-lg text-amber-500 mb-4">priority</h3>
        <div class="font-mono text-4xl text-white mb-2">
          €499<span class="text-lg text-neutral-500">/mo</span>
        </div>
        <p class="text-neutral-500 text-sm mb-6">Production workloads</p>
        <ul class="text-neutral-400 text-sm space-y-2 mb-8">
          <li>▸ Priority POP lanes</li>
          <li>▸ Region selection</li>
          <li>▸ 99.9% SLA</li>
          <li>▸ Self-serve scaling</li>
        </ul>
        <a href="/signup?plan=priority" 
           class="block text-center bg-amber-600 text-black py-2 font-semibold hover:bg-amber-500">
          Deploy Priority →
        </a>
      </div>
      
      // Tier 3 - Dedicated
      <div class="bg-neutral-950 border border-neutral-800 p-8">
        <h3 class="font-mono text-lg text-neutral-400 mb-4">dedicated</h3>
        <div class="font-mono text-4xl text-white mb-2">
          €19,999<span class="text-lg text-neutral-500">/mo</span>
        </div>
        <p class="text-neutral-500 text-sm mb-6">Dedicated infrastructure</p>
        <ul class="text-neutral-400 text-sm space-y-2 mb-8">
          <li>▸ Region-pinned POP</li>
          <li>▸ CPU isolation</li>
          <li>▸ 99.99% SLA</li>
          <li>▸ Manual onboarding</li>
        </ul>
        <a href="/contact" 
           class="block text-center border border-neutral-700 py-2 text-neutral-300 hover:border-amber-600">
          Contact Us
        </a>
      </div>
    </div>
  </div>
</section>
```

### features.templ

Update feature cards with terminal aesthetic:

```go
// Before
@FeatureCard("⚡", "Parallel Workers", "Create multiple signing workers...")
@FeatureCard("📊", "Real-time Analytics", "Monitor signing operations...")

// After - Terminal style cards, no emojis in production
// Use simple geometric icons or text prefixes
<section class="bg-black py-24">
  <div class="max-w-6xl mx-auto px-6">
    <h2 class="font-mono text-3xl text-white mb-12">
      <span class="text-amber-500">_</span>capabilities
    </h2>
    
    <div class="grid md:grid-cols-3 gap-6">
      // Card template - terminal style
      <div class="bg-neutral-950 border border-neutral-800 p-6 hover:border-amber-600">
        <h3 class="font-mono text-amber-500 text-lg mb-2">inline_signing</h3>
        <p class="text-neutral-400">On the execution path, not behind a queue.</p>
      </div>
      
      <div class="bg-neutral-950 border border-neutral-800 p-6 hover:border-amber-600">
        <h3 class="font-mono text-amber-500 text-lg mb-2">exit_guarantee</h3>
        <p class="text-neutral-400">Export keys anytime. Sovereignty by default.</p>
      </div>
      
      <div class="bg-neutral-950 border border-neutral-800 p-6 hover:border-amber-600">
        <h3 class="font-mono text-amber-500 text-lg mb-2">plugin_architecture</h3>
        <p class="text-neutral-400">secp256k1 built-in. Bring your own algorithms.</p>
      </div>
      
      <div class="bg-neutral-950 border border-neutral-800 p-6 hover:border-amber-600">
        <h3 class="font-mono text-amber-500 text-lg mb-2">audit_trail</h3>
        <p class="text-neutral-400">Every signature logged. Compliance ready.</p>
      </div>
      
      <div class="bg-neutral-950 border border-neutral-800 p-6 hover:border-amber-600">
        <h3 class="font-mono text-amber-500 text-lg mb-2">kubernetes_native</h3>
        <p class="text-neutral-400">Helm charts, CRDs, GitOps ready.</p>
      </div>
      
      <div class="bg-neutral-950 border border-neutral-800 p-6 hover:border-amber-600">
        <h3 class="font-mono text-amber-500 text-lg mb-2">open_source</h3>
        <p class="text-neutral-400">Apache 2.0. Self-host forever.</p>
      </div>
    </div>
  </div>
</section>
```

### cta.templ

```go
// Before
<div class="text-7xl mb-6 animate-bounce">🔔</div>
<h2>Ready to secure your keys?</h2>
<p>Sign up free. First signature in 5 minutes.</p>

// After - Terminal aesthetic
<section class="bg-neutral-950 py-24 border-t border-neutral-800">
  <div class="max-w-4xl mx-auto px-6 text-center">
    <h2 class="font-mono text-3xl text-white mb-6">
      <span class="text-amber-500">$</span> deploy signing infrastructure
    </h2>
    <p class="text-neutral-400 mb-10">
      Keys remote. Signing inline. You sovereign.
    </p>
    <div class="flex justify-center gap-4">
      <a href="/signup" 
         class="bg-amber-600 text-black font-semibold px-8 py-3 hover:bg-amber-500">
        Deploy POPSigner →
      </a>
      <a href="/docs" 
         class="border border-neutral-700 text-neutral-300 px-8 py-3 hover:border-amber-600">
        Documentation
      </a>
    </div>
  </div>
</section>
```

### footer.templ

```go
// Before
<span>🔔</span><span>BanhBaoRing</span>

// After - Terminal minimal
<footer class="bg-black border-t border-neutral-900 py-12">
  <div class="max-w-6xl mx-auto px-6">
    <div class="flex justify-between items-center">
      <div>
        <span class="font-mono text-lg text-white">
          <span class="text-amber-500">◇</span> POPSigner
        </span>
        <p class="text-sm text-neutral-600 mt-1">
          Point-of-Presence signing infrastructure
        </p>
      </div>
      <div class="flex gap-8 text-sm text-neutral-500">
        <a href="/docs" class="hover:text-amber-500">Docs</a>
        <a href="https://github.com/..." class="hover:text-amber-500">GitHub</a>
        <a href="/contact" class="hover:text-amber-500">Contact</a>
      </div>
    </div>
    <div class="mt-8 pt-8 border-t border-neutral-900 text-xs text-neutral-700">
      © 2025 POPSigner. Apache 2.0.
    </div>
  </div>
</footer>
```

---

## After Editing Templates

Regenerate the Go files:

```bash
cd control-plane
templ generate
```

---

## Verification

```bash
cd control-plane

# Generate templates
templ generate

# Build
go build ./...

# Run locally and check visually
go run ./cmd/server

# Open http://localhost:8080 and verify:
# - No bell emoji
# - No "Ring ring!"
# - No time-based claims
# - Correct pricing (€49/€499/€19,999)
# - POPSigner branding throughout

# Check for remaining references
grep -r "banhbao" ./templates/ --include="*.templ"
grep -r "Ring ring" ./templates/ --include="*.templ"
grep -r "🔔" ./templates/ --include="*.templ"
```

---

## Checklist

```
□ nav.templ - logo, brand name
□ hero.templ - headline, copy, CTAs (no time claims)
□ problems.templ - reframe or convert to "what_it_is"
□ solution.templ - new positioning
□ how_it_works.templ - remove time badges
□ features.templ - new feature list
□ pricing.templ - €49/€499/€19,999 tiers
□ cta.templ - new CTA (no bell)
□ footer.templ - brand, copyright
□ templ generate passes
□ go build passes
□ Visual verification - no forbidden elements
□ No remaining "banhbao", "Ring ring", or 🔔 references
```

---

## Output

After completion, the landing page reflects POPSigner branding with infrastructure-focused copy.

