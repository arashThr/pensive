FROM node:24-slim AS tailwind-builder
WORKDIR /app
RUN npm init -y && npm install tailwindcss @tailwindcss/cli
COPY ./web/templates ./templates
COPY ./tailwind/style.css ./style.css
RUN npx @tailwindcss/cli -i ./style.css -o /style.css --minify

FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -v -o ./server ./cmd/server

FROM alpine
WORKDIR /app
RUN apk add --no-cache ffmpeg
COPY ./web/assets ./web/assets
COPY --from=build /app/server ./server
COPY --from=tailwind-builder /style.css ./web/assets/style.css
CMD [ "./server" ]
