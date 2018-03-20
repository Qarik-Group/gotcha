package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	fmt "github.com/jhunt/go-ansi"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jhunt/go-cli"
)

func timing(step string, f func()) {
	start := time.Now()
	f()
	end := time.Now()
	took := float64(end.UnixNano()-start.UnixNano()) / 1000000
	fmt.Fprintf(os.Stderr, "@G{%5.3f ms} to %s\n", took, step)
}

var Version string

//GotchaCAPath is a constant containing the path to the .gotcha folder which contains the CA/certs/keys gotcha will use for TLS
var GotchaCAPath = os.Getenv("HOME") + "/.gotcha"

type Opt struct {
	Help        bool `cli:"-h, --help"`
	Version     bool `cli:"-v, --version"`
	SkipVerify  bool `cli:"-k, -N, --no-verify"`
	OnlyHeaders bool `cli:"-H, --only-headers"`
	Redirect    bool `cli:"-r, --redirect"`
	KeepReferer bool `cli:"--keep-referer"`
	TLS         bool `cli:"--tls"`
}

func usage(out io.Writer) {
	fmt.Fprintf(out, "Usage: @G{gotcha} [-hHNv] @C{https://target.system} [local port]\n\n")
	fmt.Fprintf(out, "  -h, --help           Show this help screen\n")
	fmt.Fprintf(out, "  -v, --version        Print version information and exit\n")
	fmt.Fprintf(out, "  -H, --only-headers   Only dump HTTP request/response headers (skip the body).\n")
	fmt.Fprintf(out, "  -k, --no-verify      Do not verify TLS/SSL certificates.\n")
	fmt.Fprintf(out, "  -r, --redirect       Rewrite and return 3xx redirects.\n")
	fmt.Fprintf(out, "      --keep-referer   Pass Referer: headers through, even with -r.\n")
	fmt.Fprintf(out, "      --tls            Present TLS (with a custom CA) to clients connecting\n")
	fmt.Fprintf(out, "                       to us.  The CA will be dumped to standard error.\n")
}

type Cert struct {
	RawCertificate *x509.Certificate
	RawKey         *rsa.PrivateKey

	Certificate string
	Key         string
}

func (ca *Cert) Sign(cert *Cert) error {
	raw, err := x509.CreateCertificate(rand.Reader, cert.RawCertificate, ca.RawCertificate, cert.RawKey.Public(), ca.RawKey)
	if err != nil {
		return err
	}

	cert.Certificate = string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: raw,
	}))

	return nil
}

func certificate(name string, serial int, ttl time.Duration) (*Cert, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	cert := &x509.Certificate{
		SignatureAlgorithm:    x509.SHA512WithRSA, /* FIXME: hard-coded */
		PublicKeyAlgorithm:    x509.RSA,
		Subject:               pkix.Name{CommonName: name},
		DNSNames:              []string{name},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
		IsCA:         true,
		MaxPathLen:   1,
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(ttl),
	}

	pKey := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))

	return &Cert{
		RawCertificate: cert,
		RawKey:         key,

		//	Certificate: pCert,
		Key: pKey,
	}, nil
}

