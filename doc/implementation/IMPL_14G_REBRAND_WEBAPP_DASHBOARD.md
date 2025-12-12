# Agent Task: Rebrand Web App - Dashboard & Auth

> **Parallel Execution:** ✅ Can run independently
> **Dependencies:** None
> **Estimated Time:** 1-2 hours

---

## Objective

Update dashboard, auth, and layout templates with POPSigner branding and **Bloomberg Terminal / HFT aesthetic**.

---

## Design Aesthetic

**CRITICAL:** Match the landing page terminal aesthetic.

### Visual Direction
- **True black background** (`#000000`, `bg-black`)
- **Bloomberg Orange/Amber accents** (`#f59e0b`, `amber-500/600`)
- **Cyan for data highlights** (`#06b6d4`)
- **No violet/purple** - that's crypto wallet, not infrastructure
- **Dark mode ONLY** - terminals are dark

### Dashboard Specific
- Data-dense layouts
- Monospace for keys, addresses, metrics
- Subtle borders (`border-neutral-800`)
- Orange accent for primary actions
- Green/Red for success/error states (trading terminal colors)

### Typography
- **IBM Plex Sans** for body text
- **IBM Plex Mono** for data, keys, addresses, metrics

---

## Scope

### Files to Modify

| File | Changes |
|------|---------|
| `control-plane/templates/layouts/base.templ` | Title, meta tags |
| `control-plane/templates/layouts/dashboard.templ` | Branding |
| `control-plane/templates/layouts/auth.templ` | Logo, branding |
| `control-plane/templates/layouts/landing.templ` | Title |
| `control-plane/templates/components/sidebar.templ` | Logo |
| `control-plane/templates/pages/dashboard.templ` | Branding |
| `control-plane/templates/pages/login.templ` | Branding, copy |
| `control-plane/templates/pages/signup.templ` | Branding, copy |
| `control-plane/templates/pages/onboarding.templ` | Branding, copy |
| `control-plane/templates/pages/forgot_password.templ` | Branding |
| `control-plane/templates/pages/keys_list.templ` | Add export visibility |
| `control-plane/templates/pages/keys_detail.templ` | Add export action |

---

## Implementation

### layouts/base.templ

```go
// Before
<title>BanhBaoRing - { pageTitle }</title>
<meta name="description" content="BanhBaoRing - Secure key management">

// After
<title>POPSigner - { pageTitle }</title>
<meta name="description" content="POPSigner - Point-of-Presence signing infrastructure">
<meta name="keywords" content="signing, infrastructure, celestia, cosmos, keys">
```

### layouts/auth.templ

```go
// Before
<div class="logo">
    <span class="text-3xl">🔔</span>
    <span>BanhBaoRing</span>
</div>

// After - Terminal aesthetic
<div class="min-h-screen bg-black flex items-center justify-center">
  <div class="max-w-md w-full">
    <div class="text-center mb-8">
      <span class="text-amber-500 text-2xl">◇</span>
      <span class="font-mono text-white text-xl ml-2">POPSigner</span>
    </div>
    { children... }
  </div>
</div>
```

### components/sidebar.templ

```go
// Before
<a href="/" class="logo">
    <span class="text-2xl">🔔</span>
    <span>BanhBaoRing</span>
</a>

// After - Terminal sidebar
<aside class="w-64 bg-neutral-950 border-r border-neutral-800 min-h-screen">
  <div class="p-4 border-b border-neutral-800">
    <a href="/" class="flex items-center gap-2">
      <span class="text-amber-500">◇</span>
      <span class="font-mono text-white font-semibold">POPSigner</span>
    </a>
  </div>
  
  // Nav items with terminal styling
  <nav class="p-4 space-y-1">
    <a href="/dashboard" class="flex items-center gap-2 px-3 py-2 text-neutral-400 hover:text-white hover:bg-neutral-900">
      <span class="font-mono text-sm">dashboard</span>
    </a>
    <a href="/keys" class="flex items-center gap-2 px-3 py-2 text-neutral-400 hover:text-white hover:bg-neutral-900">
      <span class="font-mono text-sm">keys</span>
    </a>
    <a href="/audit" class="flex items-center gap-2 px-3 py-2 text-neutral-400 hover:text-white hover:bg-neutral-900">
      <span class="font-mono text-sm">audit_log</span>
    </a>
    <a href="/settings" class="flex items-center gap-2 px-3 py-2 text-neutral-400 hover:text-white hover:bg-neutral-900">
      <span class="font-mono text-sm">settings</span>
    </a>
  </nav>
</aside>
```

