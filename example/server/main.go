package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pongsatt/go-rpc"
	"github.com/pongsatt/go-rpc/example"
	"github.com/pongsatt/go-rpc/messaging"
)

func main() {
	kafkaClient := messaging.NewCpKafkaClient(&messaging.CpKafkaConfig{
		Brokers: "localhost:9092",
	})

	requestReplyClient := rpc.NewRequestReplyClient("RealServer", kafkaClient, &rpc.RequestReplyConfig{})

	err := example.NewRealServerProvider(
		requestReplyClient, &example.RealServer{})
	if err != nil {
		panic(err)
	}
	fmt.Println("server started")

	termChan := make(chan os.Signal)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM)

	<-termChan
	kafkaClient.Shutdown()
}
