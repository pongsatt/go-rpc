package rpc_test

import (
	"errors"
	"testing"
	"time"

	"github.com/pongsatt/go-rpc"
	"github.com/pongsatt/go-rpc/messaging"
	"github.com/pongsatt/go-rpc/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRequestReplyClient_Request(t *testing.T) {
	type args struct {
		key               string
		payload           string
		isLocal           bool
		skipReply         bool
		errIsLocalConsume error
		errConsumeReply   error
		errPublish        error
		errHandle         error
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"local",
			args{key: "key1", payload: "payload1", isLocal: true},
			"payload1", false,
		},
		{"request-reply",
			args{key: "key1", payload: "payload1"},
			"payload1", false,
		},
		{"error-isConsumeLocal",
			args{key: "key1", payload: "payload1", isLocal: true, errIsLocalConsume: errors.New("")},
			"", true,
		},
		{"error-consume-reply",
			args{key: "key1", payload: "payload1", errConsumeReply: errors.New("")},
			"", true,
		},
		{"error-request-reply-handle",
			args{key: "key1", payload: "payload1", isLocal: true, errHandle: errors.New("")},
			"payload1", true,
		},
		{"error-publish",
			args{key: "key1", payload: "payload1", errPublish: errors.New("")},
			"payload1", true,
		},
		{"error-timeout",
			args{key: "key1", payload: "payload1", skipReply: true},
			"payload1", true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mocks.MessagingClient{}

			mockClient.On("IsLocalConsume", mock.Anything, tt.args.key).
				Return(tt.args.isLocal, tt.args.errIsLocalConsume)

			mockClient.On("Consume", "test-request", mock.Anything, mock.Anything).
				Return(nil)

			var replyMessage func(msg *messaging.Msg) error

			mockClient.On("Publish", mock.Anything).
				Return(tt.args.errPublish).
				Run(func(args mock.Arguments) {
					msg := args.Get(0).(*messaging.Msg)

					if !tt.args.skipReply {
						go replyMessage(msg)
					}
				})

			mockClient.On("Consume", "test-reply", mock.Anything, mock.Anything).
				Return(tt.args.errConsumeReply).
				Run(func(args mock.Arguments) {
					replyMessage = args.Get(2).(func(msg *messaging.Msg) error)
				})

			timeout := 1 * time.Second
			config := &rpc.RequestReplyConfig{Timeout: &timeout}
			c := rpc.NewRequestReplyClient("test", &mockClient, config)

			mockHandler := func(payload []byte) ([]byte, error) {
				return payload, tt.args.errHandle
			}

			c.SubscribeRequest(mockHandler)
			got, err := c.Request(tt.args.key, []byte(tt.args.payload))

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, string(got), tt.want)
		})
	}
}

func TestRequestReplyClient_SubscribeRequest(t *testing.T) {
	type args struct {
		retHandle         string
		errHandle         error
		errConsumeRequest error
		errPublish        error
	}
	tests := []struct {
		name                    string
		args                    args
		want                    string
		wantSubscribeRequestErr bool
		wantRequestMessageErr   bool
	}{
		{
			"success", args{
				retHandle: "response1",
			},
			"response1", false, false,
		},
		{
			"error-handler", args{
				retHandle: "response1",
				errHandle: errors.New(""),
			},
			"response1", false, true,
		},
		{
			"error-publish", args{
				retHandle:  "response1",
				errPublish: errors.New(""),
			},
			"response1", false, true,
		},
		{
			"error-consume", args{
				retHandle:         "response1",
				errConsumeRequest: errors.New(""),
			},
			"response1", true, false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MessagingClient{}

			handler := func(payload []byte) ([]byte, error) {
				if tt.args.errHandle != nil {
					return nil, tt.args.errHandle
				}
				return []byte(tt.args.retHandle), nil
			}

			var requestMessage func(msg *messaging.Msg) error

			mockClient.On("Consume", "test-request", mock.Anything, mock.Anything).
				Return(tt.args.errConsumeRequest).
				Run(func(args mock.Arguments) {
					requestMessage = args.Get(2).(func(msg *messaging.Msg) error)
				})

			var publishMsg *messaging.Msg

			mockClient.On("Publish", mock.Anything).
				Return(tt.args.errPublish).
				Run(func(args mock.Arguments) {
					publishMsg = args.Get(0).(*messaging.Msg)
				})

			c := rpc.NewRequestReplyClient("test", mockClient, &rpc.RequestReplyConfig{})

			err := c.SubscribeRequest(handler)

			if tt.wantSubscribeRequestErr {
				assert.Error(t, err)
				return
			}

			requestMsg := &messaging.Msg{
				ID:    "1",
				Reply: "test-reply",
				Key:   "key1",
			}
			err = requestMessage(requestMsg)

			if tt.wantRequestMessageErr {
				assert.Error(t, err)
				return
			}

			expectedPublishMsg := &messaging.Msg{
				ID:    "1",
				Topic: "test-reply",
				Key:   "key1",
				Value: []byte(tt.want),
			}

			assert.Equal(t, expectedPublishMsg, publishMsg)

		})
	}
}
