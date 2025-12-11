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
        // Trading Terminal Palette
        'terminal': {
          // Backgrounds
          'bg': '#0C0C0C',           // Carbon Black - primary background
          'card': '#1B1B1F',         // Graphite Grey - card backgrounds
          'elevated': '#232328',     // Slightly elevated surfaces
          'border': '#2A2A30',       // Subtle borders
          'border-highlight': '#3A3A42', // Highlighted borders on hover

          // Text
          'text': '#FFFFFF',         // Primary white
          'muted': '#A0A0A5',        // Muted text
          'dim': '#6B6B70',          // Very dim text

          // Accents
          'accent': '#FF6A00',       // Laser Orange - primary CTA
          'accent-hover': '#FF8533', // Orange hover state
          'teal': '#00F0D1',         // Electric Teal - micro accent
          'teal-dim': '#00C4AA',     // Teal for hover states
        },
      },
      fontFamily: {
        // New font stack
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
        'glow-orange': '0 0 20px -5px rgba(255, 106, 0, 0.4)',
        'glow-orange-lg': '0 0 40px -10px rgba(255, 106, 0, 0.5)',
        'glow-teal': '0 0 20px -5px rgba(0, 240, 209, 0.3)',
        'glow-teal-sm': '0 0 8px rgba(0, 240, 209, 0.4)',
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'pulse-soft': 'pulseSoft 3s ease-in-out infinite',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
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
        'terminal-grid': 'linear-gradient(to right, rgba(42, 42, 48, 0.3) 1px, transparent 1px), linear-gradient(to bottom, rgba(42, 42, 48, 0.3) 1px, transparent 1px)',
        'gradient-radial-dark': 'radial-gradient(ellipse at center, rgba(255, 106, 0, 0.08) 0%, transparent 70%)',
      },
    },
  },
  plugins: [],
}
