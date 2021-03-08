package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pongsatt/go-rpc"
	"github.com/pongsatt/go-rpc/example"
	"github.com/pongsatt/go-rpc/messaging"
)

func main() {
	kafkaClient := messaging.NewCpKafkaClient(&messaging.CpKafkaConfig{
		Brokers: "localhost:9092",
	})

	timeout := 30 * time.Second
	requestReplyClient := rpc.NewRequestReplyClient("RealServer", kafkaClient, &rpc.RequestReplyConfig{
		Timeout: &timeout,
	})

	// interface -> proxy
	proxy := example.NewServerProxy(requestReplyClient)

	// execute normal code
	client := example.NewCleint(proxy)

	for i := 0; i < 1; i++ {
		start := time.Now()
		id, err := client.Create("test")
		fmt.Printf("time use %s\n", time.Since(start))

		if err != nil {
			fmt.Printf("error creating %v\n", err)
		} else {
			fmt.Printf("got id %s\n", id)
		}
	}

	termChan := make(chan os.Signal)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)

	<-termChan
	kafkaClient.Shutdown()
}