func loadOrGenerateCA() (*Cert, error) {
	if _, err := os.Stat(GotchaCAPath); !os.IsNotExist(err) {
		fmt.Printf("Using cached CA certificate located at: %s/ca_cert.pem\n", GotchaCAPath)
		cert, err := ioutil.ReadFile(GotchaCAPath + "/ca_cert.pem")
		if err != nil {
			return nil, err
		}
		key, err := ioutil.ReadFile(GotchaCAPath + "/ca_key.pem")
		if err != nil {
			return nil, err
		}
		certBlock, _ := pem.Decode(cert)
		if certBlock == nil {
			return nil, errors.New("Failed to decode ca certificate")
		}
		keyBlock, _ := pem.Decode(key)
		if keyBlock == nil {
			return nil, errors.New("Failed to decode ca private key")
		}
		rawKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, err
		}
		rawCert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return nil, err
		}

		return &Cert{
			RawCertificate: rawCert,
			RawKey:         rawKey,

			Certificate: string(cert),
			Key:         string(key),
		}, nil
	}

	fmt.Printf("gotcha CA folder does not exist ... generating $HOME/.gotcha folder to use for TLS files\n")
	err := os.MkdirAll(GotchaCAPath, os.ModePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create gotcha ca folder at $HOME/.gotcha: %s\n", err)
	}

	ca, err := certificate("gotcha-ca", 1, 100*365*24*time.Hour)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate a CA certificate: %s\n", err)
		os.Exit(1)
	}

	if err := ca.Sign(ca); err != nil {
		fmt.Fprintf(os.Stderr, "failed to sign CA certificate: %s\n", err)
		os.Exit(1)
	}
	f, err := os.Create(GotchaCAPath + "/ca_cert.pem")
	if err != nil {
		return nil, err
	}
	f.WriteString(ca.Certificate)
	f.Close()

	f, err = os.Create(GotchaCAPath + "/ca_key.pem")
	if err != nil {
		return nil, err
	}
	f.WriteString(ca.Key)
	f.Close()
	return ca, nil
}
func setupTLS(server *http.Server) {
	ca, err := loadOrGenerateCA()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load or generate a CA: %s\n", err)
		os.Exit(1)
	}

	cert, err := certificate("gotcha", 2, 10*365*24*time.Hour)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate a certificate: %s\n", err)
		os.Exit(1)
	}

	if err := ca.Sign(cert); err != nil {
		fmt.Fprintf(os.Stderr, "failed to sign certificate: %s\n", err)
		os.Exit(1)
	}

	pair, err := tls.X509KeyPair([]byte(cert.Certificate), []byte(cert.Key))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse certificate: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("@G{CA Certificate:}\n%s\n\n", ca.Certificate)
	server.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{pair},
		NextProtos:   []string{"http/1.1"},
	}
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
	}

	if len(args) > 2 {
		usage(os.Stderr)
		os.Exit(1)
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
	}

	target, err := url.Parse(backend)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse target '%s': %s\n", args[0], err)
		os.Exit(1)
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
	if !opt.Redirect {
		fmt.Fprintf(os.Stderr, "redirects will be followed\n")
	} else {
		fmt.Fprintf(os.Stderr, "redirects will be returned\n")
	}

	/* cert! */

	server := &http.Server{
		Addr: bind,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		end, err := url.Parse(req.URL.String())
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse requested uri '%s': %s\n", req.URL, err)
			w.WriteHeader(599)
			return
		}
		wanted := end.Host
		end.Host = target.Host
		end.Scheme = target.Scheme
		b2b, err := http.NewRequest(req.Method, end.String(), req.Body)
		for header, values := range req.Header {
			if header == "Referer" && opt.Redirect && !opt.KeepReferer {
				continue
			}
			for _, value := range values {
				b2b.Header.Add(header, value)
			}
		}

		b2b.ContentLength = req.ContentLength
		b2b.TransferEncoding = req.TransferEncoding

		fmt.Fprintf(os.Stderr, "\n\n>>>  REQUEST  ===========================================\n")
		dumpRequest(os.Stderr, b2b, opt.OnlyHeaders)

		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if opt.Redirect {
					return http.ErrUseLastResponse
				}

				if len(via) > 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				for header, values := range via[0].Header {
					for _, value := range values {
						req.Header.Add(header, value)
					}
				}

				fmt.Fprintf(os.Stderr, "\n\n@@@  REDIRECT ===========================================\n")
				dumpRequest(os.Stderr, req, opt.OnlyHeaders)
				return nil
			},
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: opt.SkipVerify,
				},
				Proxy: http.ProxyFromEnvironment,
			},
		}
		fmt.Fprintf(os.Stderr, "\n")
		var res *http.Response
		timing("relay request", func() {
			res, err = client.Do(b2b)
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read response: %s\n", err)
			w.WriteHeader(599)
			return
		}

		fmt.Fprintf(os.Stderr, "\n\n<<<  RESPONSE  ==========================================\n")
		dumpResponse(os.Stderr, res, opt.OnlyHeaders)

		fmt.Fprintf(os.Stderr, "\n")
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
				if header == "Location" && opt.Redirect {
					u, err := url.Parse(value)
					if err == nil {
						u.Scheme = "https"
						u.Host = wanted
						value = u.String()
					}
				}
				w.Header().Add(header, value)
			}
		}

		timing("relay response", func() {
			w.WriteHeader(res.StatusCode)
			w.Write(b)
		})
	})
	if opt.TLS {
		setupTLS(server)
		server.ListenAndServeTLS("", "")
	} else {
		server.ListenAndServe()
	}
}

