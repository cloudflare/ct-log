// Command server implements certificate transparency APIs, using a trillian log
// as the backend storage.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/cloudflare/ct-log/config"
	"github.com/cloudflare/ct-log/ct"
	"github.com/cloudflare/ct-log/custom"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/google/certificate-transparency-go/trillian/ctfe"
	"github.com/google/trillian"
	"github.com/google/trillian/crypto/keys/der"
	"github.com/google/trillian/crypto/keyspb"
	"github.com/google/trillian/extension"
	"github.com/google/trillian/monitoring/prometheus"
	"github.com/google/trillian/quota"
	"github.com/google/trillian/server"
	"github.com/google/trillian/server/interceptor"
	"github.com/google/trillian/storage"
	"github.com/google/trillian/util"
	"github.com/google/trillian/util/election"
	"golang.org/x/net/netutil"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"

	// Register PEMKeyFile, PrivateKey and PKCS11Config ProtoHandlers
	_ "github.com/google/trillian/crypto/keys/der/proto"
	_ "github.com/google/trillian/crypto/keys/pem/proto"
	_ "github.com/google/trillian/crypto/keys/pkcs11/proto"
)

var (
	Version   = "dev"
	GoVersion = runtime.Version()

	configFile = flag.String("cfg", "", "Path to a YAML config file.")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "-cfg string")
		fmt.Fprintln(flag.CommandLine.Output(), "\tPath to a YAML config file.")
	}
}

//go:generate go run ./../admin/gen-server-client/main.go

