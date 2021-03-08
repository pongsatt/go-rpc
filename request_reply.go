// go:generate mockery --name=MessagingClient

package rpc

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/pongsatt/go-rpc/messaging"
)

var (
	defaultTimeout = 30 * time.Second
)

// request represents a request from client with correlation id
type request struct {
	ID string
}

// response represents the reply message from the server with correlation id
type response struct {
	ID      string
	Payload []byte
}

// pendingCall represents an active request
type pendingCall struct {
	Req   request
	Res   response
	Done  chan bool
	Error error
}

// MessagingClient represents messaging client such as kafka
type MessagingClient interface {
	Publish(msg *messaging.Msg) error
	Consume(topic string, groupID string, handler func(msg *messaging.Msg) error) error
	IsLocalConsume(topic string, key string) (bool, error)
}

// RequestReplyConfig represents request reply configuration
type RequestReplyConfig struct {
	Timeout *time.Duration
}

// RequestReplyClient represents client instance
type RequestReplyClient struct {
	topic             string
	mutex             sync.Mutex
	messagingClient   MessagingClient
	pending           map[string]*pendingCall
	isReplySubscribed bool
	handler           func(payload []byte) ([]byte, error)
	config            *RequestReplyConfig
}

// NewRequestReplyClient creates new request reply client instance
func NewRequestReplyClient(topic string, client MessagingClient, config *RequestReplyConfig) *RequestReplyClient {
	if config.Timeout == nil {
		config.Timeout = &defaultTimeout
	}

	return &RequestReplyClient{
		topic:           topic,
		pending:         make(map[string]*pendingCall, 1),
		messagingClient: client,
		config:          config,
	}
}

func newCall(req request) *pendingCall {
	done := make(chan bool)
	return &pendingCall{
		Req:  req,
		Done: done,
	}
}

func (c *RequestReplyClient) getRequestTopic() string {
	return c.topic + "-request"
}

func (c *RequestReplyClient) getRequestGroupID() string {
	return "go-rpc-request-group1"
}

func (c *RequestReplyClient) getReplyTopic() string {
	return c.topic + "-reply"
}

func (c *RequestReplyClient) getReplyGroupID() string {
	return "go-rpc-reply-group1"
}

// Request sends a request with payload and wait for a response.
// key is used as a partition key.
func (c *RequestReplyClient) Request(key string, payload []byte) ([]byte, error) {
	isLocal, err := c.messagingClient.IsLocalConsume(c.getRequestTopic(), key)

	if err != nil {
		return nil, err
	}

	if isLocal && c.handler != nil {
		return c.handler(payload)
	}

	if !c.isReplySubscribed {
		err = c.subscribeReply()

		if err != nil {
			return nil, err
		}
		c.isReplySubscribed = true
	}

	c.mutex.Lock()
	id := uuid.NewString()
	req := request{ID: id}
	call := newCall(req)

	c.pending[id] = call

	err = c.messagingClient.Publish(&messaging.Msg{
		Topic: c.getRequestTopic(),
		Key:   key,
		Value: payload,
		Reply: c.getReplyTopic(),
		ID:    id,
	})

	if err != nil {
		delete(c.pending, id)
		c.mutex.Unlock()
		return nil, err
	}
	c.mutex.Unlock()

	select {
	case <-call.Done:
	case <-time.After(*c.config.Timeout):
		call.Error = errors.New("request timeout")
	}

	if call.Error != nil {
		return nil, call.Error
	}

	return call.Res.Payload, nil
}

// SubscribeRequest waits for a request to come in and call the handler func
func (c *RequestReplyClient) SubscribeRequest(handler func(payload []byte) ([]byte, error)) error {
	c.handler = handler

	return c.messagingClient.Consume(c.getRequestTopic(), c.getRequestGroupID(), func(msg *messaging.Msg) error {
		respPayload, err := handler(msg.Value)

		if err != nil {
			return err
		}

		err = c.messagingClient.Publish(&messaging.Msg{
			Topic: msg.Reply,
			Key:   msg.Key,
			Value: respPayload,
			ID:    msg.ID,
		})

		if err != nil {
			return err
		}

		return nil
	})
}

// subscribeReply processes reply message
func (c *RequestReplyClient) subscribeReply() error {
	return c.messagingClient.Consume(c.getReplyTopic(), c.getReplyGroupID(), func(msg *messaging.Msg) error {
		resp := response{
			ID:      msg.ID,
			Payload: msg.Value,
		}

		err := c.readReplyMessage(resp)

		if err != nil {
			return err
		}

		return nil
	})
}

func (c *RequestReplyClient) readReplyMessage(res response) error {
	c.mutex.Lock()
	call := c.pending[res.ID]
	delete(c.pending, res.ID)
	c.mutex.Unlock()

	if call == nil {
		return errors.New("no pending request found")
	}
	call.Res = res
	call.Done <- true
	return nil
}
