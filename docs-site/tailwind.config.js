/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: ['./src/**/*.{js,jsx,md,mdx}'],
  theme: {
    extend: {
      colors: {
        ink: '#080b10',
        panel: '#10151d',
        line: '#202938',
        knot: { 300: '#9dffbd', 400: '#67efa0', 500: '#35d47d' },
      },
      boxShadow: { glow: '0 0 80px rgba(53,212,125,.12)' },
    },
  },
  plugins: [require('@tailwindcss/typography')],
}
