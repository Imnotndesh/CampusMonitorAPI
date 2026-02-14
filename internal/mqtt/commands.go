package mqtt

import (
	"encoding/json"
	"fmt"
	"time"

	_ "CampusMonitorAPI/internal/logger"
)

type Command struct {
	Command   string                 `json:"command"`
	CommandID string                 `json:"command_id,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Timestamp int64                  `json:"timestamp,omitempty"`
}

func (c *Client) SendDeepScan(probeID string, cmdID int, duration int) error {
	cmd := Command{
		Command:   "deep_scan",
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload: map[string]interface{}{
			"duration": duration,
		},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendConfigUpdate(probeID string, cmdID int, config map[string]interface{}) error {
	cmd := Command{
		Command:   "config_update",
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload:   config,
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendGetConfig(probeID string, cmdID int) error {
	cmd := Command{
		Command:   "get_config",
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload:   map[string]interface{}{},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendSetWifi(probeID string, cmdID int, ssid, password string) error {
	cmd := Command{
		Command:   "set_wifi",
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload: map[string]interface{}{
			"ssid":     ssid,
			"password": password,
		},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendSetMqtt(probeID string, cmdID int, broker string, port int, user, password string) error {
	cmd := Command{
		Command:   "set_mqtt",
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload: map[string]interface{}{
			"broker":   broker,
			"port":     port,
			"user":     user,
			"password": password,
		},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendRenameProbe(probeID string, cmdID int, newID string) error {
	cmd := Command{
		Command:   "rename_probe",
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload: map[string]interface{}{
			"new_id": newID,
		},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendRestart(probeID string, cmdID int, delay int) error {
	cmd := Command{
		Command:   "restart",
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload: map[string]interface{}{
			"delay": delay,
		},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendOTAUpdate(probeID string, cmdID int, url string) error {
	cmd := Command{
		Command:   "ota_update",
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload: map[string]interface{}{
			"url": url,
		},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendFactoryReset(probeID string, cmdID int) error {
	cmd := Command{
		Command:   "factory_reset",
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload:   map[string]interface{}{},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendPing(probeID string, cmdID int) error {
	cmd := Command{
		Command:   "ping",
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload:   map[string]interface{}{},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendGetStatus(probeID string, cmdID int) error {
	cmd := Command{
		Command:   "get_status",
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload:   map[string]interface{}{},
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) SendRawCommand(probeID string, cmdID int, commandType string, params map[string]interface{}) error {
	cmd := Command{
		Command:   commandType,
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload:   params,
		Timestamp: time.Now().Unix(),
	}

	return c.publishCommand(probeID, cmd)
}

func (c *Client) BroadcastCommand(cmdID int, commandType string, params map[string]interface{}) error {
	cmd := Command{
		Command:   commandType,
		CommandID: fmt.Sprintf("%d", cmdID),
		Payload:   params,
		Timestamp: time.Now().Unix(),
	}

	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	topic := "campus/probes/broadcast/command"
	token := c.client.Publish(topic, 1, false, payload)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("failed to publish broadcast command: %w", token.Error())
	}

	c.log.Info("Broadcast command sent: %s (ID: %d)", commandType, cmdID)
	return nil
}

func (c *Client) publishCommand(probeID string, cmd Command) error {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}
	topic := fmt.Sprintf("campus/probes/%s/command", probeID)

	c.log.Info("Publishing to topic: %s", topic)
	c.log.Info("Payload: %s", string(payload))

	token := c.client.Publish(topic, 1, false, payload)
	token.Wait()

	if token.Error() != nil {
		return fmt.Errorf("failed to publish command: %w", token.Error())
	}

	c.log.Info("Command sent to %s: %s (ID: %s)", probeID, cmd.Command, cmd.CommandID)
	return nil
}
