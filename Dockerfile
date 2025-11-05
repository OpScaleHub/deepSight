# Start from the official Golang image for building the app
FROM golang:1.21 AS builder

WORKDIR /app

# Copy go mod file and download dependencies. If a go.sum exists it will be created by `go mod download`/`go mod tidy`.
COPY go.mod ./
RUN go mod download || true

# Copy the source code
COPY . .

# Build the Go application. Disable CGO and force Linux target so the binary
# is statically linked and runnable in the lightweight final image.
ENV CGO_ENABLED=0
ENV GOOS=linux
RUN go build -ldflags="-s -w" -o main .

# Create a lightweight image for running
FROM alpine:latest

WORKDIR /root/

# Install certificates (if your app makes outbound TLS calls)
RUN apk add --no-cache ca-certificates

# Copy the app binary and static assets from the builder stage.
COPY --from=builder /app/main .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static

# Expose port
EXPOSE 8080

# Suggested OCI labels so registries (including GHCR) can link the image to the
# source repository and show metadata in the Packages UI. Update values if needed.
LABEL org.opencontainers.image.title="deepSight"
LABEL org.opencontainers.image.description="A tiny uptime & runtime dashboard"
LABEL org.opencontainers.image.url="https://github.com/OpScaleHub/deepSight"
LABEL org.opencontainers.image.source="https://github.com/OpScaleHub/deepSight"
LABEL org.opencontainers.image.licenses="MIT"

# Run the Go binary
CMD ["./main"]