### pages/login.templ

```go
// Before
<h1>Sign in to BanhBaoRing</h1>
<p>Secure key management for your rollup</p>

// After - Terminal login
<div class="bg-neutral-950 border border-neutral-800 p-8">
  <h1 class="font-mono text-xl text-white mb-2">
    <span class="text-amber-500">$</span> login
  </h1>
  <p class="text-neutral-500 text-sm mb-6">Point-of-Presence signing infrastructure</p>
  
  // Form with terminal styling
  <form class="space-y-4">
    <div>
      <label class="font-mono text-sm text-neutral-400">email</label>
      <input type="email" class="w-full bg-black border border-neutral-700 text-white p-2 mt-1 font-mono focus:border-amber-600">
    </div>
    <div>
      <label class="font-mono text-sm text-neutral-400">password</label>
      <input type="password" class="w-full bg-black border border-neutral-700 text-white p-2 mt-1 font-mono focus:border-amber-600">
    </div>
    <button type="submit" class="w-full bg-amber-600 text-black font-semibold py-2 hover:bg-amber-500">
      Sign In →
    </button>
  </form>
</div>
```

### pages/signup.templ

```go
// Before
<h1>Create your BanhBaoRing account</h1>
<p>Get started with secure key management</p>

// After - Terminal signup
<div class="bg-neutral-950 border border-neutral-800 p-8">
  <h1 class="font-mono text-xl text-white mb-2">
    <span class="text-amber-500">$</span> deploy
  </h1>
  <p class="text-neutral-500 text-sm mb-6">Create your POPSigner account</p>
  
  // Form with terminal styling
  <form class="space-y-4">
    <div>
      <label class="font-mono text-sm text-neutral-400">email</label>
      <input type="email" class="w-full bg-black border border-neutral-700 text-white p-2 mt-1 font-mono focus:border-amber-600">
    </div>
    <div>
      <label class="font-mono text-sm text-neutral-400">password</label>
      <input type="password" class="w-full bg-black border border-neutral-700 text-white p-2 mt-1 font-mono focus:border-amber-600">
    </div>
    <button type="submit" class="w-full bg-amber-600 text-black font-semibold py-2 hover:bg-amber-500">
      Create Account →
    </button>
  </form>
</div>
```

### pages/onboarding.templ

```go
// Before
<h1>Welcome to BanhBaoRing!</h1>
<p>Let's set up your first key</p>

// After
<h1>Welcome to POPSigner</h1>
<p>Let's configure your signing infrastructure</p>
```

### pages/keys_list.templ

Add export visibility:

```go
// Add "Exportable" badge to key cards
if key.Exportable {
    <span class="badge badge-success">Exportable</span>
}

// Add Export button to actions
<button hx-get={ "/keys/" + key.ID + "/export" }>
    Export Key
</button>
```

### pages/keys_detail.templ

Add export section:

```go
// Add to key details
<div class="detail-row">
    <span class="label">Exportable</span>
    <span class="value">
        if key.Exportable {
            ✓ Yes (exit guaranteed)
        } else {
            ✗ No
        }
    </span>
</div>

// Add export action
if key.Exportable {
    <div class="action-section">
        <h3>Export Key</h3>
        <p class="text-sm text-zinc-400">
            Download this key for use in local keyrings. 
            This is your exit guarantee.
        </p>
        <button hx-post={ "/keys/" + key.ID + "/export" }
                class="btn-secondary">
            Export Private Key
        </button>
    </div>
}
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

# Run locally
go run ./cmd/server

# Test pages:
# - /login
# - /signup
# - /dashboard
# - /keys
# - /keys/{id}

# Check for remaining references
grep -r "banhbao" ./templates/ --include="*.templ"
grep -r "BanhBao" ./templates/ --include="*.templ"
grep -r "🔔" ./templates/ --include="*.templ"
```

---

## Checklist

```
□ layouts/base.templ - title, meta tags
□ layouts/dashboard.templ - any branding
□ layouts/auth.templ - logo
□ layouts/landing.templ - title
□ components/sidebar.templ - logo
□ pages/dashboard.templ - branding
□ pages/login.templ - branding, copy
□ pages/signup.templ - branding, copy
□ pages/onboarding.templ - branding, copy
□ pages/forgot_password.templ - branding
□ pages/keys_list.templ - export visibility
□ pages/keys_detail.templ - export action
□ templ generate passes
□ go build passes
□ No remaining "banhbao", "BanhBao", or 🔔 references
```

---

## Output

After completion, the dashboard and auth pages reflect POPSigner branding with exit guarantee visibility.

