// Package types defines Go types for the V3SP3R library.
// Go library for the V3SP3R AI Brain for Flipper Zero. Provides natural language control of Flipper Zero hacking tool via AI-powered command generation and BLE communication.
package types

import (
	"fmt"
	"strings"
)

// CommandRequest represents commandrequest data.
type CommandRequest struct {
	Context         string
	SafetyLevel     string
	ConfirmRequired bool
	NaturalLanguage string
}

// Validate checks that the CommandRequest is valid.
func (o *CommandRequest) Validate() error {
	if strings.TrimSpace(o.NaturalLanguage) == "" {
		return fmt.Errorf("naturallanguage is required")
	}
	return nil
}

// CommandResult represents commandresult data.
type CommandResult struct {
	SafetyWarning        string
	Description          string
	Command              string
	SubCommands          []SubCommand
	RequiresConfirmation bool
}

// Validate checks that the CommandResult is valid.
func (o *CommandResult) Validate() error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

// SubCommand represents subcommand data.
type SubCommand struct {
	Action     string
	Parameters map[string]string
	Target     string
}

// DeviceStatus represents devicestatus data.
type DeviceStatus struct {
	StorageUsed     float64
	BatteryLevel    int
	FirmwareVersion string
	Connected       bool
	CurrentApp      string
}

// BLEConfig represents bleconfig data.
type BLEConfig struct {
	CharacteristicUUID string
	Timeout            int
	DeviceAddress      string
	ServiceUUID        string
}

// HistoryEntry represents historyentry data.
type HistoryEntry struct {
	Command   string
	Request   string
	Timestamp string
	Success   bool
}
