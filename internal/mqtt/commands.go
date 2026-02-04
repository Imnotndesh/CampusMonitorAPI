package mqtt

import (
	"encoding/json"
	"fmt"
)

type CommandType string

const (
	CommandDeepScan     CommandType = "deep_scan"
	CommandUpdateConfig CommandType = "update_config"
	CommandRestart      CommandType = "restart"
	CommandOTAUpdate    CommandType = "ota_update"
)

type Command struct {
	Type    CommandType            `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

type DeepScanCommand struct {
	Duration int `json:"duration,omitempty"`
}

type ConfigUpdateCommand struct {
	ReportInterval int    `json:"report_interval,omitempty"`
	MQTTServer     string `json:"mqtt_server,omitempty"`
	MQTTPort       int    `json:"mqtt_port,omitempty"`
}

type OTAUpdateCommand struct {
	URL     string `json:"url"`
	Version string `json:"version"`
}

func (c *Client) SendDeepScan(probeID string, duration int) error {
	topic := fmt.Sprintf("campus/probes/%s/cmd", probeID)

	cmd := DeepScanCommand{
		Duration: duration,
	}

	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal deep scan command: %w", err)
	}

	c.log.Info("Sending deep scan command to probe: %s (duration: %ds)", probeID, duration)

	if err := c.Publish(topic, payload); err != nil {
		return fmt.Errorf("failed to send deep scan command: %w", err)
	}

	c.log.Info("Deep scan command sent successfully to probe: %s", probeID)
	return nil
}

func (c *Client) SendConfigUpdate(probeID string, config ConfigUpdateCommand) error {
	topic := fmt.Sprintf("campus/probes/%s/cmd", probeID)

	c.log.Info("Sending config update command to probe: %s", probeID)
	c.log.Debug("Config update payload: %+v", config)

	if err := c.PublishJSON(topic, config); err != nil {
		return fmt.Errorf("failed to send config update: %w", err)
	}

	c.log.Info("Config update command sent successfully to probe: %s", probeID)
	return nil
}

func (c *Client) SendOTAUpdate(probeID string, url, version string) error {
	topic := fmt.Sprintf("campus/probes/%s/cmd", probeID)

	cmd := OTAUpdateCommand{
		URL:     url,
		Version: version,
	}

	c.log.Info("Sending OTA update command to probe: %s (version: %s)", probeID, version)

	if err := c.PublishJSON(topic, cmd); err != nil {
		return fmt.Errorf("failed to send OTA update command: %w", err)
	}

	c.log.Info("OTA update command sent successfully to probe: %s", probeID)
	return nil
}

func (c *Client) SendRestart(probeID string) error {
	topic := fmt.Sprintf("campus/probes/%s/cmd", probeID)

	cmd := Command{
		Type: CommandRestart,
	}

	c.log.Warn("Sending restart command to probe: %s", probeID)

	if err := c.PublishJSON(topic, cmd); err != nil {
		return fmt.Errorf("failed to send restart command: %w", err)
	}

	c.log.Info("Restart command sent successfully to probe: %s", probeID)
	return nil
}

func (c *Client) SendRawCommand(probeID string, payload []byte) error {
	topic := fmt.Sprintf("campus/probes/%s/cmd", probeID)

	c.log.Debug("Sending raw command to probe: %s (size: %d bytes)", probeID, len(payload))

	if err := c.Publish(topic, payload); err != nil {
		return fmt.Errorf("failed to send raw command: %w", err)
	}

	c.log.Debug("Raw command sent successfully to probe: %s", probeID)
	return nil
}

func (c *Client) BroadcastCommand(cmd Command) error {
	topic := "campus/probes/broadcast/cmd"

	c.log.Warn("Broadcasting command to all probes: %s", cmd.Type)

	if err := c.PublishJSON(topic, cmd); err != nil {
		return fmt.Errorf("failed to broadcast command: %w", err)
	}

	c.log.Info("Broadcast command sent successfully: %s", cmd.Type)
	return nil
}
