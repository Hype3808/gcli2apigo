# gcli2apigo

OpenAI-compatible API proxy for Google's Gemini models with web dashboard management.

## Features

- **OpenAI-Compatible API** - Drop-in replacement for OpenAI API endpoints
- **Native Gemini API** - Direct access to Gemini API endpoints
- **Web Dashboard** - Manage OAuth credentials with a modern web interface
- **Multiple Authentication** - Support for API keys and OAuth2 flows
- **Usage Tracking** - Monitor API usage with daily limits and reset schedules
- **Error Tracking** - Display API error codes on credential cards
- **Ban Management** - Ban/unban credentials through the dashboard
- **Mobile Responsive** - Dashboard works on desktop, tablet, and mobile devices
- **Automatic Setup** - Creates required directories and files on first run

## Supported Models

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

## Quick Start

### Using Docker (Recommended)

```bash
# Clone the repository
git clone https://github.com/Hype3808/gcli2apigo
cd gcli2apigo

# Run with docker-compose
docker-compose up -d
```

### Manual Installation

```bash
# Build the application
go build -o gcli2apigo.exe .

# Set environment variables
export GEMINI_AUTH_PASSWORD="your-secure-password"
export HOST="0.0.0.0"
export PORT="7860"

# Run the server
./gcli2apigo.exe
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GEMINI_AUTH_PASSWORD` | `123456` | Authentication password for API access |
| `HOST` | `0.0.0.0` | Server bind address |
| `PORT` | `7860` | Server port |
| `OAUTH_CREDS_FOLDER` | `oauth_creds` | Directory for OAuth credential files |
| `DEBUG_LOGGING` | `false` | Enable verbose debug logging (set to `true` to enable) |

### Directory Structure

```
gcli2apigo/
├── oauth_creds/           # OAuth credentials directory
│   ├── banlist.json      # Banned credentials list
│   ├── usage_stats.json  # Usage tracking data
│   └── *.json           # Individual OAuth credential files
├── internal/             # Go source code
└── gcli2apigo.exe       # Compiled binary
```

## API Endpoints

### OpenAI-Compatible Endpoints

```bash
# Chat completions
POST /v1/chat/completions
Authorization: Bearer your-password

# List models
GET /v1/models
Authorization: Bearer your-password
```

### Native Gemini Endpoints

```bash
# Generate content
POST /v1beta/models/{model}/generateContent
Authorization: Bearer your-password

# Stream generate content
POST /v1beta/models/{model}/streamGenerateContent
Authorization: Bearer your-password

# List models
GET /v1beta/models
Authorization: Bearer your-password
```

### Dashboard Endpoints

- `GET /` - Dashboard login/main page
- `GET /dashboard/login` - Login page
- `POST /dashboard/login` - Login authentication
- `GET /dashboard/logout` - Logout
- `GET /dashboard/oauth/start` - Start OAuth flow
- `POST /dashboard/api/credentials/upload` - Upload credential files

## Usage Examples

### OpenAI-Compatible Request

```bash
curl -X POST http://localhost:7860/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-password" \
  -d '{
    "model": "gemini-2.5-pro",
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ]
  }'
```

### Native Gemini Request

```bash
curl -X POST http://localhost:7860/v1beta/models/gemini-2.5-pro/generateContent \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-password" \
  -d '{
    "contents": [
      {"parts": [{"text": "Hello, how are you?"}]}
    ]
  }'
```

## Web Dashboard

Access the web dashboard at `http://localhost:7860` to:

- View and manage OAuth credentials
- Monitor API usage and limits
- Ban/unban credentials
- Upload credential files (JSON or ZIP)
- View error codes from failed API requests
- Track daily usage statistics

### Dashboard Features

- **Credential Management** - Add, delete, ban/unban OAuth credentials
- **Usage Monitoring** - Real-time usage tracking with progress bars
- **Error Display** - Shows last API error code on credential cards
- **File Upload** - Drag-and-drop or browse to upload credential files
- **Mobile Responsive** - Works on all device sizes
- **Bulk Operations** - Select multiple credentials for batch operations

## Docker Deployment

### Using Docker Compose

```yaml
version: '3.8'
services:
  gcli2apigo:
    build: .
    ports:
      - "7860:7860"
    environment:
      - GEMINI_AUTH_PASSWORD=your-secure-password
      - HOST=0.0.0.0
      - PORT=7860
    volumes:
      - ./oauth_creds:/app/oauth_creds
    restart: unless-stopped
```

### Using Docker Run

```bash
docker build -t gcli2apigo .
docker run -d \
  -p 7860:7860 \
  -e GEMINI_AUTH_PASSWORD=your-secure-password \
  -v $(pwd)/oauth_creds:/app/oauth_creds \
  --name gcli2apigo \
  gcli2apigo
```

## Development

### Prerequisites

- Go 1.21 or later
- Git

### Building from Source

```bash
# Clone the repository
git clone https://github.com/Hype3808/gcli2apigo
cd gcli2apigo

# Install dependencies
go mod tidy

# Build the application
go build -o gcli2apigo.exe .

# Run tests
go test ./...
```

### Project Structure

```
gcli2apigo/
├── main.go                    # Application entry point
├── internal/
│   ├── auth/                 # Authentication and OAuth handling
│   ├── banlist/              # Credential banning functionality
│   ├── client/               # Gemini API client
│   ├── config/               # Configuration and models
│   ├── dashboard/            # Web dashboard handlers
│   ├── routes/               # API route handlers
│   ├── transformers/         # Request/response transformers
│   └── usage/                # Usage tracking
├── oauth_creds/              # OAuth credentials storage
├── Dockerfile                # Docker container definition
├── docker-compose.yml        # Docker Compose configuration
└── README.md                 # This file
```

## Security

- All credential files are stored with restricted permissions (0600)
- OAuth credentials directory is created with 0700 permissions
- Dashboard requires password authentication
- API endpoints require Bearer token authentication
- Input validation prevents path traversal attacks

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the Apache-2.0 License - see the [LICENSE](LICENSE) file for details.

## Support

For support, please open an issue on GitHub or contact the maintainers.