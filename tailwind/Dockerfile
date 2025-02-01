FROM node:alpine
WORKDIR /tailwind
RUN npm init -y && npm install tailwindcss @tailwindcss/cli

CMD npx @tailwindcss/cli --cwd ./app -i ./src/style.css -o /app/dst/style.css --watch
