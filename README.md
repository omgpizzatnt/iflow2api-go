# iflow2api-go

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A high-performance OpenAI-compatible API proxy for iFlow, written in Go.

## Features

- **OpenAI API Compatible**: Drop-in replacement for OpenAI's `/v1/chat/completions` endpoint
- **Anthropic API Compatible**: Full support for `/v1/messages` endpoint (Claude Code)
- **TLS Browser Fingerprinting**: Impersonates Chrome 124 for better API access
- **HMAC-SHA256 Signatures**: Automatic request signing for authentication
- **OAuth Authentication**: Support for OAuth login with automatic and manual modes
- **SSE Streaming**: Full support for streaming responses
- **Vision Support**: Image content detection and conversion
- **Reasoning Content Support**: Handles `reasoning_content` from thinking models (GLM-5, DeepSeek, etc.)
- **Cross-Platform**: Single binary, runs on Windows, Linux, macOS

## Quick Start

### Installation

```bash
# Build from source
git clone https://github.com/omgpizzatnt/iflow2api-go
cd iflow2api-go
go build -o iflow2api ./cmd/iflow2api
```

### Authentication

Option 1: OAuth Login
```bash
./iflow2api oauth                    # Automatic mode (browser callback)
./iflow2api oauth manual              # Manual mode (paste authorization code)
```

Option 2: Environment Variable
```bash
export IFlow_API_KEY=your_api_key_here
```

Option 3: Config File
Create `config.json` in the same directory as the binary:
```json
{
  "api_key": "your_api_key_here",
  "base_url": "https://apis.iflow.cn/v1"
}
```

### Running the Server

```bash
./iflow2api
```

Default: `http://0.0.0.0:28000`

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_HOST` | Server host | `0.0.0.0` |
| `SERVER_PORT` | Server port | `28000` |
| `API_KEY` | iFlow API key | - |
| `BASE_URL` | iFlow API base URL | `https://apis.iflow.cn/v1` |
| `PROXY_ENABLED` | Enable upstream proxy | `false` |
| `PROXY_URL` | Upstream proxy URL | - |
| `TLS_ENABLED` | Enable TLS impersonation | `true` |
| `TLS_BROWSER_PROFILE` | Browser profile | `chrome124` |
| `TLS_PLATFORM` | Platform (windows/mac/linux) | `windows` |

### Config File

The application looks for `config.json` in the binary directory:

```json
{
  "api_key": "sk-xxxxx",
  "base_url": "https://apis.iflow.cn/v1",
  "oauth_access_token": "...",
  "oauth_refresh_token": "...",
  "oauth_expires_at": "2026-03-12T12:00:00Z"
}
```

## API Endpoints

### OpenAI Compatible

#### Chat Completions (Non-Streaming)
```bash
curl -X POST http://localhost:28000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

#### Chat Completions (Streaming)
```bash
curl -X POST http://localhost:28000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

### Anthropic Compatible

#### Messages (Non-Streaming)
```bash
curl -X POST http://localhost:28000/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

#### Messages (Streaming)
```bash
curl -X POST http://localhost:28000/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

### Health Check
```bash
curl http://localhost:28000/health
```

### Model List
```bash
curl http://localhost:28000/v1/models
```

## Supported Models

- `glm-5` (Recommended)
- `glm-4.6`, `glm-4.7`
- `deepseek-v3.2-chat`
- `qwen3-coder-plus`
- `kimi-k2`, `kimi-k2.5`, `kimi-k2-thinking`
- `minimax-m2.5`
- `qwen-vl-max` (Vision)
- And more...

## Development

```bash
# Run tests
go test ./...

# Build for different platforms
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o iflow2api-linux ./cmd/iflow2api
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o iflow2api-mac ./cmd/iflow2api
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o iflow2api.exe ./cmd/iflow2api
```

## Project Structure

```
.
├── cmd/
│   └── iflow2api/
│       ├── main.go      # Server entry point
│       └── oauth.go     # OAuth login handler
├── internal/
│   ├── config/          # Configuration management
│   ├── handlers/        # HTTP handlers
│   ├── oauth/           # OAuth client and refresher
│   ├── proxy/           # Core proxy logic
│   ├── transport/       # TLS impersonation
│   └── vision/          # Vision support
├── go.mod
├── go.sum
└── README.md
```

## License

MIT

## Acknowledgments

- Original Python version: [cacaview/iflow2api](https://github.com/cacaview/iflow2api)
- TLS impersonation: [aarock1234/mimic](https://github.com/aarock1234/mimic)
