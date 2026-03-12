# iflow2api-go

[English](README.md) | [简体中文](README.zh-CN.md)

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

用 Go 编写的高性能 iFlow OpenAI 兼容 API 代理。

## 特性

- **OpenAI API 兼容**: 可直接替换 OpenAI 的 `/v1/chat/completions` 端点
- **Anthropic API 兼容**: 完整支持 `/v1/messages` 端点 (Claude Code)
- **TLS 浏览器指纹**: 模拟 Chrome 124 以获得更好的 API 访问
- **HMAC-SHA256 签名**: 自动请求签名进行身份验证
- **OAuth 认证**: 支持自动和手动模式的 OAuth 登录
- **SSE 流式传输**: 完整支持流式响应
- **视觉支持**: 图像内容检测和转换
- **推理内容支持**: 处理思考模型的 `reasoning_content` (GLM-5, DeepSeek 等)
- **跨平台**: 单一二进制文件,支持 Windows、Linux、macOS

## 快速开始

### 安装

```bash
# 从源代码构建
git clone https://github.com/omgpizzatnt/iflow2api-go
cd iflow2api-go
go build -o iflow2api ./cmd/iflow2api
```

### 身份验证

方式 1: OAuth 登录
```bash
./iflow2api oauth                    # 自动模式 (浏览器回调)
./iflow2api oauth manual              # 手动模式 (粘贴授权码)
```

方式 2: 环境变量
```bash
export IFlow_API_KEY=your_api_key_here
```

方式 3: 配置文件
在与二进制文件相同的目录下创建 `config.json`:
```json
{
  "api_key": "your_api_key_here",
  "base_url": "https://apis.iflow.cn/v1"
}
```

### 运行服务器

```bash
./iflow2api
```

默认地址: `http://0.0.0.0:28000`

## 配置

### 环境变量

| 变量 | 说明 | 默认值 |
|----------|-------------|---------|
| `SERVER_HOST` | 服务器主机 | `0.0.0.0` |
| `SERVER_PORT` | 服务器端口 | `28000` |
| `API_KEY` | iFlow API 密钥 | - |
| `BASE_URL` | iFlow API 基础 URL | `https://apis.iflow.cn/v1` |
| `PROXY_ENABLED` | 启用上游代理 | `false` |
| `PROXY_URL` | 上游代理 URL | - |
| `TLS_ENABLED` | 启用 TLS 模拟 | `true` |
| `TLS_BROWSER_PROFILE` | 浏览器配置 | `chrome124` |
| `TLS_PLATFORM` | 平台 (windows/mac/linux) | `windows` |

### 配置文件

应用程序会在二进制文件所在目录下查找 `config.json`:

```json
{
  "api_key": "sk-xxxxx",
  "base_url": "https://apis.iflow.cn/v1",
  "oauth_access_token": "...",
  "oauth_refresh_token": "...",
  "oauth_expires_at": "2026-03-12T12:00:00Z"
}
```

## API 端点

### OpenAI 兼容

#### 聊天补全 (非流式)
```bash
curl -X POST http://localhost:28000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

#### 聊天补全 (流式)
```bash
curl -X POST http://localhost:28000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

### Anthropic 兼容

#### 消息 (非流式)
```bash
curl -X POST http://localhost:28000/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-5",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

#### 消息 (流式)
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

### 健康检查
```bash
curl http://localhost:28000/health
```

### 模型列表
```bash
curl http://localhost:28000/v1/models
```

## 支持的模型

- `glm-5` (推荐)
- `glm-4.6`, `glm-4.7`
- `deepseek-v3.2-chat`
- `qwen3-coder-plus`
- `kimi-k2`, `kimi-k2.5`, `kimi-k2-thinking`
- `minimax-m2.5`
- `qwen-vl-max` (视觉)
- 以及更多...

## 开发

```bash
# 运行测试
go test ./...

# 为不同平台构建
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o iflow2api-linux ./cmd/iflow2api
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o iflow2api-mac ./cmd/iflow2api
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o iflow2api.exe ./cmd/iflow2api
```

## 项目结构

```
.
├── cmd/
│   └── iflow2api/
│       ├── main.go      # 服务器入口
│       └── oauth.go     # OAuth 登录处理
├── internal/
│   ├── config/          # 配置管理
│   ├── handlers/        # HTTP 处理器
│   ├── oauth/           # OAuth 客户端和刷新器
│   ├── proxy/           # 核心代理逻辑
│   ├── transport/       # TLS 模拟
│   └── vision/          # 视觉支持
├── go.mod
├── go.sum
└── README.md
```

## 许可证

MIT

## 致谢

- 原始 Python 版本: [cacaview/iflow2api](https://github.com/cacaview/iflow2api)
- TLS 模拟: [aarock1234/mimic](https://github.com/aarock1234/mimic)
