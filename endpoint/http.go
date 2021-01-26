package endpoint

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/vocdoni/multirpc/metrics"
	"github.com/vocdoni/multirpc/transports"
	"github.com/vocdoni/multirpc/transports/mhttp"
	"go.vocdoni.io/dvote/log"
)

// HTTPEndPoint handles the HTTP connection
type HTTPEndPoint struct {
	Proxy        *mhttp.Proxy
	MetricsAgent *metrics.Agent
	transport    transports.Transport
	id           string
	config       HTTPapi
}

// ID returns the name of the transport implemented on the endpoint
func (e *HTTPEndPoint) ID() string {
	return e.id
}

// Transport returns the transport used for this endpoint
func (e *HTTPEndPoint) Transport() transports.Transport {
	return e.transport
}

// SetOption configures a endpoint option, valid options are:
// listenHost:string, listenPort:int32, tlsDomain:string, tlsDirCert:string, metricsInterval:int
func (e *HTTPEndPoint) SetOption(name string, value interface{}) error {
	switch name {
	case "listenHost":
		if fmt.Sprintf("%T", value) != "string" {
			return fmt.Errorf("listenHost must be a valid string")
		}
		e.config.ListenHost = value.(string)
	case "listenPort":
		if fmt.Sprintf("%T", value) != "int32" {
			return fmt.Errorf("listenPort must be a valid int32")
		}
		e.config.ListenPort = value.(int32)
	case "tlsDomain":
		if fmt.Sprintf("%T", value) != "string" {
			return fmt.Errorf("tlsDomain must be a valid string")
		}
		e.config.TLSdomain = value.(string)
	case "tlsDirCert":
		if fmt.Sprintf("%T", value) != "string" {
			return fmt.Errorf("tlsDirCert must be a valid string")
		}
		e.config.TLSdirCert = value.(string)
	case "tlsConfig":
		if tc, ok := value.(*tls.Config); !ok {
			return fmt.Errorf("tlsConfig must be of type *tls.Config")
		} else {
			e.config.TLSconfig = tc
		}
	case "metricsInterval":
		if fmt.Sprintf("%T", value) != "int" {
			return fmt.Errorf("metricsInterval must be a valid int")
		}
		if e.config.Metrics == nil {
			e.config.Metrics = &metrics.Metrics{}
		}
		e.config.Metrics.Enabled = true
		e.config.Metrics.RefreshInterval = value.(int)
	}
	return nil
}

// Init creates a new websockets/http mixed endpoint
func (e *HTTPEndPoint) Init(listener chan transports.Message) error {
	log.Infof("creating API service")

	// Create a HTTP Proxy service
	pxy, err := proxy(e.config.ListenHost, e.config.ListenPort, e.config.TLSdomain, e.config.TLSdirCert)
	if err != nil {
		return err
	}
	pxy.TLSConfig = e.config.TLSconfig

	// Create a HTTP+Websocket transport and attach the proxy
	ts := new(mhttp.HttpHandler)
	ts.Init(new(transports.Connection))
	ts.SetProxy(pxy)
	go ts.Listen(listener)

	// Attach the metrics agent (Prometheus)
	var ma *metrics.Agent
	if e.config.Metrics != nil && e.config.Metrics.Enabled {
		ma = metrics.NewAgent("/metrics", time.Second*time.Duration(e.config.Metrics.RefreshInterval), pxy)
	}
	e.id = "http"
	e.Proxy = pxy
	e.MetricsAgent = ma
	e.transport = ts
	return nil
}
