// internal/mqtt/commands.go (Updated)
package mqtt

import (
	"encoding/json"
	"fmt"
	"time"

	_ "CampusMonitorAPI/internal/logger"
)

type Command struct {
	Command   string                 `json:"command"`
	CommandID string                 `json:"command_id"`
	Params    map[string]interface{} `json:"params,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

func (c *Client) SendDeepScan(probeID string, duration int) error {
	cmd := Command{
		Command:   "deep_scan",
		CommandID: fmt.Sprintf("cmd-%d", time.Now().Unix()),
		Params: map[string]interface{}{
			"duration": duration,
		},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendConfigUpdate(probeID string, config map[string]interface{}) error {
	cmd := Command{
		Command:   "config_update",
		CommandID: fmt.Sprintf("cmd-%d", time.Now().Unix()),
		Params:    config,
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendRestart(probeID string, delay int) error {
	cmd := Command{
		Command:   "restart",
		CommandID: fmt.Sprintf("cmd-%d", time.Now().Unix()),
		Params: map[string]interface{}{
			"delay": delay,
		},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendOTAUpdate(probeID string, url string, version string) error {
	cmd := Command{
		Command:   "ota_update",
		CommandID: fmt.Sprintf("cmd-%d", time.Now().Unix()),
		Params: map[string]interface{}{
			"url":     url,
			"version": version,
		},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendPing(probeID string) error {
	cmd := Command{
		Command:   "ping",
		CommandID: fmt.Sprintf("cmd-%d", time.Now().Unix()),
		Params:    map[string]interface{}{},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendGetStatus(probeID string) error {
	cmd := Command{
		Command:   "get_status",
		CommandID: fmt.Sprintf("cmd-%d", time.Now().Unix()),
		Params:    map[string]interface{}{},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendRawCommand(probeID string, commandType string, params map[string]interface{}) error {
	cmd := Command{
		Command:   commandType,
		CommandID: fmt.Sprintf("cmd-%d", time.Now().Unix()),
		Params:    params,
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) BroadcastCommand(commandType string, params map[string]interface{}) error {
	cmd := Command{
		Command:   commandType,
		CommandID: fmt.Sprintf("broadcast-cmd-%d", time.Now().Unix()),
		Params:    params,
		Timestamp: time.Now().Unix(),
	}

	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	topic := "campus/probes/broadcast/cmd"
	token := c.client.Publish(topic, 1, false, payload)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("failed to publish broadcast command: %w", token.Error())
	}

	c.log.Info("Broadcast command sent: %s", commandType)
	return nil
}

func (c *Client) publishCommand(probeID string, cmd Command) error {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	topic := fmt.Sprintf("campus/probes/%s/cmd", probeID)
	token := c.client.Publish(topic, 1, false, payload)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("failed to publish command: %w", token.Error())
	}

	c.log.Info("Command sent to %s: %s (ID: %s)", probeID, cmd.Command, cmd.CommandID)
	return nil
}
