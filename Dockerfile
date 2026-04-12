# Stage 1: Build the bot
FROM golang:1.26.2-alpine AS builder

WORKDIR /app

# Install dependencies for building, certificates, and timezone data
RUN apk update && apk add --no-cache git tzdata ca-certificates

# Create a non-root user
RUN adduser -D -g '' botuser

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the executable
# Added -ldflags "-w -s" to strip debug info and reduce binary size further
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o vet-bot ./cmd/bot

# Stage 2: Create a minimal runtime image
FROM scratch

WORKDIR /app

# Copy SSL certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy user information
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy the pre-built binary
COPY --from=builder /app/vet-bot .

# Use the non-root user
USER botuser

EXPOSE 8080

# The bot will be run directly
CMD ["./vet-bot"]
