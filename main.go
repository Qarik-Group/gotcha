package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Fprintf(os.Stderr, "USAGE: gotcha https://target.system [bind]\n")
		os.Exit(1)
		return
	}

	if os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "-?" {
		fmt.Fprintf(os.Stderr, "USAGE: gotcha https://target.system [bind]\n")
		return
	}

	target, err := url.Parse(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse target '%s': %s\n", os.Args[1], err)
		os.Exit(1)
		return
	}
	fmt.Fprintf(os.Stderr, "targeting %s\n", target)

	bind := ":3128"
	if len(os.Args) == 3 {
		bind = os.Args[2]
		if strings.IndexRune(bind, ':') < 0 {
			bind = ":" + bind
		}
	}
	fmt.Fprintf(os.Stderr, "binding %s\n", bind)

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		end, err := url.Parse(req.URL.String())
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse requested uri '%s': %s\n", req.URL, err)
			w.WriteHeader(599)
			return
		}
		end.Host = target.Host
		end.Scheme = target.Scheme
		b2b, err := http.NewRequest(req.Method, end.String(), req.Body)
		for header, values := range req.Header {
			for _, value := range values {
				b2b.Header.Add(header, value)
			}
		}

		if x, err := httputil.DumpRequestOut(b2b, true); err == nil {
			fmt.Fprintf(os.Stderr, "%s\n", string(x))
		}

		res, err := http.DefaultClient.Do(b2b)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read response: %s\n", err)
			w.WriteHeader(599)
			return
		}
		if x, err := httputil.DumpResponse(res, true); err == nil {
			fmt.Fprintf(os.Stderr, "%s\n", string(x))
		}

		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read body: %s\n", err)
			w.WriteHeader(599)
			return
		}
		for header, values := range res.Header {
			for _, value := range values {
				w.Header().Add(header, value)
			}
		}
		w.WriteHeader(res.StatusCode)
		w.Write(b)
	})

	http.ListenAndServe(bind, nil)
}
