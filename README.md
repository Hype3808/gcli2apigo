# gcli2apigo

OpenAI-compatible API proxy for Google's Gemini models with OAuth credential management and Google APIs proxy support.

## Features

- **OpenAI-Compatible API**: Drop-in replacement for OpenAI API endpoints
- **Native Gemini API**: Direct access to Gemini models via native endpoints
- **Google APIs Proxy**: Proxy requests to any Google API service
- **OAuth Credential Management**: Web dashboard for managing multiple Google OAuth credentials
- **Automatic Credential Rotation**: Load balancing across multiple credentials
- **Rate Limiting**: Built-in rate limiting to avoid 429 errors
- **Usage Tracking**: Monitor API usage per credential
- **Credential Banning**: Temporarily disable problematic credentials
- **Multi-language Support**: English and Chinese UI
- **Docker Support**: Easy deployment with Docker and Docker Compose

## Models

### Basic Models

- gemini-2.5-pro
- gemini-2.5-pro-preview-03-25
- gemini-2.5-pro-preview-05-06
- gemini-2.5-pro-preview-06-05
- gemini-flash-latest
- gemini-2.5-flash
- gemini-2.5-flash-preview-05-20
- gemini-2.5-flash-preview-04-17
- gemini-2.5-flash-image-preview
- gemini-2.5-flash-image

### Fake Streaming Variations (EN)

- gemini-2.5-pro-fake
- gemini-2.5-pro-preview-03-25-fake
- gemini-2.5-pro-preview-05-06-fake
- gemini-2.5-pro-preview-06-05-fake
- gemini-flash-latest-fake
- gemini-2.5-flash-fake
- gemini-2.5-flash-preview-05-20-fake
- gemini-2.5-flash-preview-04-17-fake

### Fake Streaming Variations (ZH)

- 假流式/gemini-2.5-pro
- 假流式/gemini-2.5-pro-preview-03-25
- 假流式/gemini-2.5-pro-preview-05-06
- 假流式/gemini-2.5-pro-preview-06-05
- 假流式/gemini-flash-latest
- 假流式/gemini-2.5-flash
- 假流式/gemini-2.5-flash-preview-05-20
- 假流式/gemini-2.5-flash-preview-04-17

`-fake` and `假流式/` model variations indicate fake streaming models, make sure stream=True is enabled when using these models
To use `-fake` set DEFAULT_LANGUAGE to en in `.env` file
To use `假流式/` set DEFAULT_LANGUAGE to zh in `.env` file

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/Hype3808/gcli2apigo.git
cd gcli2apigo

# Start the service (uses default password: 123456)
docker compose up -d

# Access the dashboard
open http://localhost:7860
```

### Manual Installation

```bash
# Prerequisites: Go 1.24+
git clone https://github.com/Hype3808/gcli2apigo.git
cd gcli2apigo

# Install dependencies
go mod download

# Create .env file
cp .env.example .env
# Edit .env with your configuration

# Run the server
go run main.go
```

## Configuration

### Authentication

The service supports three authentication strategies:

**Option 1: Universal Password (Simplest)**
```bash
PASSWORD=your_password
```
Both dashboard and API use the same password.

**Option 2: Separate Passwords**
```bash
GEMINI_AUTH_PASSWORD=dashboard_password  # Dashboard login
GEMINI_API_KEY=api_key                   # API requests
```

**Option 3: Default (First-time Setup)**
If no authentication is configured, the service defaults to `PASSWORD=123456`.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PASSWORD` | Universal password for dashboard and API | `123456` (if empty) |
| `HOST` | Server bind address | `0.0.0.0` |
| `PORT` | Server port | `7860` |
| `OAUTH_CREDS_FOLDER` | OAuth credentials directory | `oauth_creds` |
| `DEFAULT_LANGUAGE` | UI language (zh/en) | `zh` |
| `CREDENTIAL_RATE_LIMIT_RPS` | Max requests per second per credential | `8` |
| `MAX_RETRY_ATTEMPTS` | Max retry attempts on 429 errors | `5` |
| `DEBUG_LOGGING` | Enable debug logging | `false` |

See [.env.example](.env.example) for all available options.

## Usage

### Dashboard

Access the web dashboard at `http://localhost:7860`:

1. **Login** with your configured password
2. **Add Credentials** via OAuth flow or JSON upload
3. **Monitor Usage** for each credential
4. **Ban/Unban** credentials as needed
5. **Configure Settings** like language and rate limits

### API Endpoints

#### OpenAI-Compatible

```bash
# Chat completions
curl -X POST http://localhost:7860/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_PASSWORD" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# List models
curl http://localhost:7860/v1/models \
  -H "Authorization: Bearer YOUR_PASSWORD"
```

