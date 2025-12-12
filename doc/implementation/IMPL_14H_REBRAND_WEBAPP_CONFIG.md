# Agent Task: Rebrand Web App - Config & Static Assets

> **Parallel Execution:** ✅ Can run independently
> **Dependencies:** None
> **Estimated Time:** 1 hour

---

## Objective

Update control-plane configuration files, static assets, and build files.

---

## Scope

### Files to Modify

| File | Changes |
|------|---------|
| `control-plane/config.yaml` | App name, URLs |
| `control-plane/config/config.example.yaml` | App name, URLs |
| `control-plane/go.mod` | Module description |
| `control-plane/Makefile` | Binary names |
| `control-plane/docker/Dockerfile` | Image labels |
| `control-plane/docker/docker-compose.yml` | Service names |
| `control-plane/tailwind.config.js` | Theme colors (optional) |
| `control-plane/static/css/input.css` | CSS variables |
| `control-plane/static/js/app.js` | Any branding refs |
| `control-plane/README.md` | Documentation |
| `control-plane/internal/config/config.go` | Default values |

---

## Implementation

### config.yaml

```yaml
# Before
app:
  name: BanhBaoRing
  url: https://banhbaoring.io

# After
app:
  name: POPSigner
  url: https://popsigner.io
  description: Point-of-Presence signing infrastructure
```

### config/config.example.yaml

```yaml
# POPSigner Control Plane Configuration
# Copy to config.yaml and update values

app:
  name: POPSigner
  url: https://popsigner.io
  
server:
  host: 0.0.0.0
  port: 8080

# ...
```

### Makefile

```makefile
# Before
.PHONY: build
build:
	go build -o banhbaoring-server ./cmd/server

# After
.PHONY: build
build:
	go build -o popsigner-server ./cmd/server

.PHONY: docker
docker:
	docker build -t popsigner-control-plane:dev .
```

### docker/Dockerfile

```dockerfile
# Before
LABEL org.opencontainers.image.title="BanhBaoRing Control Plane"

# After
LABEL org.opencontainers.image.title="POPSigner Control Plane"
LABEL org.opencontainers.image.description="POPSigner - Point-of-Presence signing infrastructure"
```

### docker/docker-compose.yml

```yaml
# Before
services:
  banhbaoring:
    image: banhbaoring-control-plane:dev
    container_name: banhbaoring

# After
services:
  popsigner:
    image: popsigner-control-plane:dev
    container_name: popsigner
```

### tailwind.config.js - Bloomberg Terminal Aesthetic

```javascript
// Bloomberg Terminal / HFT aesthetic
// Orange/amber on black, dark-only, data-dense
module.exports = {
  darkMode: 'class', // Always dark
  theme: {
    extend: {
      colors: {
        // PRIMARY: Bloomberg Orange/Amber
        primary: {
          50: '#fffbeb',
          100: '#fef3c7',
          200: '#fde68a',
          300: '#fcd34d',
          400: '#fbbf24',
          500: '#f59e0b',  // Main amber
          600: '#d97706',  // Bloomberg orange
          700: '#b45309',
          800: '#92400e',
          900: '#78350f',
        },
        // ACCENT: Terminal cyan for data highlights
        accent: {
          400: '#22d3ee',
          500: '#06b6d4',
          600: '#0891b2',
        },
        // Terminal backgrounds
        terminal: {
          black: '#000000',
          surface: '#0a0a0a',
          elevated: '#141414',
          hover: '#1f1f1f',
        },
        // Data colors
        data: {
          positive: '#22c55e',  // Green
          negative: '#ef4444',  // Red
          neutral: '#f59e0b',   // Orange
        },
      },
      fontFamily: {
        sans: ['IBM Plex Sans', 'Inter', 'system-ui', 'sans-serif'],
        mono: ['IBM Plex Mono', 'JetBrains Mono', 'SF Mono', 'monospace'],
      },
    },
  },
}
```

### static/css/input.css - Terminal Black + Bloomberg Orange

