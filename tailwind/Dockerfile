FROM node:alpine
RUN wget https://github.com/tailwindlabs/tailwindcss/releases/download/v4.1.12/tailwindcss-linux-x64-musl -O /usr/local/bin/tailwindcss \
    && chmod +x /usr/local/bin/tailwindcss
 
CMD ["tailwindcss", "--cwd", "/templates", "-i", "/src/style.css", "-o", "/dst/style.css", "--watch"]
