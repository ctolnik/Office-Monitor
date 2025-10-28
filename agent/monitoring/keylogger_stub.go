// +build !windows

package monitoring

import (
	"fmt"
)

type Keylogger struct {
	enabled bool
}

func NewKeylogger(serverURL, computerName, username string, monitoredProcesses []string, bufferSizeChars, sendIntervalMin int) *Keylogger {
	return &Keylogger{
		enabled: false,
	}
}

func (k *Keylogger) Start() error {
	return fmt.Errorf("keylogger not supported on non-Windows platforms")
}

func (k *Keylogger) Stop() {
}