#### Native Gemini API

```bash
# Generate content
curl -X POST http://localhost:7860/v1beta/models/gemini-2.0-flash-exp/generateContent \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_PASSWORD" \
  -d '{
    "contents": [{"parts": [{"text": "Hello!"}]}]
  }'

# Stream generate content
curl -X POST http://localhost:7860/v1beta/models/gemini-2.0-flash-exp/streamGenerateContent \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_PASSWORD" \
  -d '{
    "contents": [{"parts": [{"text": "Hello!"}]}]
  }'
```

#### Google APIs Proxy

```bash
# List Cloud Storage buckets
curl http://localhost:7860/googleapis/storage/v1/b \
  -H "Authorization: Bearer YOUR_PASSWORD"

# Any Google API endpoint
curl http://localhost:7860/googleapis/{api_path} \
  -H "Authorization: Bearer YOUR_PASSWORD"
```

### Health Check

```bash
curl http://localhost:7860/health
# Response: {"status":"healthy","service":"gcli2apigo"}
```

## Credential Management

### Adding Credentials

**Method 1: OAuth Flow (Recommended)**
1. Go to Dashboard → OAuth Setup
2. Click "Start OAuth Flow"
3. Follow Google's authorization prompts
4. Credentials are automatically saved

**Method 2: JSON Upload**
1. Download OAuth credentials from Google Cloud Console
2. Go to Dashboard → Upload Credentials
3. Select and upload the JSON file

### Credential Rotation

The service automatically rotates between available credentials:
- Load balancing across all active credentials
- Automatic retry with different credentials on failure
- Rate limiting per credential to avoid 429 errors

### Banning Credentials

Temporarily disable problematic credentials:
1. Go to Dashboard → Credentials
2. Click "Ban" on the credential
3. The credential will be excluded from rotation
4. Click "Unban" to re-enable

## Docker Deployment

### Docker Compose

```yaml
version: '3.8'
services:
  gcli2apigo:
    image: gcli2apigo
    ports:
      - "7860:7860"
    environment:
      - PASSWORD=your_password
      - DEFAULT_LANGUAGE=en
    volumes:
      - ./oauth_creds:/oauth_creds
    restart: unless-stopped
```

See [DOCKER.md](DOCKER.md) for detailed Docker deployment guide.

### Production Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for:
- Kubernetes deployment
- Nginx reverse proxy setup
- SSL/TLS configuration
- Security best practices
- Monitoring and logging

## Development

### Project Structure

```
gcli2apigo/
├── main.go                 # Application entry point
├── internal/
│   ├── auth/              # OAuth and credential management
│   ├── banlist/           # Credential banning logic
│   ├── client/            # GCP API clients
│   ├── config/            # Configuration management
│   ├── dashboard/         # Web dashboard handlers
│   ├── httputil/          # HTTP utilities
│   ├── i18n/              # Internationalization
│   ├── routes/            # API route handlers
│   ├── transformers/      # Request/response transformers
│   └── usage/             # Usage tracking
├── oauth_creds/           # OAuth credentials storage
├── .env.example           # Environment configuration template
├── Dockerfile             # Docker image definition
└── docker-compose.yml     # Docker Compose configuration
```

### Building from Source

```bash
# Build for current platform
go build -o gcli2apigo .

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o gcli2apigo-linux .

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o gcli2apigo.exe .
```

### Running Tests

```bash
go test ./...
```

## Troubleshooting

### Common Issues

**Port already in use**
```bash
# Change port in .env
PORT=8080
```

**Permission denied on oauth_creds**
```bash
chmod 700 oauth_creds
```

**429 Rate Limit Errors**
- Reduce `CREDENTIAL_RATE_LIMIT_RPS` in .env
- Add more OAuth credentials
- Enable debug logging to see which credentials are hitting limits

**Authentication Failed**
- Verify PASSWORD or GEMINI_AUTH_PASSWORD is set correctly
- Check .env file is loaded (look for log message on startup)
- Ensure Authorization header format: `Bearer YOUR_PASSWORD`

### Debug Logging

Enable detailed logging:
```bash
DEBUG_LOGGING=true
```

View logs:
```bash
# Docker Compose
docker-compose logs -f

# Manual
# Logs are printed to stdout
```

## Security

- **Change default password** immediately in production
- **Use HTTPS** with a reverse proxy (Nginx/Traefik)
- **Restrict network access** using firewalls
- **Secure credential files** with proper permissions (chmod 600)
- **Regular backups** of oauth_creds directory
- **Monitor usage** for suspicious activity

## License

See [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## Support

- **Issues**: [GitHub Issues](https://github.com/Hype3808/gcli2apigo/issues)
- **Logs**: Enable DEBUG_LOGGING for detailed troubleshooting

