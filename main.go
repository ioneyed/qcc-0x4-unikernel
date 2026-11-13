package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort    = "8080"
	maxFileBytes   = 64 << 10
	maxOutputBytes = 64 << 10
)

var page = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Unikernel blast-radius demo</title>
  <style>
    :root { color-scheme: dark; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
    body { max-width: 58rem; margin: 3rem auto; padding: 0 1.25rem; background: #0b1020; color: #dce6ff; }
    h1 { color: #67e8f9; } h2 { margin-top: 2rem; }
    code, pre { background: #172036; border-radius: .35rem; }
    code { padding: .12rem .3rem; } pre { padding: 1rem; overflow: auto; }
    a { color: #fbbf24; } .warning { border-left: .3rem solid #fb7185; padding-left: 1rem; }
    .status { color: {{if .Enabled}}#86efac{{else}}#fca5a5{{end}}; }
  </style>
</head>
<body>
  <h1>Same vulnerable app, smaller blast radius</h1>
  <p class="warning"><strong>Deliberately vulnerable.</strong> Use only on a local demo machine. Unsafe endpoints are <span class="status">{{if .Enabled}}enabled{{else}}disabled{{end}}</span>.</p>
  <p>This Go application exposes two intentionally bad endpoints. Run it natively, then boot it with Ops/Nanos and compare what an attacker can reach.</p>

  <h2>1. Arbitrary file read</h2>
  <p><a href="/read?path=/etc/passwd">Read <code>/etc/passwd</code></a></p>
  <pre>curl --get --data-urlencode 'path=/etc/passwd' http://localhost:8080/read</pre>

  <h2>2. Unauthenticated command execution</h2>
  <p><a href="/exec?cmd=id">Run <code>id</code></a> · <a href="/exec?cmd=uname%20-a">Run <code>uname -a</code></a></p>
  <pre>curl --get --data-urlencode 'cmd=id' http://localhost:8080/exec</pre>

  <h2>What to observe</h2>
  <p>On a normal host, the application can see the host filesystem and invoke a shell. In the unikernel, the host account database is replaced by a tiny guest file (typically one <code>root</code> entry using <code>/bin/nobash</code>), and process creation fails. The HTTP bugs still exist; their reachable environment is smaller.</p>
  <p><small>{{.GOOS}}/{{.GOARCH}} · pid {{.PID}} · <a href="/healthz">health</a></small></p>
</body>
</html>`))

type pageData struct {
	Enabled bool
	GOOS    string
	GOARCH  string
	PID     int
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	unsafeEnabled := os.Getenv("DEMO_UNSAFE") == "1"
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           newHandler(unsafeEnabled),
		ReadHeaderTimeout: 3 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	log.Printf("listening on http://0.0.0.0:%s (unsafe demo endpoints: %t)", port, unsafeEnabled)
	log.Fatal(server.ListenAndServe())
}

func newHandler(unsafeEnabled bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.Execute(w, pageData{unsafeEnabled, runtime.GOOS, runtime.GOARCH, os.Getpid()}); err != nil {
			log.Printf("render page: %v", err)
		}
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("/read", unsafeOnly(unsafeEnabled, readFile))
	mux.HandleFunc("/exec", unsafeOnly(unsafeEnabled, runCommand))
	return logRequests(mux)
}

func unsafeOnly(enabled bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			http.Error(w, "demo endpoint disabled; set DEMO_UNSAFE=1", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// readFile deliberately performs no path validation. This is the vulnerability.
func readFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "missing path query parameter", http.StatusBadRequest)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("open %q: %v", path, err), http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.Copy(w, io.LimitReader(f, maxFileBytes))
}

// runCommand deliberately sends unauthenticated input to a shell. This is the vulnerability.
func runCommand(w http.ResponseWriter, r *http.Request) {
	command := r.URL.Query().Get("cmd")
	if command == "" {
		http.Error(w, "missing cmd query parameter", http.StatusBadRequest)
		return
	}
	if len(command) > 256 {
		http.Error(w, "command is too long", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if len(output) > maxOutputBytes {
		output = output[:maxOutputBytes]
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "exec %q: %v\n", command, err)
	}
	_, _ = w.Write(output)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s from %s in %s", r.Method, r.URL.RequestURI(), stripPort(r.RemoteAddr), time.Since(started).Round(time.Millisecond))
	})
}

func stripPort(address string) string {
	parts := strings.Split(address, ":")
	if len(parts) < 2 {
		return address
	}
	if _, err := strconv.Atoi(parts[len(parts)-1]); err != nil {
		return address
	}
	return strings.Join(parts[:len(parts)-1], ":")
}
