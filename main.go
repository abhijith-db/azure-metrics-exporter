package main

import (
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/jessevdk/go-flags"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/webdevops/go-common/azuresdk/armclient"
	"github.com/webdevops/go-common/azuresdk/azidentity"
	"github.com/webdevops/go-common/azuresdk/prometheus/tracing"

	"github.com/webdevops/azure-metrics-exporter/config"
)

const (
	Author = "webdevops.io"

	UserAgent = "azure-metrics-exporter/"
)

var (
	argparser *flags.Parser
	Opts      config.Opts

	AzureClient             *armclient.ArmClient
	AzureResourceTagManager *armclient.ResourceTagManager

	prometheusCollectTime    *prometheus.SummaryVec
	prometheusMetricRequests *prometheus.CounterVec

	metricsCache *cache.Cache
	azureCache   *cache.Cache

	//go:embed templates/*.html
	templates embed.FS

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
)

func main() {
	initArgparser()
	initLogger()

	logger.Infof("starting azure-metrics-exporter v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), Author)
	logger.Info(string(Opts.GetJson()))
	initSystem()
	metricsCache = cache.New(1*time.Minute, 1*time.Minute)
	azureCache = cache.New(1*time.Minute, 1*time.Minute)

	logger.Infof("init Azure connection")
	initAzureConnection()
	initMetricCollector()

	// Initialize pprof if enabled
	if Opts.Server.PprofEnabled {
		go startPprofServer()
	}

	logger.Infof("starting http server on %s", Opts.Server.Bind)
	startHttpServer()
}

func initArgparser() {
	argparser = flags.NewParser(&Opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		var flagsErr *flags.Error
		if ok := errors.As(err, &flagsErr); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}
}

func initAzureConnection() {
	var err error

	if Opts.Azure.Environment != nil {
		if err := os.Setenv(azidentity.EnvAzureEnvironment, *Opts.Azure.Environment); err != nil {
			logger.Warnf(`unable to set envvar "%s": %v`, azidentity.EnvAzureEnvironment, err.Error())
		}
	}

	AzureClient, err = armclient.NewArmClientFromEnvironment(logger)
	if err != nil {
		logger.Fatal(err.Error())
	}
	AzureClient.SetUserAgent(UserAgent + gitTag)

	if err := AzureClient.Connect(); err != nil {
		logger.Fatal(err.Error())
	}

	AzureResourceTagManager, err = AzureClient.TagManager.ParseTagConfig(Opts.Azure.ResourceTags)
	if err != nil {
		logger.Fatalf(`unable to parse resourceTag configuration "%s": %v"`, Opts.Azure.ResourceTags, err.Error())
	}
}

// start and handle prometheus handler
func startHttpServer() {
	mux := http.NewServeMux()

	// Add pprof endpoints if enabled and using same bind address
	if Opts.Server.PprofEnabled && (Opts.Server.PprofBind == "" || Opts.Server.PprofBind == Opts.Server.Bind) {
		logger.Info("adding pprof endpoints to main server at /debug/pprof/")
		// Import of _ "net/http/pprof" automatically registers handlers with http.DefaultServeMux
		// We need to manually add them to our custom mux
		mux.HandleFunc("/debug/pprof/", func(w http.ResponseWriter, r *http.Request) {
			http.DefaultServeMux.ServeHTTP(w, r)
		})
	}

	// healthz
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err)
		}
	})

	// readyz
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err)
		}
	})

	mux.Handle(config.MetricsUrl, tracing.RegisterAzureMetricAutoClean(promhttp.Handler()))

	mux.HandleFunc(config.ProbeMetricsResourceUrl, probeMetricsResourceHandler)

	mux.HandleFunc(config.ProbeMetricsListUrl, probeMetricsListHandler)

	mux.HandleFunc(config.ProbeMetricsSubscriptionUrl, probeMetricsSubscriptionHandler)

	mux.HandleFunc(config.ProbeMetricsScrapeUrl, probeMetricsScrapeHandler)

	mux.HandleFunc(config.ProbeMetricsResourceGraphUrl, probeMetricsResourceGraphHandler)

	// report
	tmpl := template.Must(template.ParseFS(templates, "templates/*.html"))
	mux.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		cspNonce := base64.StdEncoding.EncodeToString([]byte(uuid.New().String()))

		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Referrer-Policy", "same-origin")
		w.Header().Add("X-Frame-Options", "DENY")
		w.Header().Add("X-XSS-Protection", "1; mode=block")
		w.Header().Add("X-Content-Type-Options", "nosniff")
		w.Header().Add("Content-Security-Policy",
			fmt.Sprintf(
				"default-src 'self'; script-src 'nonce-%[1]s'; style-src 'nonce-%[1]s'; img-src 'self' data:",
				cspNonce,
			),
		)

		templatePayload := struct {
			Nonce string
		}{
			Nonce: cspNonce,
		}

		if err := tmpl.ExecuteTemplate(w, "query.html", templatePayload); err != nil {
			logger.Error(err)
		}
	})

	srv := &http.Server{
		Addr:         Opts.Server.Bind,
		Handler:      mux,
		ReadTimeout:  Opts.Server.ReadTimeout,
		WriteTimeout: Opts.Server.WriteTimeout,
	}
	logger.Fatal(srv.ListenAndServe())
}

func initMetricCollector() {
	prometheusCollectTime = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "azurerm_stats_metric_collecttime",
			Help: "Azure Insights stats collecttime",
		},
		[]string{
			"subscriptionID",
			"handler",
			"filter",
		},
	)
	prometheus.MustRegister(prometheusCollectTime)

	prometheusMetricRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "azurerm_stats_metric_requests",
			Help: "Azure Insights resource requests",
		},
		[]string{
			"subscriptionID",
			"handler",
			"filter",
			"result",
		},
	)
	prometheus.MustRegister(prometheusMetricRequests)
}

// startPprofServer starts the pprof server
func startPprofServer() {
	var pprofBind string
	if Opts.Server.PprofBind != "" {
		pprofBind = Opts.Server.PprofBind
	} else {
		pprofBind = Opts.Server.Bind
	}

	// If pprof is using the same bind address as the main server,
	// the pprof endpoints will be added to the main server instead
	if pprofBind == Opts.Server.Bind {
		logger.Infof("pprof endpoints will be available on main server at %s", pprofBind)
		return
	}

	logger.Infof("starting pprof server on %s", pprofBind)

	pprofMux := http.NewServeMux()
	// The pprof endpoints are automatically registered when we import _ "net/http/pprof"
	// They will be available at /debug/pprof/
	pprofMux.Handle("/debug/pprof/", http.DefaultServeMux)

	pprofServer := &http.Server{
		Addr:         pprofBind,
		Handler:      pprofMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Errorf("pprof server failed: %v", err)
	}
}
