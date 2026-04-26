FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bot ./cmd/bot


FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/bot /bot

ENTRYPOINT ["/bot"]
