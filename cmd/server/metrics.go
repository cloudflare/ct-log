package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	buildInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "build_info",
			Help: "A metric with a constant '1' value labeled by version, and goversion.",
		},
		[]string{"version", "goversion"},
	)
	reqsByColo = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "reqs_by_colo",
			Help: "Incremented each time a request is received from a specific colo.",
		},
		[]string{"colo"},
	)
)

func metrics(metricsList net.Listener) {
	buildInfo.WithLabelValues(Version, GoVersion).Set(1)
	prometheus.MustRegister(buildInfo)
	prometheus.MustRegister(reqsByColo)
	// prometheus.MustRegister(qm.NumLeaves)
	// prometheus.MustRegister(qm.TreeSize)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			fmt.Fprintln(rw, "Hello, I'm a certificate transparency log's metrics and debugging server! Who are you?")
		} else {
			rw.WriteHeader(404)
			fmt.Fprintln(rw, "404 not found")
		}
	})
	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	mux.HandleFunc("/debug/version", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "Version: %s, GoVersion: %s", Version, GoVersion)
	})

	server := http.Server{Handler: mux}
	glog.Exit(server.Serve(metricsList))
}
