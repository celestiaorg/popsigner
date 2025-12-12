/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./templates/**/*.templ",
    "./templates/**/*_templ.go",
    "./static/js/**/*.js",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // BanhBaoRing Design System - Dark Mode Primary
        'terminal': {
          // Backgrounds - Deep purple-black
          'bg': '#0c0a14',           // Deep purple-black - primary background
          'card': '#1a1625',         // Card backgrounds
          'elevated': '#2d2640',     // Elevated surfaces
          'border': '#4a3f5c',       // Subtle borders
          'border-highlight': '#6b5b8a', // Highlighted borders on hover

          // Text
          'text': '#faf5ff',         // Primary text (purple-tinted white)
          'muted': '#c4b5d6',        // Muted text
          'dim': '#8b7fa3',          // Very dim text

          // Accents - Purple + Orange
          'accent': '#f97316',       // Orange - primary CTA
          'accent-hover': '#fb923c', // Orange hover state
          'purple': '#a855f7',       // Celestia purple
          'teal': '#00F0D1',         // Electric Teal - success states
          'teal-dim': '#00C4AA',     // Teal for hover states
        },
        // Add explicit purple shades for gradients
        'bao': {
          'purple': {
            50: '#fdf4ff',
            100: '#fae8ff',
            200: '#f5d0fe',
            300: '#f0abfc',
            400: '#e879f9',
            500: '#d946ef',
            600: '#a855f7',
            700: '#7e22ce',
            800: '#6b21a8',
            900: '#581c87',
          },
          'orange': {
            400: '#fb923c',
            500: '#f97316',
            600: '#ea580c',
          },
        },
      },
      fontFamily: {
        // BanhBaoRing font stack
        display: ['Outfit', 'system-ui', 'sans-serif'],
        body: ['Plus Jakarta Sans', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
      },
      fontSize: {
        // Slightly larger base for better readability
        base: ['1rem', { lineHeight: '1.6' }],
      },
      borderRadius: {
        'xl': '0.875rem',
        '2xl': '1rem',
        '3xl': '1.5rem',
      },
      boxShadow: {
        'glow-purple': '0 0 20px -5px rgba(168, 85, 247, 0.4)',
        'glow-purple-lg': '0 0 40px -10px rgba(168, 85, 247, 0.5)',
        'glow-orange': '0 0 20px -5px rgba(249, 115, 22, 0.4)',
        'glow-orange-lg': '0 0 40px -10px rgba(249, 115, 22, 0.5)',
        'glow-teal': '0 0 20px -5px rgba(0, 240, 209, 0.3)',
        'glow-teal-sm': '0 0 8px rgba(0, 240, 209, 0.4)',
      },
      animation: {
        'fade-in': 'fadeIn 0.5s ease-out forwards',
        'slide-up': 'slideUp 0.5s ease-out forwards',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'pulse-soft': 'pulseSoft 3s ease-in-out infinite',
        'bounce': 'bounce 2s infinite',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(20px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(10px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' },
        },
        pulseSoft: {
          '0%, 100%': { opacity: '0.5' },
          '50%': { opacity: '1' },
        },
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-conic': 'conic-gradient(from 180deg at 50% 50%, var(--tw-gradient-stops))',
        'terminal-grid': 'linear-gradient(to right, rgba(74, 63, 92, 0.3) 1px, transparent 1px), linear-gradient(to bottom, rgba(74, 63, 92, 0.3) 1px, transparent 1px)',
        'gradient-radial-dark': 'radial-gradient(ellipse at center, rgba(168, 85, 247, 0.08) 0%, transparent 70%)',
      },
    },
  },
  plugins: [],
}
