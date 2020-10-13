package endpoint

import (
	"fmt"
	"time"

	"github.com/vocdoni/mtrouter/metrics"
	"github.com/vocdoni/mtrouter/net"
	"github.com/vocdoni/mtrouter/router"
	"github.com/vocdoni/mtrouter/types"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
)

// EndPoint handles the Websocket connection
type EndPoint struct {
	Router       *router.Router
	Proxy        *net.Proxy
	MetricsAgent *metrics.Agent
}

// NewHttpWsEndpoint creates a new websockets/http mixed endpoint
func NewHttpWsEndpoint(cfg *types.API, signer *ethereum.SignKeys, tf func() types.MessageAPI) (*EndPoint, error) {
	if cfg == nil {
		return nil, fmt.Errorf("cannot create endpoint, configuration is nil")
	}
	log.Infof("creating API service")

	// Create a HTTP Proxy service
	pxy, err := proxy(cfg.ListenHost, cfg.ListenPort, cfg.TLSdomain, cfg.TLSdirCert)
	if err != nil {
		return nil, err
	}

	// Create a HTTP+Websocket transport and attach the proxy
	ts := new(net.HttpWsHandler)
	ts.Init(new(types.Connection))
	ts.SetProxy(pxy)

	// Create the channel for incoming messages and attach to transport
	listenerOutput := make(chan types.Message)
	go ts.Listen(listenerOutput)
	transportMap := make(map[string]net.Transport)
	transportMap["httpws"] = ts

	// Create a new router and attach the transports
	r := router.NewRouter(listenerOutput, transportMap, signer, tf)

	// Attach the metrics agent (Prometheus)
	var ma *metrics.Agent
	if cfg.Metrics != nil && cfg.Metrics.Enabled {
		ma = metrics.NewAgent("/metrics", time.Second*time.Duration(cfg.Metrics.RefreshInterval), pxy)
	}
	return &EndPoint{Router: r, Proxy: pxy, MetricsAgent: ma}, nil
}

// proxy creates a new service for routing HTTP connections using go-chi server
// if tlsDomain is specified, it will use letsencrypt to fetch a valid TLS certificate
func proxy(host string, port int32, tlsDomain, tlsDir string) (*net.Proxy, error) {
	pxy := net.NewProxy()
	pxy.Conn.TLSdomain = tlsDomain
	pxy.Conn.TLScertDir = tlsDir
	pxy.Conn.Address = host
	pxy.Conn.Port = port
	log.Infof("creating proxy service, listening on %s:%d", host, port)
	if pxy.Conn.TLSdomain != "" {
		log.Infof("configuring proxy with TLS domain %s", tlsDomain)
	}
	return pxy, pxy.Init()
}
