package main

/*
Messages have the following structure:

{
  "request": {
    "error": "no signature provided",
    "request": "1234",
    "timestamp": 1602582404
  },
  "id": "1234",
  "signature": "6e1f5705f41c767d6d3ba516..."
}

You can test with curl:

curl -s 127.0.0.1:7788/main -X POST -d '{"request":{"method":"hello", "request":"1234"}, "id":"1234"}'
*/

import (
	"fmt"

	"github.com/vocdoni/multirpc/endpoint"
	"github.com/vocdoni/multirpc/example/message"
	"github.com/vocdoni/multirpc/router"
	"github.com/vocdoni/multirpc/types"

	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
)

func main() {
	log.Init("debug", "stdout")

	// API configuration
	api := &types.API{
		ListenHost: "0.0.0.0",
		ListenPort: 7788,
	}
	// Generate signing keys
	sig := ethereum.NewSignKeys()
	sig.Generate()

	// Create HTTPWS endpoint
	ep, err := endpoint.NewHttpWsEndpoint(api, sig, message.NewAPI)
	if err != nil {
		panic(err)
	}

	// Add namespace /main to the transport httpws
	ep.Router.Transports["httpws"].AddNamespace("/main")

	// And handler for namespace main and method hello
	if err := ep.Router.AddHandler("hello", "/main", hello, false, true); err != nil {
		log.Fatal(err)
	}

	if err := ep.Router.AddHandler("addkey", "/main", addKey, false, false); err != nil {
		log.Fatal(err)
	}

	// Add a private method
	if err := ep.Router.AddHandler("getsecret", "/main", getSecret, true, false); err != nil {
		log.Fatal(err)
	}

	// Start routing
	ep.Router.Route()
}

//////////////
// Handlers //
//////////////

func hello(rr types.RouterRequest) {
	msg := &message.MyAPI{}
	msg.ID = rr.Id
	msg.Reply = fmt.Sprintf("hello! got your message with ID %s", rr.Id)
	rr.Send(router.BuildReply(msg, rr))
}

func addKey(rr types.RouterRequest) {
	msg := &message.MyAPI{}

	if ok := rr.Signer.Authorized[rr.Address]; ok {
		msg.Error = fmt.Sprintf("address %s already authorized", rr.Address.Hex())
	} else {
		rr.Signer.AddAuthKey(rr.Address)
		log.Infof("adding pubKey %s", rr.SignaturePublicKey)
		msg.Reply = fmt.Sprintf("added new authorized address %s", rr.Address.Hex())
	}

	rr.Send(router.BuildReply(msg, rr))
}

func getSecret(rr types.RouterRequest) {
	msg := &message.MyAPI{Reply: "the secret is foobar123456"}
	rr.Send(router.BuildReply(msg, rr))
}
