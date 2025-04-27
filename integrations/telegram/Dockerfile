FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -v -o ./bot ./integrations/telegram

FROM alpine
WORKDIR /app
COPY --from=build /app/bot ./bot
COPY .env .env
CMD [ "./bot" ]