```css
/* POPSigner Terminal Aesthetic
   Bloomberg Terminal / HFT vibes
   Orange/amber on true black, dark-only */

@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  /* === PRIMARY: Bloomberg Orange/Amber === */
  --primary-400: #fbbf24;
  --primary-500: #f59e0b;
  --primary-600: #d97706;
  
  /* === ACCENT: Terminal Cyan === */
  --accent-400: #22d3ee;
  --accent-500: #06b6d4;
  
  /* === TERMINAL BACKGROUNDS === */
  --bg-primary: #000000;      /* True black */
  --bg-secondary: #0a0a0a;    /* Card/surface */
  --bg-tertiary: #141414;     /* Elevated */
  --bg-hover: #1f1f1f;        /* Hover */
  
  /* === TEXT === */
  --text-primary: #e5e5e5;
  --text-secondary: #a3a3a3;
  --text-tertiary: #737373;
  --text-accent: #f59e0b;     /* Orange highlight */
  
  /* === BORDERS === */
  --border: #262626;
  --border-hover: #404040;
  
  /* === DATA COLORS (trading terminal) === */
  --data-positive: #22c55e;   /* Green - up */
  --data-negative: #ef4444;   /* Red - down */
  --data-neutral: #f59e0b;    /* Orange - highlight */
}

/* No light theme. Terminal systems are dark. */

body {
  background-color: var(--bg-primary);
  color: var(--text-primary);
}

/* Monospace for data-heavy elements */
.font-data {
  font-family: 'IBM Plex Mono', 'JetBrains Mono', monospace;
  font-feature-settings: 'tnum' 1; /* Tabular numbers */
}

/* Orange accent links */
a {
  color: var(--text-accent);
}
a:hover {
  color: var(--primary-400);
}

/* Terminal-style cards */
.card {
  background-color: var(--bg-secondary);
  border: 1px solid var(--border);
}
.card:hover {
  border-color: var(--primary-600);
}

/* Primary button - Bloomberg orange */
.btn-primary {
  background-color: var(--primary-600);
  color: #000000;
}
.btn-primary:hover {
  background-color: var(--primary-500);
}
```

### internal/config/config.go

```go
// Before
const (
    DefaultAppName = "BanhBaoRing"
    DefaultAppURL  = "https://banhbaoring.io"
)

// After
const (
    DefaultAppName = "POPSigner"
    DefaultAppURL  = "https://popsigner.io"
)
```

### control-plane/README.md

```markdown
# POPSigner Control Plane

Point-of-Presence signing infrastructure - Control Plane API.

## Overview

The control plane provides the multi-tenant API for POPSigner,
including key management, signing operations, and billing.

...
```

---

## Static Assets

### Create New Logo

Create `control-plane/static/img/logo.svg`:

```svg
<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
  <!-- Simple geometric diamond/node shape -->
  <path d="M12 2L22 12L12 22L2 12L12 2Z" 
        stroke="currentColor" 
        stroke-width="2" 
        fill="none"/>
</svg>
```

Remove or replace bell-related assets if any exist.

---

## Verification

```bash
cd control-plane

# Build CSS
npx tailwindcss -i static/css/input.css -o static/css/output.css

# Build
go build ./...

# Run
go run ./cmd/server

# Check config loads correctly
# Check for remaining references
grep -r "banhbao" . --include="*.yaml" --include="*.yml" --include="*.go"
grep -r "BanhBao" . --include="*.yaml" --include="*.yml" --include="*.go"
```

---

## Checklist

```
□ config.yaml - app name, URLs
□ config/config.example.yaml - app name, URLs
□ go.mod - module description
□ Makefile - binary names
□ docker/Dockerfile - image labels
□ docker/docker-compose.yml - service names
□ tailwind.config.js - colors (optional)
□ static/css/input.css - CSS variables
□ static/js/app.js - any branding
□ internal/config/config.go - default values
□ README.md - documentation
□ Create new logo.svg (geometric, no emoji)
□ go build passes
□ CSS build passes
□ No remaining "banhbao" references
```

---

## Output

After completion, the control plane config and assets reflect POPSigner branding.

