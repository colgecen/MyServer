package security

import (
	"encoding/json"
	"time"
)

// ComplianceEntry is SIEM-ready export format.
type ComplianceEntry struct {
	Timestamp time.Time `json:"@timestamp"`
	Command   string    `json:"command"`
	User      string    `json:"user"`
	Level     string    `json:"level"`
	Decision  string    `json:"decision"`
	Signature string    `json:"signature,omitempty"`
}

func ExportJSON(entries []ComplianceEntry) ([]byte, error) {
	return json.MarshalIndent(entries, "", "  ")
}

func ExportNDJSON(entries []ComplianceEntry) []byte {
	var out []byte
	for _, e := range entries {
		b, _ := json.Marshal(e)
		out = append(out, b...)
		out = append(out, '\n')
	}
	return out
}
