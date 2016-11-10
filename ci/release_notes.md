## Improvements

Custom headers are now re-instated if the backend redirects us.
Previously, the request was retried without the custom headers,
causing all sorts of havoc when troubleshooting services like
Vault.

## New Features

`gotcha` can now run as a CF app. It honors the `PORT` env var,
and makes use of `GOTCHA_BACKEND` and `SKIP_SSL_VERIFY` for
configuring the backend to proxy.

`gotcha` now sends HTTP requests honoring the `HTTP_PROXY`,
`HTTPS_PROXY`, `NO_PROXY` environment variables, and their
lower case equivalents.
