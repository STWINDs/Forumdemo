# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct \
	GOSUMDB=sum.golang.google.cn

# Install build dependencies
RUN apk add --no-cache gcc musl-dev git ca-certificates && update-ca-certificates

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o forum-app ./cmd/main.go

# Stage 2: Final
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/forum-app .
# Copy config files
COPY --from=builder /app/config/config.yaml ./config/

# Export port 8080
EXPOSE 8080

# Command to run the executable
CMD ["./forum-app"]
