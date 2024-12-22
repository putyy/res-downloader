/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'selector',
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
    'node_modules/naive-ui/**'
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}

