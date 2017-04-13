package main

import (
	"crypto/tls"
	fmt "github.com/jhunt/go-ansi"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jhunt/go-cli"
)

func timing(step string, f func()) {
	start := time.Now()
	f()
	end := time.Now()
	took := float64(end.UnixNano()-start.UnixNano()) / 1000000
	fmt.Fprintf(os.Stderr, "%s took %5.3f ms\n", step, took)
}

var Version string

type Opt struct {
	Help        bool `cli:"-h, --help"`
	Version     bool `cli:"-v, --version"`
	SkipVerify  bool `cli:-k, -N, --no-verify"`
	OnlyHeaders bool `cli:-H, --only-headers"`
}

func usage(out io.Writer) {
	fmt.Fprintf(out, "Usage: @G{gotcha} [-hHNv] @C{https://target.system} [local port]\n\n")
	fmt.Fprintf(out, "  -h, --help           Show this help screen\n")
	fmt.Fprintf(out, "  -v, --version        Print version information and exit\n")
	fmt.Fprintf(out, "  -H, --only-headers   Only dump HTTP request/response headers (skip the body).\n")
	fmt.Fprintf(out, "  -k, --no-verify      Do not verify TLS/SSL certificates.\n")
}

func main() {
	var opt Opt
	verifyStr := strings.ToLower(os.Getenv("SSL_SKIP_VERIFY"))
	if verifyStr != "" && verifyStr != "no" && verifyStr != "false" && verifyStr != "0" {
		opt.SkipVerify = true
	}

	_, args, err := cli.Parse(&opt)

	if opt.Version {
		if Version == "" {
			fmt.Printf("gotcha (development)\n")
		} else {
			fmt.Printf("gotcha v%s\n", Version)
		}
		os.Exit(0)
	}

	if opt.Help {
		usage(os.Stdout)
		return
	}

	if len(args) > 2 {
		usage(os.Stderr)
		os.Exit(1)
		return
	}

	backend := os.Getenv("GOTCHA_BACKEND")
	if len(args) >= 1 {
		backend = args[0]
	}

	if backend == "" {
		fmt.Fprintf(os.Stderr, "No backend host specified, and no $GOTCHA_BACKEND environment variable set\n\n"+
			"If you are deploying gotcha as a Cloud Foundry application, don't forget to `cf set-env"+
			" appname GOTCHA_BACKEND https://host/url'\n\n")
		os.Exit(1)
		return
	}

	target, err := url.Parse(backend)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse target '%s': %s\n", args[0], err)
		os.Exit(1)
		return
	}
	fmt.Fprintf(os.Stderr, "targeting %s\n", target)

	bind := ":3128"
	if os.Getenv("PORT") != "" {
		bind = ":" + os.Getenv("PORT")
	}
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

		b2b.ContentLength = req.ContentLength
		b2b.TransferEncoding = req.TransferEncoding

		if x, err := httputil.DumpRequestOut(b2b, !opt.OnlyHeaders); err == nil {
			fmt.Fprintf(os.Stderr, "%s\n", string(x))
		}

		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				for header, values := range via[0].Header {
					for _, value := range values {
						req.Header.Add(header, value)
					}
				}

				fmt.Printf("-- REDIRECT ------\n")
				if x, err := httputil.DumpRequestOut(req, !opt.OnlyHeaders); err == nil {
					fmt.Fprintf(os.Stderr, "%s\n", string(x))
				}
				return nil
			},
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: opt.SkipVerify,
				},
				Proxy: http.ProxyFromEnvironment,
			},
		}
		var res *http.Response
		timing("request", func() {
			res, err = client.Do(b2b)
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read response: %s\n", err)
			w.WriteHeader(599)
			return
		}

		fmt.Fprintf(os.Stderr, "\n\n\n")
		if x, err := httputil.DumpResponse(res, !opt.OnlyHeaders); err == nil {
			fmt.Fprintf(os.Stderr, "%s\n", string(x))
		}

		var b []byte
		timing("receive response", func() {
			b, err = ioutil.ReadAll(res.Body)
		})

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

		timing("send response", func() {
			w.WriteHeader(res.StatusCode)
			w.Write(b)
		})
	})

	http.ListenAndServe(bind, nil)
}
