gotcha - A Debugging MiTM HTTP Proxy
====================================

`gotcha` is a small HTTP/HTTPS man-in-the-middle proxy, which you
can use to troubleshoot HTTP traffic that occurs behind the veil
of SSL/TLS encryption.

Man-in-the-Middle?!
-------------------

Isn't that _dangerous_?

Yes, yes it is -- if it happens to you when you aren't expecting
it.  As a diagnostic method, it's pretty sweet.

It works like this:

```
+----------------+     +------------------------+     +----------+
|    CLIENT      | --> |        GOTCHA          | --> | UPSTREAM |
|                |     |                        |     +----------+
| connecting to  |     | binds 127.0.0.1:3128   |
| 127.0.0.1:3128 |     | connects to            |
| (plain HTTP)   |     | https://api.end.poi.nt |
+----------------+     +------------------------+
                                  |
                                  |
                                  V
                              +--------+
                              | stdout |
                              +--------+
```

So, instead of connecting directly to https://api.end.poi.nt, the
client connects to the `gotcha` process, which is listening on
some local port (https://127.0.0.1:3128 by default) and MitM-ing the
requests/responses to the upstream.

The diagnostics part comes in because `gotcha` dumps the entire HTTP
conversation to standard output, so you can see it in your
terminal.  The upstream is still talking HTTPS, but you can now
see headers, bodies and response codes!
