// Package router provides the routing and entry point for the go-dvote API
package router

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/multirpc/transports"
	"github.com/vocdoni/multirpc/types"
	"gitlab.com/vocdoni/go-dvote/crypto"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	"gitlab.com/vocdoni/go-dvote/log"
)

type registeredMethod struct {
	public        bool
	skipSignature bool
	handler       func(types.RouterRequest)
}

// Router holds a router object
type Router struct {
	Transports  map[string]transports.Transport
	messageType func() types.MessageAPI
	methods     map[string]registeredMethod
	inbound     <-chan types.Message
	signer      *ethereum.SignKeys
}

// NewRouter creates a router multiplexer instance
func NewRouter(inbound <-chan types.Message, transports map[string]transports.Transport,
	signer *ethereum.SignKeys, messageTypeFunc func() types.MessageAPI) *Router {
	r := new(Router)
	r.methods = make(map[string]registeredMethod)
	r.inbound = inbound
	r.Transports = transports
	r.signer = signer
	r.messageType = messageTypeFunc
	return r
}

// AddHandler adds a new function handler for serving a specific method identified by name.
func (r *Router) AddHandler(method, namespace string, handler func(types.RouterRequest), private, skipSignature bool) error {
	log.Debugf("adding new handler %s for namespace %s", method, namespace)
	if private {
		return r.registerPrivate(namespace, method, handler)
	}
	return r.registerPublic(namespace, method, handler, skipSignature)
}

// Route routes requests through the Router object
func (r *Router) Route() {
	if len(r.methods) == 0 {
		log.Warnf("router methods are not properly initialized")
		return
	}
	for {
		msg := <-r.inbound
		request, err := r.getRequest(msg.Namespace, msg.Data, msg.Context)
		if err != nil {
			go r.SendError(request, err.Error())
			continue
		}

		method := r.methods[msg.Namespace+request.Method]
		if !method.skipSignature && !request.Authenticated {
			go r.SendError(request, "invalid authentication")
			continue
		}
		log.Infof("api method %s/%s", msg.Namespace, request.Method)
		log.Debugf("received: %s\n\t%+v", msg.Data, request)
		go method.handler(request)
	}
}

func (r *Router) getRequest(namespace string, payload []byte, context types.MessageContext) (request types.RouterRequest, err error) {
	// First unmarshal the outer layer, to obtain the request ID, the signed
	// request, and the signature.
	log.Debugf("got request: %s", payload)
	reqOuter := &types.RequestMessage{}
	if err := json.Unmarshal(payload, &reqOuter); err != nil {
		return request, err
	}

	request.Id = reqOuter.ID
	request.MessageContext = context
	request.Message = r.messageType()
	if err := json.Unmarshal(reqOuter.MessageAPI, request.Message); err != nil {
		return request, err
	}

	// Check both Ids are equals (avoid replay attack)
	if request.Id != request.Message.GetID() {
		return request, fmt.Errorf("request Id and message Id do not match")
	}
	request.Method = request.Message.GetMethod()

	if request.Method == "" {
		return request, fmt.Errorf("method is empty")
	}

	method, ok := r.methods[namespace+request.Method]
	if !ok {
		return request, fmt.Errorf("method not valid: (%s)", request.Method)
	}

	if !method.skipSignature {
		if len(reqOuter.Signature) < 64 {
			return request, fmt.Errorf("no signature provided")
		}
		if request.SignaturePublicKey, err = ethereum.PubKeyFromSignature(reqOuter.MessageAPI, reqOuter.Signature); err != nil {
			return request, err
		}
		// TBD: remove when everything is compressed only
		//	if request.SignaturePublicKey, err = ethereum.CompressPubKey(request.SignaturePublicKey); err != nil {
		//		return request, err
		//	}
		if len(request.SignaturePublicKey) == 0 {
			return request, fmt.Errorf("could not extract public key from signature")
		}
		if request.Address, err = ethereum.AddrFromPublicKey(request.SignaturePublicKey); err != nil {
			return request, err
		}
		log.Debugf("recovered signer address: %s", request.Address.Hex())
		request.Private = !method.public

		// If private method, check authentication
		if method.public {
			request.Authenticated = true
		} else {
			if r.signer.Authorized[request.Address] {
				request.Authenticated = true
			}
		}
	}

	// Add the signer for signing the reply
	request.Signer = r.signer
	return request, err
}

