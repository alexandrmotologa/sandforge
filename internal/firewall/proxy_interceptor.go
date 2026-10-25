package firewall

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// ProxyInterceptor runs an egress-filtering HTTP/HTTPS forward proxy.
type ProxyInterceptor struct {
	controller *Controller
	server     *http.Server
	listener   net.Listener
	addr       string
	mu         sync.Mutex
}

// NewProxyInterceptor creates a forward proxy attached to the firewall controller.
func NewProxyInterceptor(controller *Controller) *ProxyInterceptor {
	return &ProxyInterceptor{
		controller: controller,
	}
}

// Start binds to an available localhost port and begins serving proxy requests.
func (p *ProxyInterceptor) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("binding proxy listener: %w", err)
	}
	p.listener = l
	p.addr = fmt.Sprintf("http://%s", l.Addr().String())

	p.server = &http.Server{
		Handler:      http.HandlerFunc(p.handleRequest),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		_ = p.server.Serve(l)
	}()

	return nil
}

// Addr returns the proxy URL (e.g. http://127.0.0.1:54321).
func (p *ProxyInterceptor) Addr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.addr
}

// EnvVars returns environment variables that configure tools (curl, npm, pip, git) to route via this proxy.
func (p *ProxyInterceptor) EnvVars() []string {
	addr := p.Addr()
	if addr == "" {
		return nil
	}
	return []string{
		"HTTP_PROXY=" + addr,
		"HTTPS_PROXY=" + addr,
		"ALL_PROXY=" + addr,
		"http_proxy=" + addr,
		"https_proxy=" + addr,
		"all_proxy=" + addr,
	}
}

// Close terminates the proxy server.
func (p *ProxyInterceptor) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return p.server.Shutdown(ctx)
	}
	return nil
}

func (p *ProxyInterceptor) handleRequest(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodConnect {
		p.handleConnect(w, req)
		return
	}
	p.handleHTTP(w, req)
}

func (p *ProxyInterceptor) handleConnect(w http.ResponseWriter, req *http.Request) {
	host, portStr, err := net.SplitHostPort(req.Host)
	if err != nil {
		host = req.Host
		portStr = "443"
	}
	port, _ := strconv.Atoi(portStr)

	allowed, alert := p.controller.CheckAndRecord("proxy-connect", host, port, "https", req.RequestURI)
	if !allowed {
		w.Header().Set("X-SandForge-Firewall", "BLOCKED")
		http.Error(w, fmt.Sprintf("SandForge Egress Block: %s (Alert ID: %s)", alert.Reason, alert.ID), http.StatusForbidden)
		return
	}

	destConn, err := net.DialTimeout("tcp", req.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer destConn.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Tunnel bi-directional TCP stream
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(destConn, clientConn)
		if tc, ok := destConn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(clientConn, destConn)
		if tc, ok := clientConn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()

	wg.Wait()
}

func (p *ProxyInterceptor) handleHTTP(w http.ResponseWriter, req *http.Request) {
	host := req.URL.Hostname()
	if host == "" {
		host = req.Host
	}
	portStr := req.URL.Port()
	if portStr == "" {
		portStr = "80"
	}
	port, _ := strconv.Atoi(portStr)

	allowed, alert := p.controller.CheckAndRecord("proxy-http", host, port, "http", req.URL.String())
	if !allowed {
		w.Header().Set("X-SandForge-Firewall", "BLOCKED")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"error":"egress_blocked","reason":%q,"alert_id":%q}`, alert.Reason, alert.ID)))
		return
	}

	req.RequestURI = ""
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
