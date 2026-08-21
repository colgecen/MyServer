package protocol_test

import (
	"encoding/json"
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/protocol"
)

func TestEnvelope_Validate(t *testing.T) {
	cases := []struct {
		name    string
		env     protocol.Envelope
		wantErr bool
	}{
		{"missing id", protocol.Envelope{Type: protocol.TypeRequest, Action: protocol.ActionChat}, true},
		{"missing type", protocol.Envelope{ID: "1"}, true},
		{"request missing action", protocol.Envelope{ID: "1", Type: protocol.TypeRequest}, true},
		{"ok response", protocol.Envelope{ID: "1", Type: protocol.TypeResponse}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.env.Validate(); (err != nil) != tc.wantErr {
				t.Fatalf("Validate err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestEnvelope_RoundTrip(t *testing.T) {
	req, err := protocol.NewRequest("abc", protocol.ActionExecCommand, protocol.ExecRequest{
		Command:    "ls",
		Shell:      "bash",
		Workdir:    "/tmp",
		TimeoutSec: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var back protocol.Envelope
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	var payload protocol.ExecRequest
	if err := back.Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Command != "ls" || payload.Workdir != "/tmp" {
		t.Fatalf("payload roundtrip mismatch: %+v", payload)
	}
}

func TestNewError(t *testing.T) {
	e := protocol.NewError("id-1", "E_BAD", "boom")
	if e.Type != protocol.TypeError || e.Error == nil {
		t.Fatal("error envelope malformed")
	}
	if e.Error.Code != "E_BAD" || e.Error.Message != "boom" {
		t.Fatalf("unexpected error payload: %+v", e.Error)
	}
}
