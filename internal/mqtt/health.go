// internal/mqtt/health.go

package mqtt

import (
	"context"
	"fmt"
	"time"
)

type HealthStatus struct {
	Connected      bool      `json:"connected"`
	LastConnected  time.Time `json:"last_connected,omitempty"`
	LastDisconnect time.Time `json:"last_disconnect,omitempty"`
	Subscriptions  int       `json:"subscriptions"`
}

func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := &HealthStatus{
		Connected:     c.connected && c.client.IsConnected(),
		Subscriptions: len(c.handlers),
	}

	return status, nil
}

func (c *Client) WaitForConnection(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if c.IsConnected() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("connection timeout after %v", timeout)
}
