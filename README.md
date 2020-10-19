# multirpc

A JSON driven, signature ready, letsencrypt compatible modular RPC stack for multiple transports.

This GoLang module provides a full stack for implementing a flexible RPC service.
The same mux router could be used for providing service to multiple network transports in paralel.

The aim for multirpc is to provide and easy way to implement an RPC communication over any transport layer, 
using the same JSON structure and signature schema.
The API consumer only needs to provide the application layer (define the custom JSON fields and handlers).

The RPC provides security mechanisms for validating the origin and for authentication (using secp256k1 cryptography).

The network encryption (if any) is handled on the transport layer.

Current transports supported:
+ `HTTP` with go-chi
+ `HTTPs` with go-chi and letsencrypt
+ `WS` with "nhooyr.io/websocket"
+ `WSS` with "nhooyr.io/websocket" and letsencrypt
+ `libp2p` with libp2p and a custom pubsub protocol

More could be easy added, see the `transports` module.

All JSON requests follows the next schema which is automatically handled by this module.

```json
{
  "request": {
    "request": "1234",
    "timestamp": 1602582404,

    "customfields...":"..."
  },
  "id": "1234",
  "signature": "6e1f5705f41c767d6d3ba516..."
}
```

The module consumer only needs to provide a compatible (`types.MessageAPI`) struct, as the following example:

```golang
type MyAPI struct {
	ID        string   `json:"request"`
	Method    string   `json:"method,omitempty"`
	PubKeys   []string `json:"pubKeys,omitempty"`
	Timestamp int32    `json:"timestamp"`
	Error     string   `json:"error,omitempty"`
	Reply     string   `json:"reply,omitempty"`
}

func (ma *MyAPI) GetID() string {
	return ma.ID
}

func (ma *MyAPI) SetID(id string) {
	ma.ID = id
}

func (ma *MyAPI) SetTimestamp(ts int32) {
	ma.Timestamp = ts
}

func (ma *MyAPI) SetError(e string) {
	ma.Error = e
}

func (ma *MyAPI) GetMethod() string {
	return ma.Method
}

func NewAPI() types.MessageAPI {
	return &MyAPI{}
}
```

So `GetID()`, `SetID()`, `SetTimestamp()`, `SetError()`, `GetMethod()` must be implemented.

Also a special standalone function that returns the custom type is required `NewApi()`.

### HTTP+WS

To start the HTTP(s) +  Websocket multirpc stack, the `endpoint` package can be used as follows.

```golang
	// API configuration
	api := &types.API{
		ListenHost: "0.0.0.0",
		ListenPort: 7788,
	}
	// Generate signing keys
	sig := ethereum.NewSignKeys()
	sig.Generate()

	// Create the channel for incoming messages and attach to transport
	listener := make(chan types.Message)

	// Create HTTPWS endpoint (for HTTP(s) + Websockets(s) handling) using the endpoint helper
	ep, err := endpoint.NewHttpWsEndpoint(api, sig, listener)
	if err != nil {
		panic(err)
	}

	// Create the transports map, this allows adding several transports on the same router
	transportMap := make(map[string]transports.Transport)
	transportMap[ep.ID()] = ep.Transport

	// Create a new router and attach the transports
	r := router.NewRouter(listener, transportMap, sig, message.NewAPI)

	// Add namespace /main to the transport httpws
	r.Transports[ep.ID()].AddNamespace("/main")

	// And handler for namespace main and method hello
	if err := r.AddHandler("hello", "/main", hello, false, true); err != nil {
		log.Fatal(err)
	}

	if err := r.AddHandler("addkey", "/main", addKey, false, false); err != nil {
		log.Fatal(err)
	}

	// Add a private method
	if err := r.AddHandler("getsecret", "/main", getSecret, true, false); err != nil {
		log.Fatal(err)
	}

	// Start routing
	r.Route()
```

### HTTP+WS with TLS

In order to enable TLS encryption with letsencrypt, the API must be configured as follows:

```golang
    api := &types.API{
        ListenHost: "0.0.0.0",
        ListenPort: 443,
        TLSdomain:  "myValidDomain.com",
        TLSdirCert: "/tmp/tlsdir"
    }
```	

### Example

Find the full example on the `example` directory of this repository.

Start the server

```js
$ go run example/server/server.go 
2020-10-13T14:42:54+02:00       INFO    server/server.go:34     logger construction succeeded at level debug and output stdout
2020-10-13T14:42:54+02:00       INFO    endpoint/endpoint.go:27 creating API service
2020-10-13T14:42:54+02:00       INFO    endpoint/endpoint.go:65 creating proxy service, listening on 0.0.0.0:7788
2020-10-13T14:42:54+02:00       INFO    net/proxy.go:128        starting go-chi http server
2020-10-13T14:42:54+02:00       INFO    net/proxy.go:139        proxy ready at http://[::]:7788
2020-10-13T14:42:54+02:00       DEBUG   router/router.go:45     adding new handler hello for namespace /main
2020-10-13T14:42:54+02:00       DEBUG   router/router.go:45     adding new handler addkey for namespace /main
2020-10-13T14:42:54+02:00       DEBUG   router/router.go:45     adding new handler getsecret for namespace /main
```

At this point you can already use `curl` for making http requests.

```js
$ curl -s 127.0.0.1:7788/main -X POST -d '{"request":{"method":"hello", "request":"1234"}, "id":"1234"}'

{"request":{"reply":"hello! got your message with ID 1234","request":"1234","timestamp":1602593026},"id":"1234","signature":"5ddc0fd1a13c7612875c089feea712bae6df2d05c5cea3b4e9cfaf6e109ae3bb1d3b6915f8933f3ea20179209a076e1e7fd6328efdabc08c347d9c5807eb2bef01"}
```

But in order to use the signature mechanism, a more advanced client tool must be used. In this case we provide a `client.go` example code.

```js
$ echo '{"method":"getsecret"}' | go run example/client/client.go -key=4f81e884843a5910af16dd85424bdd6a4bb524159abeee798ed557cd6418eb17
{"error":"invalid authentication","request":"539","timestamp":1602593846}
```

```js
 $ echo '{"method":"addkey"}' | go run example/client/client.go -key=4f81e884843a5910af16dd85424bdd6a4bb524159abeee798ed557cd6418eb17

{"reply":"added new authorized address 0xD7B5E12Fbe91Efd61E06B35A7bc06028cbe0209E","request":"28","timestamp":1602593157}
```

```js
$ echo '{"method":"getsecret"}' | go run example/client/client.go -key=4f81e884843a5910af16dd85424bdd6a4bb524159abeee798ed557cd6418eb17

{"reply":"the secret is foobar123456","request":"691","timestamp":1602593187}
```
