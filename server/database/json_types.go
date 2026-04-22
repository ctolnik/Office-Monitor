package database

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// USBTargetFlag represents the `is_usb_target` attribute for file copy
// events. The agent historically serializes this field as a JSON boolean
// while the ClickHouse column is declared as UInt8. This type accepts both
// representations on unmarshal and always emits a JSON boolean on marshal
// so downstream consumers (frontend, reports) observe a stable contract.
type USBTargetFlag uint8

// Bool returns the boolean view of the flag.
func (f USBTargetFlag) Bool() bool {
	return f != 0
}

// MarshalJSON emits the flag as a JSON boolean to keep a stable contract
// for API consumers regardless of the underlying storage type.
func (f USBTargetFlag) MarshalJSON() ([]byte, error) {
	if f.Bool() {
		return []byte("true"), nil
	}
	return []byte("false"), nil
}

// UnmarshalJSON accepts either a JSON boolean (`true`/`false`) or a numeric
// representation (`0`/`1`/...) for backwards compatibility with mixed agent
// versions. Any non-zero numeric value is normalized to 1 to match the
// boolean semantics of the flag.
func (f *USBTargetFlag) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	switch {
	case len(trimmed) == 0, bytes.Equal(trimmed, []byte("null")):
		*f = 0
		return nil
	case bytes.Equal(trimmed, []byte("true")):
		*f = 1
		return nil
	case bytes.Equal(trimmed, []byte("false")):
		*f = 0
		return nil
	}

	// Accept any integer shape; normalize non-zero values to 1.
	if n, err := strconv.ParseInt(string(trimmed), 10, 64); err == nil {
		if n != 0 {
			*f = 1
		} else {
			*f = 0
		}
		return nil
	}

	return fmt.Errorf("is_usb_target: unsupported JSON value %q", string(trimmed))
}

var (
	_ json.Marshaler   = USBTargetFlag(0)
	_ json.Unmarshaler = (*USBTargetFlag)(nil)
)
