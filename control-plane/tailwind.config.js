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
        // BanhBaoRing warm, inviting palette inspired by Vietnamese bánh bao
        'bao': {
          // Core backgrounds
          bg: '#0d0b14',
          card: '#1a1625',
          border: '#3d3454',
          // Text colors
          text: '#faf5ff',
          muted: '#9b8fb8',
          // Accent - warm amber/gold
          accent: '#f59e0b',
        },
      },
      fontFamily: {
        // Crimson Pro - elegant serif for headings
        heading: ['Crimson Pro', 'Georgia', 'serif'],
        // Figtree - friendly, readable body text
        body: ['Figtree', 'system-ui', 'sans-serif'],
        // JetBrains Mono - excellent for code
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
        'glow': '0 0 20px -5px rgba(245, 158, 11, 0.3)',
        'glow-lg': '0 0 40px -10px rgba(245, 158, 11, 0.4)',
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
        'gradient-warm': 'linear-gradient(135deg, #f59e0b 0%, #ef4444 50%, #f59e0b 100%)',
      },
    },
  },
  plugins: [],
}

