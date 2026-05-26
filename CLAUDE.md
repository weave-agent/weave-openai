# CLAUDE.md

## Provider Runtime Configuration

Provider initialization should resolve HTTP and retry runtime through the shared SDK helpers:

- `providerhttp.ForProvider(cfg, "<provider>")`
- `providerretry.ForProvider(cfg, "<provider>")`

Store the configured `*http.Client` on provider state. For OpenAI-compatible providers using `utils/openaicompat.Stream`, store the resolved retry policy in `openaicompat.ProviderConfig.RetryConfig` during initialization.

Do not create production `&http.Client{}` instances directly in providers; this bypasses global and provider-specific transport config.

## Token Accounting

OpenAI streams Chat Completions usage through `utils/openaicompat.Stream`. Keep usage mapping in the shared OpenAI-compatible transport when possible, including cached prompt tokens from `prompt_tokens_details.cached_tokens`.

Do not expose `sdk.TokenCounter` from this extension until it has an exact Chat Completions-compatible count strategy or a provider-canonical tokenizer. Heuristic preflight estimates belong in the agent fallback path, not in a provider API that claims exact token counts.
