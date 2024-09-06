const colors = require('tailwindcss/colors');

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./**/*.go', './static/input.css'],
  theme: {
    extend: {
      colors: {
        dark: colors.slate,
        primary: colors.pink,
      },
      accentColor: colors.pink,
    },
  },
  plugins: [
    require('@tailwindcss/typography'),
    require('@tailwindcss/forms'),
    require('@tailwindcss/aspect-ratio'),
    require('@tailwindcss/container-queries'),
  ],
};
