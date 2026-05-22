# weave-openai

OpenAI provider extension for [weave](https://github.com/weave-agent/weave) — an event-driven coding agent framework.

## Fork & Customize

1. Fork this repo
2. Edit the extension implementation
3. Install your fork: `weave install github.com/<you>/weave-openai --name openai`

The `--name openai` ensures your fork shadows the official extension.

## Install

```bash
weave install github.com/weave-agent/weave-openai --name openai
```

## Configuration

The OpenAI provider supports shared provider HTTP and retry settings from Weave config. Values under `providers.defaults` apply to all providers, and values under `providers.openai` override defaults for OpenAI only.

```json
{
  "providers": {
    "defaults": {
      "http": {
        "dial_timeout": "10s",
        "tls_handshake_timeout": "10s",
        "response_header_timeout": "60s",
        "idle_conn_timeout": "90s"
      },
      "retry": {
        "max_retries": 5,
        "base_delay": "1s",
        "max_delay": "30s",
        "multiplier": 2.0,
        "jitter": "full"
      }
    },
    "openai": {
      "model": "gpt-5.5",
      "base_url": "https://api.openai.com/v1",
      "http": {
        "response_header_timeout": "30s"
      },
      "retry": {
        "max_retries": 2,
        "jitter": "none"
      }
    }
  }
}
```

Duration fields use Go duration strings such as `5s`, `1m`, or `500ms`. `jitter` accepts `full` or `none`.

## Development

```bash
git clone git@github.com:weave-agent/weave-openai.git
cd weave-openai

# Add temporary replace for local SDK (don't commit this)
echo 'replace github.com/weave-agent/weave => /path/to/local/weave' >> go.mod

go test ./...
```

## License

Same as the main weave project.
