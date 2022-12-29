package main

import (
	"flag"
	"github.com/abiosoft/lineprefix"
	"github.com/tv42/httpunix"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Proxy-Connection",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

type proxy struct {
	client *http.Client
}

func (p *proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	log.Println(req.RemoteAddr, " ", req.Method, " ", req.URL)

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		msg := "unsupported protocol scheme " + req.URL.Scheme
		http.Error(wr, msg, http.StatusBadRequest)
		log.Println(msg)
		return
	}

	//http: Request.RequestURI can't be set in client requests.
	//http://golang.org/src/pkg/net/http/client.go
	req.RequestURI = ""

	// Check if we are proxying a kubernetes service url
	if strings.HasSuffix(req.URL.Hostname(), ".svc.cluster.local") {
		parts := strings.Split(req.URL.Hostname(), ".")
		if len(parts) != 5 {
			msg := "Invalid kubernetes service url, too many parts"
			http.Error(wr, msg, http.StatusBadRequest)
			log.Println(msg)
			return
		}

		namespace := parts[1]
		serviceName := parts[0]
		servicePort := req.URL.Port()

		req.URL.Host = "kubeproxy" // References the unix socket
		req.URL.Scheme = "http+unix"
		req.URL.Path = "/api/v1/namespaces/" + namespace + "/services/" + serviceName + ":" + servicePort + "/proxy" + req.URL.Path
	}

	delHopHeaders(req.Header)

	resp, err := p.client.Do(req)
	if err != nil {
		http.Error(wr, "Server Error", http.StatusInternalServerError)
		log.Fatal("ServeHTTP:", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp.Body)

	log.Println(req.RemoteAddr, " ", resp.Status)

	delHopHeaders(resp.Header)

	copyHeader(wr.Header(), resp.Header)
	wr.WriteHeader(resp.StatusCode)
	if _, err = io.Copy(wr, resp.Body); err != nil {
		panic(err)
	}
}

func launchKubectlProxy(socketPath string) {
	cmd := exec.Command("kubectl", "proxy", "--unix-socket", socketPath, "--accept-hosts", ".*")

	cmd.Stdout = lineprefix.New(
		lineprefix.Writer(os.Stdout),
		lineprefix.Prefix("kubectl stdout:"),
	)
	cmd.Stderr = lineprefix.New(
		lineprefix.Writer(os.Stderr),
		lineprefix.Prefix("kubectl stderr:"),
	)

	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	} else {
		log.Fatal("'kubectl proxy' exited early, can not continue")
	}
}

func main() {
	var addr = flag.String("addr", "127.0.0.1:8080", "The addr of the application.")
	flag.Parse()

	tempDir, err := os.MkdirTemp("", "kube-http-proxy")
	if err != nil {
		panic("failed to create temp dir for kubectl-proxy's unix socket")
	}
	socketPath := path.Join(tempDir, "kubectl-proxy.sock")
	go launchKubectlProxy(socketPath)

	client := &http.Client{}
	u := &httpunix.Transport{
		DialTimeout:           100 * time.Millisecond,
		RequestTimeout:        1 * time.Second,
		ResponseHeaderTimeout: 1 * time.Second,
	}
	u.RegisterLocation("kubeproxy", socketPath)
	client.Transport = u

	handler := &proxy{
		client: client,
	}

	log.Println("Starting proxy server on", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
