## Improvements

Custom headers are now re-instated if the backend redirects us.
Previously, the request was retried without the custom headers,
causing all sorts of havoc when troubleshooting services like
Vault.

A new CLI argument handling library should make it easier to use
`gotcha` on days when you forget all of its options.  You can now
tack on a `-h` wherever you want to get help!

You can now optionally tell gotcha to (rewrite and) return 3xx
redirects to the client.  The rewrites allow gotcha to be used
with backends that may not set their `Location:` headers properly,
like the BOSH director.

Colorized output should make it easier to debug and reverse
engineer HTTP protocols, with minimal eye strain and cognitive
loss.  Plus, colors're pretty.

## New Features

`gotcha` can now run as a CF app. It honors the `PORT` env var,
and makes use of `GOTCHA_BACKEND` and `SKIP_SSL_VERIFY` for
configuring the backend to proxy.

`gotcha` now sends HTTP requests honoring the `HTTP_PROXY`,
`HTTPS_PROXY`, `NO_PROXY` environment variables, and their
lower case equivalents.
