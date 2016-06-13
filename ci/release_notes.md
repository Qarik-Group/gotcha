## Improvements

Custom headers are now re-instated if the backend redirects us.
Previously, the request was retried without the custom headers,
causing all sorts of havoc when troubleshooting services like
Vault.
