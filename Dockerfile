# --- Builder Stage ---
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN apk add --update gcc git build-base

# Copy all source code and assets into the builder
COPY . .

# Build a static binary
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o reel .


# --- Final Stage ---
FROM alpine:latest

# Arguments for user and group IDs, passed from docker-compose
ARG PUID=1000
ARG GUID=1000

# Install required packages
RUN apk --no-cache add ca-certificates tzdata

# Set the working directory
WORKDIR /app

# Create a non-root group and user with the specified IDs
RUN addgroup -g ${GUID} -S appgroup && \
    adduser -u ${PUID} -S appuser -G appgroup

# Copy the application binary from the builder stage
COPY --from=builder /app/reel .

# Copy web, data and config dirs from builder image
COPY --from=builder --chown=appuser:appgroup /app/web ./web
COPY --from=builder --chown=appuser:appgroup /app/data ./data
COPY --from=builder --chown=appuser:appgroup /app/config ./config

# Ensure the binary is executable and owned by the correct user
RUN chown appuser:appgroup /app/reel && chmod +x /app/reel

# Switch to the non-root user
USER appuser

# Expose the application port
EXPOSE 8081

CMD ["./reel", "-config", "/app/config/config.yml"]