package main

import (
	"fmt"
	"io/ioutil"
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/pborman/getopt"
)

func main() {
	getopt.SetParameters("https://target.system [local port]\n")
	help := getopt.BoolLong("help", 'h', "Show this help screen.")
	noverify := getopt.BoolLong("no-verify", 'N', "Do not verify TLS/SSL certificates.")

	var opts = getopt.CommandLine
	opts.Parse(os.Args)

	if *help {
		getopt.PrintUsage(os.Stdout)
		return
	}

	if opts.NArgs() != 1 && opts.NArgs() != 2 {
		getopt.PrintUsage(os.Stderr)
		os.Exit(1)
		return
	}

	args := opts.Args()
	target, err := url.Parse(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse target '%s': %s\n", args[0], err)
		os.Exit(1)
		return
	}
	fmt.Fprintf(os.Stderr, "targeting %s\n", target)

	bind := ":3128"
	if len(args) == 2 {
		bind = args[1]
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

		client := &http.Client{
			Transport:  &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: *noverify,
				},
			},
		}
		res, err := client.Do(b2b)
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
