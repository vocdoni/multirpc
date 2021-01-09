package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"

	"nhooyr.io/websocket"

	"github.com/vocdoni/multirpc/example/httpws/message"
	"github.com/vocdoni/multirpc/router"
	"gitlab.com/vocdoni/go-dvote/crypto"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
)

// APIConnection holds an API websocket connection
type APIConnection struct {
	Conn *websocket.Conn
}

// NewAPIConnection starts a connection with the given endpoint address. The
// connection is closed automatically when the test or benchmark finishes.
func NewAPIConnection(addr string) *APIConnection {
	r := &APIConnection{}
	var err error
	r.Conn, _, err = websocket.Dial(context.TODO(), addr, nil)
	if err != nil {
		log.Fatal(err)
	}
	return r
}

// Request makes a request to the previously connected endpoint
func (r *APIConnection) Request(req *message.MyAPI, signer *ethereum.SignKeys) *router.RequestMessage {

	// Prepare and send request
	req.Timestamp = (int32(time.Now().Unix()))
	req.ID = fmt.Sprintf("%d", rand.Intn(1000))
	reqInner, err := crypto.SortedMarshalJSON(req)
	if err != nil {
		log.Fatalf("%s: %v", req.Method, err)
	}
	var signature string
	if signer != nil {
		signature, err = signer.Sign(reqInner)
		if err != nil {
			log.Fatalf("%s: %v", req.Method, err)
		}
	}

	reqOuter := router.RequestMessage{
		ID:         req.ID,
		Signature:  signature,
		MessageAPI: reqInner,
	}
	reqBody, err := json.Marshal(reqOuter)
	if err != nil {
		log.Fatalf("%s: %v", req.Method, err)
	}
	log.Infof("sending: %s", reqBody)
	if err := r.Conn.Write(context.TODO(), websocket.MessageText, reqBody); err != nil {
		log.Fatalf("%s: %v", req.Method, err)
	}

	// Receive response
	_, message, err := r.Conn.Read(context.TODO())
	log.Infof("received: %s", message)
	if err != nil {
		log.Fatalf("%s: %v", req.Method, err)
	}

	var respOuter router.RequestMessage
	if err := json.Unmarshal(message, &respOuter); err != nil {
		log.Fatalf("%v", err)
	}
	if respOuter.ID != reqOuter.ID {
		log.Fatalf("%s: %v", req.Method, "request ID doesn'tb match")
	}
	if respOuter.Signature == "" {
		log.Fatalf("%s: empty signature in response: %s", req.Method, message)
	}

	return &respOuter
}

func processLine(input []byte) *message.MyAPI {
	var req message.MyAPI
	err := json.Unmarshal(input, &req)
	if err != nil {
		panic(err)
	}
	return &req
}

func main() {
	host := flag.String("host", "ws://127.0.0.1:7788/main", "URL to connect to")
	logLevel := flag.String("logLevel", "error", "log level <debug, info, warn, error>")
	privKey := flag.String("key", "", "private key for signature (leave blank for auto-generate)")
	flag.Parse()
	log.Init(*logLevel, "stdout")
	rand.Seed(time.Now().UnixNano())

	// Set or generate client signing key
	signer := ethereum.NewSignKeys()
	if *privKey != "" {
		if err := signer.AddHexKey(*privKey); err != nil {
			panic(err)
		}
	} else {
		signer.Generate()
	}

	log.Infof("connecting to %s", *host)
	c := NewAPIConnection(*host)
	defer c.Conn.Close(websocket.StatusNormalClosure, "")

	var req *message.MyAPI
	reader := bufio.NewReader(os.Stdin)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		if len(line) < 7 || strings.HasPrefix(string(line), "#") {
			continue
		}
		req = processLine(line)
		resp := c.Request(req, signer)
		fmt.Printf("%s\n", resp.MessageAPI)
	}
}