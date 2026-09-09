/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'selector',
  // Naive UI supplies its own styles; scanning its bundles slows down extraction.
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}"
  ],
  theme: {
    extend: {
      colors: {
        app: {
          background: 'var(--app-background)',
          surface: 'var(--app-surface)',
          'surface-muted': 'var(--app-surface-muted)',
          'surface-hover': 'var(--app-surface-hover)',
          border: 'var(--app-border)',
          text: 'var(--app-text)',
          muted: 'var(--app-text-muted)',
          accent: 'var(--app-accent)',
          'accent-soft': 'var(--app-accent-soft)',
          danger: 'var(--app-danger)',
          'danger-soft': 'var(--app-danger-soft)',
          sidebar: 'var(--app-sidebar)',
          'sidebar-text': 'var(--app-sidebar-text)',
          'sidebar-accent': 'var(--app-sidebar-accent)',
        },
      },
      boxShadow: {
        'app-card': 'var(--app-card-shadow)',
      },
      keyframes: {
        'update-pulse': {
          '0%, 100%': {transform: 'scale(0.9)'},
          '50%': {transform: 'scale(1)'},
        },
      },
      animation: {
        'update-pulse': 'update-pulse 2s infinite',
      },
    },
  },
  plugins: [],
}
