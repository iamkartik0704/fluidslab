FROM golang:alpine AS builder

# Set working directory
WORKDIR /app

# Copy modules manifests and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# CGO_ENABLED=0 ensures a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o dambreak ./cmd/dambreak

# Use a minimal alpine image for the final stage
FROM alpine:latest

# Add ca-certificates just in case
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/dambreak .

# Expose port 8080
EXPOSE 8080

# Command to run the executable
CMD ["./dambreak", "-mode", "serve", "-port", "8080", "-no-browser"]
