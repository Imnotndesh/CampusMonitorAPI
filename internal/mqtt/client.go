package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"CampusMonitorAPI/internal/config"
	"CampusMonitorAPI/internal/logger"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	client    mqtt.Client
	cfg       *config.MQTTConfig
	log       *logger.Logger
	handlers  map[string]MessageHandler
	mu        sync.RWMutex
	connected bool
	ctx       context.Context
	cancel    context.CancelFunc
}

type MessageHandler func(topic string, payload []byte) error

type ClientConfig struct {
	MQTT   *config.MQTTConfig
	Logger *logger.Logger
}

func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		cfg:      cfg.MQTT,
		log:      cfg.Logger,
		handlers: make(map[string]MessageHandler),
		ctx:      ctx,
		cancel:   cancel,
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", cfg.MQTT.Broker, cfg.MQTT.Port))
	opts.SetClientID(cfg.MQTT.ClientID)
	opts.SetKeepAlive(cfg.MQTT.KeepAlive)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetConnectTimeout(cfg.MQTT.ConnectTimeout)
	opts.SetAutoReconnect(cfg.MQTT.AutoReconnect)
	opts.SetCleanSession(true)

	if cfg.MQTT.Username != "" {
		opts.SetUsername(cfg.MQTT.Username)
		opts.SetPassword(cfg.MQTT.Password)
	}

	opts.SetOnConnectHandler(c.onConnect)
	opts.SetConnectionLostHandler(c.onConnectionLost)
	opts.SetReconnectingHandler(c.onReconnecting)

	c.client = mqtt.NewClient(opts)

	return c, nil
}

func (c *Client) Connect() error {
	c.log.Info("Connecting to MQTT broker: %s:%d", c.cfg.Broker, c.cfg.Port)

	token := c.client.Connect()
	if !token.WaitTimeout(c.cfg.ConnectTimeout) {
		return fmt.Errorf("connection timeout after %v", c.cfg.ConnectTimeout)
	}

	if err := token.Error(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	c.log.Info("Successfully connected to MQTT broker")
	return nil
}

func (c *Client) Disconnect() error {
	c.log.Info("Disconnecting from MQTT broker")

	c.cancel()

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	c.client.Disconnect(250)

	c.log.Info("Disconnected from MQTT broker")
	return nil
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.client.IsConnected()
}

func (c *Client) Subscribe(topic string, handler MessageHandler) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to broker")
	}

	c.mu.Lock()
	c.handlers[topic] = handler
	c.mu.Unlock()

	c.log.Debug("Subscribing to topic: %s (QoS: %d)", topic, c.cfg.QoS)

	token := c.client.Subscribe(topic, c.cfg.QoS, func(client mqtt.Client, msg mqtt.Message) {
		c.handleMessage(msg)
	})

	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe timeout for topic: %s", topic)
	}

	if err := token.Error(); err != nil {
		return fmt.Errorf("subscribe failed for topic %s: %w", topic, err)
	}

	c.log.Info("Successfully subscribed to topic: %s", topic)
	return nil
}

func (c *Client) Unsubscribe(topic string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to broker")
	}

	c.log.Debug("Unsubscribing from topic: %s", topic)

	token := c.client.Unsubscribe(topic)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("unsubscribe timeout for topic: %s", topic)
	}

	if err := token.Error(); err != nil {
		return fmt.Errorf("unsubscribe failed for topic %s: %w", topic, err)
	}

	c.mu.Lock()
	delete(c.handlers, topic)
	c.mu.Unlock()

	c.log.Info("Successfully unsubscribed from topic: %s", topic)
	return nil
}

func (c *Client) Publish(topic string, payload []byte) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to broker")
	}

	c.log.Debug("Publishing to topic: %s (size: %d bytes)", topic, len(payload))

	token := c.client.Publish(topic, c.cfg.QoS, c.cfg.RetainMessages, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout for topic: %s", topic)
	}

	if err := token.Error(); err != nil {
		return fmt.Errorf("publish failed for topic %s: %w", topic, err)
	}

	c.log.Debug("Successfully published to topic: %s", topic)
	return nil
}

func (c *Client) PublishJSON(topic string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return c.Publish(topic, payload)
}

func (c *Client) handleMessage(msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	c.log.Debug("Received message on topic: %s (size: %d bytes)", topic, len(payload))

	c.mu.RLock()
	handler, exists := c.handlers[topic]
	c.mu.RUnlock()

	if !exists {
		for registeredTopic, h := range c.handlers {
			if matchTopic(registeredTopic, topic) {
				handler = h
				exists = true
				break
			}
		}
	}

	if !exists {
		c.log.Warn("No handler found for topic: %s", topic)
		return
	}

	if err := handler(topic, payload); err != nil {
		c.log.Error("Handler error for topic %s: %v", topic, err)
	}
}

func (c *Client) onConnect(client mqtt.Client) {
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	c.log.Info("MQTT connection established")

	c.mu.RLock()
	topics := make([]string, 0, len(c.handlers))
	for topic := range c.handlers {
		topics = append(topics, topic)
	}
	c.mu.RUnlock()

	for _, topic := range topics {
		c.log.Debug("Re-subscribing to topic: %s", topic)
		token := client.Subscribe(topic, c.cfg.QoS, func(client mqtt.Client, msg mqtt.Message) {
			c.handleMessage(msg)
		})
		if token.Wait() && token.Error() != nil {
			c.log.Error("Failed to re-subscribe to %s: %v", topic, token.Error())
		}
	}
}

func (c *Client) onConnectionLost(client mqtt.Client, err error) {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	c.log.Error("MQTT connection lost: %v", err)
}

func (c *Client) onReconnecting(client mqtt.Client, opts *mqtt.ClientOptions) {
	c.log.Warn("Attempting to reconnect to MQTT broker...")
}

func matchTopic(pattern, topic string) bool {
	if pattern == topic {
		return true
	}

	patternParts := splitTopic(pattern)
	topicParts := splitTopic(topic)

	if len(patternParts) > len(topicParts) {
		return false
	}

	for i, part := range patternParts {
		if part == "#" {
			return true
		}
		if part == "+" {
			continue
		}
		if i >= len(topicParts) || part != topicParts[i] {
			return false
		}
	}

	return len(patternParts) == len(topicParts)
}

func splitTopic(topic string) []string {
	parts := []string{}
	current := ""
	for _, c := range topic {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