func swapBody(b io.ReadCloser, onlyh bool) (io.ReadCloser, io.ReadCloser, error) {
	if b == nil || onlyh {
		return b, nil, nil
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(b); err != nil {
		return nil, b, err
	}

	if err := b.Close(); err != nil {
		return nil, b, err
	}
	return ioutil.NopCloser(&buf), ioutil.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

func dumpHeader(out io.Writer, h http.Header) {
	headers := make([]string, len(h))
	i := 0
	for header := range h {
		headers[i] = header
		i++
	}

	sort.Strings(headers)
	for _, header := range headers {
		for _, value := range h[header] {
			fmt.Fprintf(out, "@B{%s}: @Y{%s}\n", header, value)
			if header == "Authorization" && strings.HasPrefix(value, "Basic ") {
				b, err := base64.StdEncoding.DecodeString(value[6:])
				if err != nil {
					fmt.Fprintf(out, "  @R{failed to decode: %s}\n", err)
				} else {
					userpass := strings.Split(string(b), ":")
					fmt.Fprintf(out, "  @C{username:} %s\n", userpass[0])
					fmt.Fprintf(out, "  @C{password:} %s\n", userpass[1])
				}
			}
		}
	}
	fmt.Fprintf(out, "\n")
}

func dumpResponse(out io.Writer, r *http.Response, onlyh bool) {
	save := r.Body

	fmt.Fprintf(out, "@G{%s %s}\n", r.Proto, r.Status)
	dumpHeader(out, r.Header)

	if !onlyh {
		save, r.Body, _ = swapBody(r.Body, onlyh)
		var b bytes.Buffer
		io.Copy(&b, save)
		fmt.Fprintf(out, "%s\n", b.String())
		return
	}
}

func dumpRequest(out io.Writer, r *http.Request, onlyh bool) {
	uri := r.RequestURI
	if uri == "" {
		uri = r.URL.RequestURI()
	}

	m := "GET"
	if r.Method != "" {
		m = r.Method
	}
	fmt.Fprintf(out, "@G{%s %s HTTP/%d.%d}\n", m, uri, r.ProtoMajor, r.ProtoMinor)

	if !(strings.HasPrefix(r.RequestURI, "http://") || strings.HasPrefix(r.RequestURI, "https://")) {
		host := r.Host
		if host == "" && r.URL != nil {
			host = r.URL.Host
		}
		if host != "" {
			fmt.Fprintf(out, "@M{Host}: @Y{%s}\n", host)
		}
	}

	if len(r.TransferEncoding) > 0 {
		fmt.Fprintf(out, "@M{Transfer-Encoding}: @Y{%s}\n", strings.Join(r.TransferEncoding, ","))
	}
	if r.Close {
		fmt.Fprintf(out, "@M{Connection}: @Y{close}\n")
	}
	dumpHeader(out, r.Header)

	if !onlyh && r.Body != nil {
		var save io.ReadCloser
		save, r.Body, _ = swapBody(r.Body, onlyh)

		b, _ := ioutil.ReadAll(r.Body)
		fmt.Fprintf(out, "%s\n", string(b))

		r.Body = save
	}
}
