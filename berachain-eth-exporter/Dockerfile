FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source from the current directory to the working Directory inside the container
COPY . .

# Build the Go app
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o berachain-eth-exporter ./cmd/main.go
# Start a new stage from scratch
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/berachain-eth-exporter .

# Command to run the executable
ENTRYPOINT ["./berachain-eth-exporter"]