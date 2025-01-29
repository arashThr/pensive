# Build Tailwind

Run: `npx @tailwindcss/cli -i style.css -o ../assets/style.css`
Note: In [setting up tailwind v4 using the standalone CLI](https://github.com/tailwindlabs/tailwindcss/discussions/15855) mentions:

> The biggest difference here from v3 is that there is no init step. In fact, if you try to use the init command you will get an error. You also won't need a tailwind.config.js file.
> if your website files do not have the same root directory as your Tailwind working directory, you will need to specify to Tailwind where to look.

That's why we add this: `@import "tailwindcss" source("../");`

Other option it to change the working directory:
`npx @tailwindcss/cli --cwd ../ -i ./tailwind/style.css -o ./assets/style.css --watch`
