// Command server implements certificate transparency APIs, using a trillian log
// as the backend storage.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/cloudflare/ct-log/b2"
	"github.com/cloudflare/ct-log/config"
	"github.com/cloudflare/ct-log/ct"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/google/certificate-transparency-go/trillian/ctfe"
	"github.com/google/trillian"
	"github.com/google/trillian/crypto/keys/der"
	"github.com/google/trillian/crypto/keyspb"
	"github.com/google/trillian/extension"
	"github.com/google/trillian/monitoring/prometheus"
	"github.com/google/trillian/server"
	"github.com/google/trillian/server/interceptor"
	"github.com/google/trillian/storage"
	"github.com/google/trillian/util"
	"golang.org/x/net/netutil"

	// Register PEMKeyFile, PrivateKey and PKCS11Config ProtoHandlers
	_ "github.com/google/trillian/crypto/keys/der/proto"
	_ "github.com/google/trillian/crypto/keys/pem/proto"
	_ "github.com/google/trillian/crypto/keys/pkcs11/proto"
)

var (
	configFile = flag.String("cfg", "", "Path to a YAML config file.")
)

//go:generate go run ./../admin/gen-server-client/main.go

func main() {
	flag.Parse()
	ctx := context.Background()

	cfg, err := config.FromFile(*configFile)
	if err != nil {
		glog.Exitf("failed to read config: %v", err)
	} else if cfg.MaxGetEntries > 0 {
		ctfe.MaxGetEntriesAllowed = cfg.MaxGetEntries
	}

	glog.CopyStandardLogTo("WARNING")
	glog.Info("**** CT HTTP Server Starting ****")

	// Connect to databases: Postgres, Kafka, HBase.
	psql, err := pdx.NewPostgres(cfg.PostgresDSN)
	if err != nil {
		glog.Exitf("failed to connect to postgres: %v", err)
	}
	defer psql.Close()

	consumer, err := pdx.NewKafkaConsumer(cfg.KafkaBrokers, cfg.KafkaTopics)
	if err != nil {
		glog.Exitf("failed to build kafka consumer: %v", err)
	}
	defer consumer.Close()

	producer, err := pdx.NewKafkaProducer(cfg.KafkaBrokers, cfg.KafkaTopics)
	if err != nil {
		glog.Exitf("failed to build kafka producer: %v", err)
	}
	defer producer.Close()

	watcher, err := pdx.NewKafkaWatcher(cfg.KafkaBrokers, cfg.KafkaTopics)
	if err != nil {
		glog.Exitf("failed to build kafka watcher: %v", err)
	}
	defer watcher.Close()

	index, err := pdx.NewHBaseIndex(cfg.HBaseQuorum, cfg.HBaseRoot, cfg.HBaseSeqTable, cfg.HBaseTreeTable)
	if err != nil {
		glog.Exitf("failed to build hbase index: %v", err)
	}
	defer index.Close()

	// Wrap our database connections in a struct that will implement
	// storage.LogStorage over them.
	logStorage := &ct.LogStorage{
		Postgres: psql,
		Consumer: consumer,
		Producer: producer,
		Index:    index,

		AdminStorage: cfg.AdminStorage,
	}

	// Initialize a quota manager and set it to watch the number of unsequenced
	// leaves in all of our logs.
	qm := ct.NewQuotaManager(cfg.MaxUnsequencedLeaves)
	for _, logConfig := range cfg.LogConfigs {
		if err := qm.WatchLog(psql, watcher, logConfig.LogId); err != nil {
			glog.Exitf("failed to initialize quota: %v", err)
		}
	}
	defer qm.Close()

	// Setup the log server.
	registry := extension.Registry{
		AdminStorage:  cfg.AdminStorage,
		LogStorage:    logStorage,
		QuotaManager:  qm,
		MetricFactory: prometheus.MetricFactory{},
		NewKeyProto: func(ctx context.Context, spec *keyspb.Specification) (proto.Message, error) {
			return der.NewProtoFromSpec(spec)
		},
	}
	ti := interceptor.New(registry.AdminStorage, registry.QuotaManager, false,
		registry.MetricFactory)
	logServer := trillianLogClient{
		interceptor.Combine(interceptor.ErrorWrapper, ti.UnaryInterceptor),
		server.NewTrillianLogRPCServer(registry, util.SystemTimeSource{}),
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

		opts := ctfe.InstanceOptions{
			Deadline:      cfg.RequestTimeout,
			MetricFactory: prometheus.MetricFactory{},
			RequestLog:    new(ctfe.DefaultRequestLog),
		}
		handlers, err := ctfe.SetUpInstance(ctx, logServer, logConfig, opts)
		if err != nil {
			glog.Exitf("failed to set up log #%v: %v", i, err)
		}

		for path, handler := range *handlers {
			mux.Handle(path, handler)
			if prefix := "/logs/microsoft"; strings.HasPrefix(path, prefix) {
				mux.Handle("/logs/actalis"+path[len(prefix):], handler)
				mux.Handle("/logs/primekey"+path[len(prefix):], handler)
			}
		}
		mux.Handle(logConfig.Prefix+"/ct/v1/report-dropped", &droppedHandler{
			logId:   logConfig.LogId,
			storage: logStorage,
		})
	}
	server := http.Server{Handler: cacheHandler{mux}}

	// Start listening for client requests.
	httpAddr := net.JoinHostPort(os.Getenv("HOST"), os.Getenv("PORT1"))
	httpList, err := net.Listen("tcp", httpAddr)
	if err != nil {
		glog.Exit(err)
	}
	httpList = netutil.LimitListener(httpList, cfg.MaxClients)

	// Start listening for connections from the prometheus server.
	metricsAddr := net.JoinHostPort(os.Getenv("HOST"), os.Getenv("PORT0"))
	metricsList, err := net.Listen("tcp", metricsAddr)
	if err != nil {
		glog.Exit(err)
	}

	// Spin off main threads of work.
	go metrics(consumer, qm, metricsList)
	go func() {
		glog.Exit(server.Serve(httpList))
	}()
	awaitSignal()
}

// awaitSignal waits for standard termination signals, then exits the process.
func awaitSignal() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigs
	glog.Warningf("Signal received: %v", sig)
	glog.Flush()
}
