FROM node:latest
WORKDIR /app/tailwind
RUN npm init -y && npm install tailwindcss @tailwindcss/cli

RUN ls /app
CMD npx @tailwindcss/cli -i ../src/style.css -o ../dst/style.css --watch
