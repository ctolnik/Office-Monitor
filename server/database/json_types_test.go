package database

import (
	"encoding/json"
	"testing"
)

func TestUSBTargetFlag_Unmarshal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    USBTargetFlag
		wantErr bool
	}{
		{name: "json_bool_true", input: `true`, want: 1},
		{name: "json_bool_false", input: `false`, want: 0},
		{name: "json_number_one", input: `1`, want: 1},
		{name: "json_number_zero", input: `0`, want: 0},
		{name: "json_number_non_zero", input: `255`, want: 1},
		{name: "json_negative_number", input: `-1`, want: 1},
		{name: "json_null", input: `null`, want: 0},
		{name: "json_whitespace_around_bool", input: ` true `, want: 1},
		{name: "json_invalid_string", input: `"yes"`, wantErr: true},
		{name: "json_invalid_float", input: `1.5`, wantErr: true},
		{name: "json_invalid_object", input: `{}`, wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got USBTargetFlag
			err := got.UnmarshalJSON([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got flag=%d", tc.input, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("input %q: want flag=%d, got %d", tc.input, tc.want, got)
			}
		})
	}
}

func TestUSBTargetFlag_MarshalJSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input USBTargetFlag
		want  string
	}{
		{name: "zero_marshals_false", input: 0, want: `false`},
		{name: "one_marshals_true", input: 1, want: `true`},
		{name: "non_zero_marshals_true", input: 42, want: `true`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := json.Marshal(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("want %q, got %q", tc.want, string(got))
			}
		})
	}
}

// TestFileCopyEvent_UnmarshalBackwardsCompat validates that both the current
// agent payload (bool) and the historical/storage payload (uint8 number) are
// accepted by the server when decoding a file event. This is the regression
// test for the `Failed to unmarshal file event` runtime error caused by the
// bool/uint8 mismatch between the agent and the server.
func TestFileCopyEvent_UnmarshalBackwardsCompat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
		want    USBTargetFlag
	}{
		{
			name: "agent_bool_true",
			payload: `{
                "computer_name":"ACC-21",
                "username":"n.kopylova",
                "source_path":"C:\\a.txt",
                "destination_path":"E:\\a.txt",
                "file_size":123,
                "file_count":1,
                "operation_type":"copy",
                "is_usb_target":true
            }`,
			want: 1,
		},
		{
			name: "agent_bool_false",
			payload: `{
                "computer_name":"ACC-21",
                "username":"n.kopylova",
                "source_path":"C:\\a.txt",
                "destination_path":"C:\\b.txt",
                "file_size":10,
                "file_count":1,
                "operation_type":"copy",
                "is_usb_target":false
            }`,
			want: 0,
		},
		{
			name: "legacy_numeric_one",
			payload: `{
                "computer_name":"ACC-21",
                "username":"n.kopylova",
                "source_path":"C:\\a.txt",
                "destination_path":"E:\\a.txt",
                "file_size":123,
                "file_count":1,
                "operation_type":"copy",
                "is_usb_target":1
            }`,
			want: 1,
		},
		{
			name: "legacy_numeric_zero",
			payload: `{
                "computer_name":"ACC-21",
                "username":"n.kopylova",
                "source_path":"C:\\a.txt",
                "destination_path":"C:\\b.txt",
                "file_size":10,
                "file_count":1,
                "operation_type":"copy",
                "is_usb_target":0
            }`,
			want: 0,
		},
		{
			name: "missing_field_defaults_to_zero",
			payload: `{
                "computer_name":"ACC-21",
                "username":"n.kopylova",
                "source_path":"C:\\a.txt",
                "destination_path":"C:\\b.txt",
                "file_size":10,
                "file_count":1,
                "operation_type":"copy"
            }`,
			want: 0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got FileCopyEvent
			if err := json.Unmarshal([]byte(tc.payload), &got); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if got.IsUSBTarget != tc.want {
				t.Fatalf("want IsUSBTarget=%d, got %d", tc.want, got.IsUSBTarget)
			}
		})
	}
}
