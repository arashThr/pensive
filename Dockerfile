FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -v -o ./server ./cmd/server

FROM scratch
WORKDIR /app
COPY --from=build /app/server ./server
COPY ./assets/style.css /app/assets/style.css
COPY .env .env
CMD [ "./server" ]
