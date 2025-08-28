FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o reel ./cmd/reel

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /app/reel .
COPY --from=builder /app/web ./web
COPY --from=builder /app/configs/config.example.yml ./config.example.yml

EXPOSE 8081
VOLUME ["/app/data", "/app/config"]

CMD ["./reel", "-config", "/app/config/config.yml"]
