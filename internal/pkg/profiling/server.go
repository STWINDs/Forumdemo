// Package profiling provides an opt-in, loopback-only diagnostics listener.
package profiling

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"time"
)

// Start returns nil when disabled. Access from a workstation uses an SSH tunnel.
func Start(address string) (*http.Server, error) {
	if address == "" {
		return nil, nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, fmt.Errorf("pprof requires a loopback IP address")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go server.Serve(listener)
	return server, nil
}
