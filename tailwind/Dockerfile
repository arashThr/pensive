FROM node:alpine
WORKDIR /app/tailwind
RUN npm init -y && npm install tailwindcss @tailwindcss/cli

CMD npx @tailwindcss/cli --cwd ../templates -i ../src/style.css -o ../dst/style.css --watch
