# Start from the official Golang image for building the app
FROM golang:1.21 as builder

WORKDIR /app

# Copy go mod and sum files; download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the Go application (replace 'main.go' with your entry point if different)
RUN go build -o main .

# Create a lightweight image for running
FROM alpine:latest

WORKDIR /root/

# Copy the app binary from the builder stage.
COPY --from=builder /app/main .

# Expose port if needed (replace 8080 with your app's port)
EXPOSE 8080

# Run the Go binary
CMD ["./main"]