func main() {
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())

	cfg, err := config.FromFile(*configFile)
	if err != nil {
		glog.Exitf("failed to read config: %v", err)
	}

	glog.CopyStandardLogTo("WARNING")
	glog.Info("**** CT HTTP Server Starting ****")

	// Connect to databases.
	local, err := custom.NewLocal(cfg.BoltPath)
	if err != nil {
		glog.Exitf("failed to open local database: %v", err)
	}
	remote, err := custom.NewRemote(cfg.B2AcctId, cfg.B2AppKey, cfg.B2Bucket, cfg.B2Url)
	if err != nil {
		glog.Exitf("failed to open remote database: %v", err)
	}

	// Wrap our database connections in a struct that will implement
	// storage.LogStorage over them.
	logStorage := &ct.LogStorage{
		Local:  local,
		Remote: remote,

		AdminStorage: cfg.AdminStorage,
	}

	// Initialize a quota manager and set it to watch the number of unsequenced
	// leaves in all of our logs.
	qm := ct.NewQuotaManager(cfg.MaxUnsequencedLeaves)
	for _, logConfig := range cfg.LogConfigs {
		qm.WatchLog(local, logConfig.LogId)
	}

	// Setup the log server.
	serverRegistry := extension.Registry{
		AdminStorage:  cfg.AdminStorage,
		LogStorage:    logStorage,
		QuotaManager:  qm,
		MetricFactory: prometheus.MetricFactory{Prefix: "server_"},
		NewKeyProto: func(ctx context.Context, spec *keyspb.Specification) (proto.Message, error) {
			return der.NewProtoFromSpec(spec)
		},
	}
	ti := interceptor.New(serverRegistry.AdminStorage, serverRegistry.QuotaManager, false, serverRegistry.MetricFactory)
	logServer := trillianLogClient{
		grpc_middleware.ChainUnaryServer(interceptor.ErrorWrapper, ti.UnaryInterceptor),
		server.NewTrillianLogRPCServer(serverRegistry, util.SystemTimeSource{}),
	}

	// Setup the web server's routes.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			fmt.Fprintln(rw, "Hello, I'm a certificate transparency log! Who are you?")
		} else {
			rw.WriteHeader(404)
			fmt.Fprintln(rw, "404 not found")
		}
	})
	for i, logConfig := range cfg.LogConfigs {
		_, err := logServer.GetLatestSignedLogRoot(ctx, &trillian.GetLatestSignedLogRootRequest{
			LogId: logConfig.LogId,
		})
		if err == storage.ErrTreeNeedsInit {
			_, err = logServer.InitLog(ctx, &trillian.InitLogRequest{
				LogId: logConfig.LogId,
			})
			if err != nil {
				glog.Exitf("failed to initialize log: %#v: %v", i, err)
			}
			glog.Infof("initialized log: %v", logConfig.LogId)
		} else if err != nil {
			glog.Exitf("failed to check if log is initialized: %#v: %v", i, err)
		}

		vcfg, err := ctfe.ValidateLogConfig(logConfig)
		if err != nil {
			glog.Exitf("failed to validate log config: %#v: %v", i, err)
		}
		opts := ctfe.InstanceOptions{
			Validated:     vcfg,
			Client:        logServer,
			Deadline:      cfg.RequestTimeout,
			MetricFactory: prometheus.MetricFactory{},
			RequestLog:    new(ctfe.DefaultRequestLog),
		}
		handlers, err := ctfe.SetUpInstance(ctx, opts)
		if err != nil {
			glog.Exitf("failed to set up log #%v: %v", i, err)
		}
		for path, handler := range *handlers {
			mux.Handle(path, handler)
		}
	}
	svc := http.Server{Handler: cacheHandler{mux}}

	// Setup the sequencing loop. This controls both sequencing and signing.
	signerRegistry := extension.Registry{
		AdminStorage:    cfg.AdminStorage,
		LogStorage:      logStorage,
		ElectionFactory: election.NoopFactory{InstanceID: "signer"},
		QuotaManager:    quota.Noop(),
		MetricFactory:   prometheus.MetricFactory{Prefix: "signer_"},
	}

	sequencerManager := server.NewSequencerManager(signerRegistry, cfg.Signer.GuardWindow)
	info := server.LogOperationInfo{
		Registry:    signerRegistry,
		BatchSize:   cfg.Signer.BatchSize,
		RunInterval: cfg.Signer.RunInterval,
		TimeSource:  util.SystemTimeSource{},
		ElectionConfig: election.RunnerConfig{
			PreElectionPause:    10 * time.Millisecond,
			MasterCheckInterval: time.Second,
			MasterHoldInterval:  (1 << 63) - 1,
			ResignOdds:          1,

			TimeSource: util.SystemTimeSource{},
		},
		NumWorkers: 1,
	}
	sequencerTask := server.NewLogOperationManager(info, sequencerManager)

	// Start listening for client requests.
	httpList, err := net.Listen("tcp", cfg.ServerAddr)
	if err != nil {
		glog.Exit(err)
	}
	httpList = netutil.LimitListener(httpList, cfg.MaxClients)

	// Start listening for connections from the prometheus server.
	metricsList, err := net.Listen("tcp", cfg.MetricsAddr)
	if err != nil {
		glog.Exit(err)
	}

	// Spin off main threads of work.
	go awaitSignal(cancel)
	go metrics(qm, metricsList)
	go func() {
		if cfg.CertFile == "" {
			glog.Exit(svc.Serve(httpList))
		} else {
			glog.Exit(svc.ServeTLS(httpList, cfg.CertFile, cfg.KeyFile))
		}
	}()
	sequencerTask.OperationLoop(ctx)

	// Give things a few seconds to tidy up
	glog.Infof("Stopping server, about to exit")
	glog.Flush()
	time.Sleep(1 * time.Second)
}

// awaitSignal waits for standard termination signals, then exits the process.
func awaitSignal(doneFn func()) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigs
	glog.Warningf("Signal received: %v", sig)
	glog.Flush()

	doneFn()
}

// cacheHandler sets the Cache-Control header on common, cache-able requests.
type cacheHandler struct {
	inner http.Handler
}

func (ch cacheHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if ray := req.Header.Get("Cf-Ray"); ray != "" {
		if i := strings.Index(ray, "-"); i != -1 {
			reqsByColo.WithLabelValues(ray[i+1:]).Inc()
		}
	}

	if req.Method == "GET" {
		if req.URL.Path == "/" {
			rw.Header().Set("Cache-Control", "public, max-age=14400")
		} else if strings.HasSuffix(req.URL.Path, "/ct/v1/get-sth") {
			rw.Header().Set("Cache-Control", "public, max-age=3600")
		} else if strings.HasSuffix(req.URL.Path, "/ct/v1/get-roots") {
			rw.Header().Set("Cache-Control", "public, max-age=14400")
		} else {
			rw.Header().Set("Cache-Control", "no-cache")
		}
	}

	ch.inner.ServeHTTP(rw, req)
}
