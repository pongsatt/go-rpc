package messaging

import (
	"context"
	"fmt"
	"hash/crc32"
	"math/rand"
	"sync"

	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"
)

// CpKafkaConfig struct
type CpKafkaConfig struct {
	Brokers string
}

type cpSubscription struct {
	topic    string
	cancel   context.CancelFunc
	doneChan chan bool
	partions []int32
}

// CpKafkaClient struct
type CpKafkaClient struct {
	config               *CpKafkaConfig
	subscriptions        map[string]*cpSubscription
	subscriptionsMutex   sync.Mutex
	producer             *kafka.Producer
	topicPartitions      map[string][]int32
	topicPartitionsMutex sync.Mutex
}

// NewCpKafkaClient creates instance
func NewCpKafkaClient(config *CpKafkaConfig) *CpKafkaClient {
	return &CpKafkaClient{
		config:          config,
		subscriptions:   make(map[string]*cpSubscription),
		topicPartitions: make(map[string][]int32),
	}
}

func (client *CpKafkaClient) getPartition(topic string, key []byte) (int32, error) {
	partitions, err := client.getPartitions(topic)

	if err != nil {
		return -1, err
	}

	if len(key) == 0 {
		return partitions[rand.Int()%len(partitions)], nil
	}

	idx := crc32.ChecksumIEEE(key) % uint32(len(partitions))
	return partitions[idx], nil
}

func (client *CpKafkaClient) getPartitions(topic string) ([]int32, error) {
	client.topicPartitionsMutex.Lock()
	partitions, ok := client.topicPartitions[topic]
	client.topicPartitionsMutex.Unlock()

	if ok {
		return partitions, nil
	}

	producer, err := client.getProducer()

	if err != nil {
		return nil, err
	}

	meta, err := producer.GetMetadata(&topic, false, 5000)

	if err != nil {
		return nil, err
	}

	topicMeta := meta.Topics[topic]

	partitions = make([]int32, 0)

	for _, partition := range topicMeta.Partitions {
		partitions = append(partitions, partition.ID)
	}

	client.topicPartitionsMutex.Lock()
	client.topicPartitions[topic] = partitions
	client.topicPartitionsMutex.Unlock()

	return partitions, nil
}

func (client *CpKafkaClient) getConsumer(groupID string) (*kafka.Consumer, error) {
	return kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":               client.config.Brokers,
		"session.timeout.ms":              6000,
		"group.id":                        groupID,
		"go.events.channel.enable":        true,
		"go.application.rebalance.enable": true,
		// Enable generation of PartitionEOF when the
		// end of a partition is reached.
		"enable.partition.eof": true,
		"auto.offset.reset":    "latest",
		"enable.auto.commit":   false,
	})
}

func (client *CpKafkaClient) getProducer() (*kafka.Producer, error) {
	if client.producer != nil {
		return client.producer, nil
	}

	return kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": client.config.Brokers})
}

// Shutdown all subscriptions
func (client *CpKafkaClient) Shutdown() {
	for _, sub := range client.subscriptions {
		sub.cancel()
	}

	for _, sub := range client.subscriptions {
		<-sub.doneChan
	}

	if client.producer != nil {
		client.producer.Close()
	}

}

// IsLocalConsume check if a key is assigned to local consumer
func (client *CpKafkaClient) IsLocalConsume(topic string, key string) (bool, error) {
	client.subscriptionsMutex.Lock()
	subscription, ok := client.subscriptions[topic]
	client.subscriptionsMutex.Unlock()

	if !ok || subscription.partions == nil {
		return false, nil
	}

	keyBytes := []byte(key)
	partition, err := client.getPartition(topic, keyBytes)

	if err != nil {
		return false, err
	}

	for _, assignedPartition := range subscription.partions {
		if partition == assignedPartition {
			return true, nil
		}
	}

	return false, nil
}

// Consume message
func (client *CpKafkaClient) Consume(topic string, groupID string, handler func(msg *Msg) error) error {
	c, err := client.getConsumer(groupID)

	if err != nil {
		return err
	}

	if err := c.SubscribeTopics([]string{topic}, nil); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	doneChan := make(chan bool)

	subscription := &cpSubscription{
		topic,
		cancel,
		doneChan,
		nil,
	}

	client.subscriptionsMutex.Lock()
	client.subscriptions[topic] = subscription
	client.subscriptionsMutex.Unlock()

	readyChan := make(chan bool)

	go func() {
		defer c.Close()

		run := true
		for run {
			select {
			case <-ctx.Done():
				fmt.Printf("topic: %s groupID: %s cancelled\n", topic, groupID)
				run = false
			case ev := <-c.Events():
				switch e := ev.(type) {
				case kafka.AssignedPartitions:
					c.Assign(e.Partitions)
					subscription.partions = make([]int32, 0)
					for _, p := range e.Partitions {
						subscription.partions = append(subscription.partions, p.Partition)
					}

					if readyChan != nil {
						readyChan <- true
						readyChan = nil
					}
				case kafka.RevokedPartitions:
					c.Unassign()
					subscription.partions = []int32{}
				case *kafka.Message:
					reply := ""
					id := ""

					for _, header := range e.Headers {
						if header.Key == "reply" {
							reply = string(header.Value)
						}

						if header.Key == "id" {
							id = string(header.Value)
						}
					}

					msg := &Msg{
						Topic: topic,
						Key:   string(e.Key),
						Value: e.Value,
						ID:    id,
						Reply: reply,
					}
					err := handler(msg)

					if err == nil {
						c.CommitMessage(e)
					}
				case kafka.PartitionEOF:
					fmt.Printf("%% Reached %v\n", e)
				case kafka.Error:
					// Errors should generally be considered as informational, the client will try to automatically recover
					fmt.Printf("Error: %v\n", e)
				}
			}
		}

		fmt.Printf("topic: %s groupID: %s finished\n", topic, groupID)
		doneChan <- true
	}()

	<-readyChan
	// fmt.Printf("topic: %s groupID: %s subscribed\n", topic, groupID)
	return nil
}

// Publish message
func (client *CpKafkaClient) Publish(msg *Msg) error {
	p, err := client.getProducer()

	key := []byte(msg.Key)
	partition, err := client.getPartition(msg.Topic, key)

	if err != nil {
		return err
	}

	deliveryChan := make(chan kafka.Event)
	defer close(deliveryChan)

	topic := msg.Topic
	value := msg.Value

	kafkaMsg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: partition},
		Key:            key,
		Value:          value,
	}

	if msg.ID != "" {
		kafkaMsg.Headers = append(kafkaMsg.Headers, kafka.Header{Key: "id", Value: []byte(msg.ID)})
	}

	if msg.Reply != "" {
		kafkaMsg.Headers = append(kafkaMsg.Headers, kafka.Header{Key: "reply", Value: []byte(msg.Reply)})
	}

	err = p.Produce(kafkaMsg, deliveryChan)

	if err != nil {
		return err
	}

	<-deliveryChan
	return nil
}