func (r *Router) registerPrivate(namespace, method string, handler func(types.RouterRequest)) error {
	if _, ok := r.methods[namespace+method]; ok {
		return fmt.Errorf("duplicate method %s for namespace %s", method, namespace)
	}
	r.methods[namespace+method] = registeredMethod{handler: handler}
	return nil
}

func (r *Router) registerPublic(namespace, method string, handler func(types.RouterRequest), skipSignature bool) error {
	if _, ok := r.methods[namespace+method]; ok {
		return fmt.Errorf("duplicate method %s for namespace %s", method, namespace)
	}
	r.methods[namespace+method] = registeredMethod{public: true, handler: handler, skipSignature: skipSignature}
	return nil
}

// SendError formats and sends an error message
func (r *Router) SendError(request types.RouterRequest, errMsg string) {
	var err error
	log.Warn(errMsg)

	// Add any last fields to the inner response, and marshal it with sorted
	// fields for signing.
	response := &types.RequestMessage{ID: request.Id}
	response.ID = request.Id

	message := r.messageType()
	message.SetID(request.Id)
	message.SetTimestamp(int32(time.Now().Unix()))
	message.SetError(errMsg)

	response.MessageAPI, err = crypto.SortedMarshalJSON(message)
	if err != nil {
		log.Error(err)
		return
	}

	// Sign the marshaled inner response.
	response.Signature, err = r.signer.Sign(response.MessageAPI)
	if err != nil {
		log.Error(err)
		// continue without the signature
	}

	if request.MessageContext != nil {
		data, err := json.Marshal(response)
		if err != nil {
			log.Warnf("error marshaling response body: %s", err)
		}
		msg := types.Message{
			TimeStamp: int32(time.Now().Unix()),
			Context:   request.MessageContext,
			Data:      data,
		}
		request.Send(msg)
	}
}

// AddAuthKey adds a new pubkey address that will have access to private methods
func (r *Router) AddAuthKey(addr common.Address) {
	r.signer.AddAuthKey(addr)
}

// DelAuthKey deletes a pubkey address from the authorized list
func (r *Router) DelAuthKey(addr common.Address) {
	delete(r.signer.Authorized, addr)
}

// BuildReply builds a response message (set ID, Timestamp and Signature)
func BuildReply(response types.MessageAPI, request types.RouterRequest) types.Message {
	var err error
	respRequest := &types.RequestMessage{ID: request.Id}

	response.SetID(request.Id)
	response.SetTimestamp(int32(time.Now().Unix()))
	respRequest.MessageAPI, err = crypto.SortedMarshalJSON(response)
	if err != nil {
		// This should never happen. If it does, return a very simple
		// plaintext error, and log the error.
		log.Error(err)
		return types.Message{
			TimeStamp: int32(time.Now().Unix()),
			Context:   request.MessageContext,
			Data:      []byte(err.Error()),
		}
	}

	// Sign the marshaled inner response.
	respRequest.Signature, err = request.Signer.Sign(respRequest.MessageAPI)
	if err != nil {
		log.Error(err)
		// continue without the signature
	}

	// We don't need to use crypto.SortedMarshalJSON here, since we don't sign these bytes.
	respData, err := json.Marshal(respRequest)
	if err != nil {
		// This should never happen. If it does, return a very simple
		// plaintext error, and log the error.
		log.Error(err)
		return types.Message{
			TimeStamp: int32(time.Now().Unix()),
			Context:   request.MessageContext,
			Data:      []byte(err.Error()),
		}
	}
	log.Debugf("response: %s", respData)
	return types.Message{
		TimeStamp: int32(time.Now().Unix()),
		Context:   request.MessageContext,
		Data:      respData,
	}
}
