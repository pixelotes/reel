FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN apk add --update gcc git build-base

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o reel .

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /app/reel .
COPY --from=builder /app/web ./web
COPY --from=builder /app/data ./data
COPY --from=builder /app/config/config.example.yml ./config.example.yml

EXPOSE 8081
VOLUME ["/app/data", "/app/configs"]

CMD ["./reel", "-config", "/app/config/config.yml"]
