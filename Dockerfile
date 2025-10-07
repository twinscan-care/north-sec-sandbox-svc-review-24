# Build stage
FROM golang:alpine AS builder

# Set working directory
WORKDIR /app

# Install git (needed for go mod download)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY ./cmd ./cmd

# Build the application, including all .go files in the cmd directory
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o review-service ./cmd

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates jq bash

# Create app directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/review-service .
COPY characteristics.json /opt/characteristics.json

# Expose port
EXPOSE 8080

# Set environment variables
ENV GIN_MODE=release
ENV PORT=8080

# Run the application
CMD ["./review-service"]
