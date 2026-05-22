# CLAUDE.md

## Provider Runtime Configuration

Provider initialization should resolve HTTP and retry runtime through the shared SDK helpers:

- `providerhttp.ForProvider(cfg, "<provider>")`
- `providerretry.ForProvider(cfg, "<provider>")`

Store the configured `*http.Client` on provider state. For OpenAI-compatible providers using `utils/openaicompat.Stream`, store the resolved retry policy in `openaicompat.ProviderConfig.RetryConfig` during initialization.

Do not create production `&http.Client{}` instances directly in providers; this bypasses global and provider-specific transport config.
