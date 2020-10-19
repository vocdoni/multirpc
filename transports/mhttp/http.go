package mhttp

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/vocdoni/multirpc/transports"
	"gitlab.com/vocdoni/go-dvote/log"
)

type HttpHandler struct {
	Proxy            *Proxy // proxy where the ws will be associated
	internalReceiver chan transports.Message
}

type HttpContext struct {
	Writer  http.ResponseWriter
	Request *http.Request

	sent chan struct{}
}

func (h *HttpHandler) Init(c *transports.Connection) error {
	h.internalReceiver = make(chan transports.Message, 1)
	return nil
}

func (h *HttpHandler) SetProxy(p *Proxy) {
	h.Proxy = p
}

func getHTTPhandler(path string, receiver chan transports.Message) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		respBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Warnf("HTTP connection closed: (%s)", err)
			return
		}
		hc := &HttpContext{Request: r, Writer: w, sent: make(chan struct{})}
		msg := transports.Message{
			Data:      respBody,
			TimeStamp: int32(time.Now().Unix()),
			Context:   hc,
			Namespace: path,
		}
		receiver <- msg

		// Don't return this func until a response is sent, because the
		// connection is closed when the handler returns.
		select {
		case <-hc.sent:
			// a response was sent.
		case <-r.Context().Done():
			// we hit chi's timeout.
		}
	}
}

// AddProxyHandler adds the current websocket handler into the Proxy
func (h *HttpHandler) AddProxyHandler(path string) {
	h.Proxy.AddHandler(path, getHTTPhandler(path, h.internalReceiver))
}

func (h *HttpContext) ConnectionType() string {
	return "HTTP"
}

func (h *HttpContext) Send(msg transports.Message) error {
	defer close(h.sent)

	h.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg.Data)+1))
	h.Writer.Header().Set("Content-Type", "application/json")
	if _, err := h.Writer.Write(msg.Data); err != nil {
		return err
	}
	// Ensure we end the response with a newline, to be nice.
	_, err := h.Writer.Write([]byte("\n"))
	return err
}

func (h *HttpHandler) ConnectionType() string {
	return "HTTP"
}

func (h *HttpHandler) Listen(receiver chan<- transports.Message) {
	for {
		msg := <-h.internalReceiver
		receiver <- msg
	}
}

func (h *HttpHandler) SendUnicast(address string, msg transports.Message) error {
	// WebSocket is not p2p so sendUnicast makes the same of Send()
	return h.Send(msg)
}

func (h *HttpHandler) Send(msg transports.Message) error {
	// TODO(mvdan): this extra abstraction layer is probably useless
	return msg.Context.(*HttpContext).Send(msg)
}

func (h *HttpHandler) SetBootnodes(bootnodes []string) {
	// No bootnodes on websockets handler
}

func (h *HttpHandler) AddPeer(peer string) error {
	// No peers on websockets handler
	return nil
}

// AddNamespace adds a new namespace to the transport
func (h *HttpHandler) AddNamespace(namespace string) error {
	if len(namespace) == 0 || namespace[0] != '/' {
		return fmt.Errorf("namespace on http must start with /")
	}
	h.AddProxyHandler(namespace)
	return nil
}

func (h *HttpHandler) Address() string {
	return h.String()
}

func (h *HttpHandler) String() string {
	return h.Proxy.Addr.String()
}
