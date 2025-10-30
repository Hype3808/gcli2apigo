# Build stage
FROM golang:alpine AS builder

WORKDIR /build

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o gcli2apigo .

# Final stage - use scratch for minimal image
FROM scratch

# Copy CA certificates - REQUIRED for HTTPS requests to Google APIs
# (googleapis.com, oauth2.googleapis.com, cloudcode-pa.googleapis.com, etc.)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /build/gcli2apigo /gcli2apigo

# Copy oauth_creds folder with banlist.json and usage_stats.json
COPY --from=builder /build/oauth_creds /oauth_creds

# Expose port
EXPOSE 7860

# Set environment variables
# Note: If PASSWORD, GEMINI_AUTH_PASSWORD, and GEMINI_API_KEY are all empty,
# PASSWORD will default to "123456" on first start
ENV HOST=0.0.0.0 \
    PORT=7860 \
    DEFAULT_LANGUAGE=zh

# Run the application
ENTRYPOINT ["/gcli2apigo"]