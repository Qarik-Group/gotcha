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


A Real Example!
---------------

It's a bit of a gimme, but here's `gotcha` being used to intercept
HTTP traffic to www.google.com (over TLS) via curl:

```
$ /gotcha https://www.google.com &
targeting https://www.google.com
binding :3128

$ curl -X HEAD http://localhost:3128
HEAD / HTTP/1.1
Host: www.google.com
User-Agent: curl/7.43.0
Accept: */*


HTTP/1.1 200 OK
Transfer-Encoding: chunked
Accept-Ranges: none
Server: gws
...
```
