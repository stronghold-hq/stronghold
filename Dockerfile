# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o stronghold-api ./cmd/api/main.go

# Ensure models directory exists
RUN mkdir -p models

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/stronghold-api .

# Copy model files (directory will be empty if no models exist)
COPY --from=builder /app/models ./models

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./stronghold-api"]
