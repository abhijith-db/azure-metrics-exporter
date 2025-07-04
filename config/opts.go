package config

import (
	"encoding/json"
	"time"
)

type (
	Opts struct {
		// logger
		Logger struct {
			Debug       bool   `long:"log.debug"    env:"LOG_DEBUG"  description:"debug mode"`
			Development bool   `long:"log.devel"    env:"LOG_DEVEL"  description:"development mode"`
			Json        bool   `long:"log.json"     env:"LOG_JSON"   description:"Switch log output to json format"`
			Level       string `long:"log.level"    env:"LOG_LEVEL"  description:"Log level (debug, info, warn, error, dpanic, panic, fatal)" default:"info"`
		}

		// azure
		Azure struct {
			Environment      *string `long:"azure-environment"            env:"AZURE_ENVIRONMENT"                description:"Azure environment name" default:"AZUREPUBLICCLOUD"`
			AdResourceUrl    *string `long:"azure-ad-resource-url"        env:"AZURE_AD_RESOURCE"                description:"Specifies the AAD resource ID to use. If not set, it defaults to ResourceManagerEndpoint for operations with Azure Resource Manager"`
			ServiceDiscovery struct {
				CacheDuration *time.Duration `long:"azure.servicediscovery.cache"            env:"AZURE_SERVICEDISCOVERY_CACHE"                description:"Duration for caching Azure ServiceDiscovery of workspaces to reduce API calls (time.Duration)" default:"30m"`
			}
			ResourceTags []string `long:"azure.resource-tag"      env:"AZURE_RESOURCE_TAG"        env-delim:" "  description:"Azure Resource tags (space delimiter)"                              default:"owner"`
		}

		Metrics struct {
			Template   string `long:"metrics.template"               env:"METRIC_TEMPLATE"                            description:"Template for metric name"   default:"{name}"`
			Help       string `long:"metrics.help"                   env:"METRIC_HELP"                                description:"Metric help (with template support)"   default:"Azure monitor insight metric"`
			Dimensions struct {
				Lowercase bool `long:"metrics.dimensions.lowercase"   env:"METRIC_DIMENSIONS_LOWERCASE"             description:"Lowercase dimension values"`
			}
		}

		// Prober settings
		Prober struct {
			ConcurrencySubscription         int  `long:"concurrency.subscription"          env:"CONCURRENCY_SUBSCRIPTION"           description:"Concurrent subscription fetches"                                  default:"5"`
			ConcurrencySubscriptionResource int  `long:"concurrency.subscription.resource" env:"CONCURRENCY_SUBSCRIPTION_RESOURCE"  description:"Concurrent requests per resource (inside subscription requests)"  default:"10"`
			Cache                           bool `long:"enable-caching"                    env:"ENABLE_CACHING"                     description:"Enable internal caching"`
		}

		// general options
		Server struct {
			// general options
			Bind         string        `long:"server.bind"              env:"SERVER_BIND"           description:"Server address"        default:":8080"`
			ReadTimeout  time.Duration `long:"server.timeout.read"      env:"SERVER_TIMEOUT_READ"   description:"Server read timeout"   default:"5s"`
			WriteTimeout time.Duration `long:"server.timeout.write"     env:"SERVER_TIMEOUT_WRITE"  description:"Server write timeout"  default:"10s"`

			// pprof options
			PprofEnabled bool   `long:"server.pprof.enabled"     env:"SERVER_PPROF_ENABLED"  description:"Enable pprof endpoints"`
			PprofBind    string `long:"server.pprof.bind"        env:"SERVER_PPROF_BIND"     description:"Pprof server address (if different from main server)"`
		}
	}
)

func (o *Opts) GetJson() []byte {
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return jsonBytes
}
