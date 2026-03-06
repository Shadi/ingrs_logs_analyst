package parser

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestNewLogSource_File(t *testing.T) {
	src, err := NewLogSource(LogSourceConfig{Source: "file", File: "test.log"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := src.(*FileLogSource)
	if !ok {
		t.Fatalf("expected *FileLogSource, got %T", src)
	}
	if fs.Filename != "test.log" {
		t.Errorf("expected filename test.log, got %s", fs.Filename)
	}
}

func TestNewLogSource_FileRequiresPath(t *testing.T) {
	_, err := NewLogSource(LogSourceConfig{Source: "file", File: ""})
	if err == nil {
		t.Fatal("expected error for empty file path")
	}
}

func TestNewLogSource_K8s(t *testing.T) {
	src, err := NewLogSource(LogSourceConfig{Source: "k8s", Namespace: "ns", Deployment: "dep"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ks, ok := src.(*K8sLogSource)
	if !ok {
		t.Fatalf("expected *K8sLogSource, got %T", src)
	}
	if ks.Namespace != "ns" || ks.Deployment != "dep" {
		t.Errorf("expected ns/dep, got %s/%s", ks.Namespace, ks.Deployment)
	}
}

func TestNewLogSource_UnknownSource(t *testing.T) {
	_, err := NewLogSource(LogSourceConfig{Source: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown source")
	}
}

type mockLogSource struct {
	entries []LogEntry
	err     error
}

func (m *mockLogSource) ReadLogs() ([]LogEntry, error) {
	return m.entries, m.err
}

func TestLoadFrom_Success(t *testing.T) {
	entries := []LogEntry{
		{RemoteAddr: "1.2.3.4", Path: "/"},
		{RemoteAddr: "5.6.7.8", Path: "/api"},
	}
	ld := NewLogData()
	err := ld.LoadFrom(&mockLogSource{entries: entries})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ld.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(ld.Entries))
	}
}

func TestLoadFrom_Error(t *testing.T) {
	ld := NewLogData()
	err := ld.LoadFrom(&mockLogSource{err: fmt.Errorf("read error")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetIPLocation_NilDB(t *testing.T) {
	ld := NewLogData()
	loc := ld.GetIPLocation("1.2.3.4")
	if loc.Country != "Unknown" {
		t.Errorf("expected Unknown country, got %s", loc.Country)
	}
	if loc.IP != "1.2.3.4" {
		t.Errorf("expected IP 1.2.3.4, got %s", loc.IP)
	}
}

func TestGetIPLocation_EmptyIP(t *testing.T) {
	ld := NewLogData()
	loc := ld.GetIPLocation("")
	if loc.IP != "" {
		t.Errorf("expected empty IP, got %s", loc.IP)
	}
	if loc.Country != "Unknown" {
		t.Errorf("expected Unknown, got %s", loc.Country)
	}
}

func TestExtractIP_XForwardedFor(t *testing.T) {
	entry := LogEntry{XForwardedFor: "10.0.0.1", RemoteAddr: "192.168.1.1"}
	if ip := ExtractIP(entry); ip != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", ip)
	}
}

func TestExtractIP_FallbackToRemoteAddr(t *testing.T) {
	entry := LogEntry{RemoteAddr: "192.168.1.1"}
	if ip := ExtractIP(entry); ip != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %s", ip)
	}
}

func TestExtractIP_CommaSeparated(t *testing.T) {
	entry := LogEntry{XForwardedFor: "10.0.0.1, 10.0.0.2, 10.0.0.3"}
	if ip := ExtractIP(entry); ip != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %s", ip)
	}
}

func TestExtractIP_Empty(t *testing.T) {
	entry := LogEntry{}
	if ip := ExtractIP(entry); ip != "" {
		t.Errorf("expected empty string, got %s", ip)
	}
}

func TestNewLogData(t *testing.T) {
	ld := NewLogData()
	if ld == nil {
		t.Fatal("expected non-nil LogData")
	}
	if len(ld.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(ld.Entries))
	}
}

func TestCloseIPDB_NilDB(t *testing.T) {
	ld := NewLogData()
	ld.CloseIPDB() // should not panic
}

const sampleLogLine = `{"time": "2026-03-06T19:03:24+00:00", "remote_addr": "", "x_forwarded_for": "46.185.164.94", "x_http_cf_connecting_ip":"", "request_id": "4d95b1b4b90ba5b0dcc7cee5b27fb32c", "remote_user": "", "bytes_sent": 347, "request_time": 0.010, "status": 200, "vhost": "webui.s13h.xyz", "request_proto": "HTTP/2.0", "path": "/_app/version.json", "request_query": "", "request_length": 589, "duration": 0.010,"method": "GET", "http_referrer": "https://webui.s13h.xyz/c/b3151387-321e-45d0-978f-78c8f0ab5e6b", "http_user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36" }`

func TestParseFullLogLine(t *testing.T) {
	var entry LogEntry
	if err := json.Unmarshal([]byte(sampleLogLine), &entry); err != nil {
		t.Fatalf("failed to parse log line: %v", err)
	}

	checks := map[string]string{
		"Time":          "2026-03-06T19:03:24+00:00",
		"RemoteAddr":    "",
		"XForwardedFor": "46.185.164.94",
		"RequestID":     "4d95b1b4b90ba5b0dcc7cee5b27fb32c",
		"VHost":         "webui.s13h.xyz",
		"Path":          "/_app/version.json",
		"Method":        "GET",
		"RequestProto":  "HTTP/2.0",
		"RemoteUser":    "",
		"RequestQuery":  "",
		"HTTPReferrer":  "https://webui.s13h.xyz/c/b3151387-321e-45d0-978f-78c8f0ab5e6b",
		"HTTPUserAgent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36",
	}
	vals := map[string]string{
		"Time": entry.Time, "RemoteAddr": entry.RemoteAddr, "XForwardedFor": entry.XForwardedFor,
		"RequestID": entry.RequestID, "VHost": entry.VHost, "Path": entry.Path,
		"Method": entry.Method, "RequestProto": entry.RequestProto, "RemoteUser": entry.RemoteUser,
		"RequestQuery": entry.RequestQuery, "HTTPReferrer": entry.HTTPReferrer, "HTTPUserAgent": entry.HTTPUserAgent,
	}
	for field, expected := range checks {
		if vals[field] != expected {
			t.Errorf("%s: expected %q, got %q", field, expected, vals[field])
		}
	}

	if entry.Status != 200 {
		t.Errorf("Status: expected 200, got %d", entry.Status)
	}
	if entry.BytesSent != 347 {
		t.Errorf("BytesSent: expected 347, got %d", entry.BytesSent)
	}
	if entry.RequestLength != 589 {
		t.Errorf("RequestLength: expected 589, got %d", entry.RequestLength)
	}
	if entry.RequestTime != 0.010 {
		t.Errorf("RequestTime: expected 0.010, got %f", entry.RequestTime)
	}
	if entry.Duration != 0.010 {
		t.Errorf("Duration: expected 0.010, got %f", entry.Duration)
	}
}

func TestExtractIP_FullLogLine(t *testing.T) {
	var entry LogEntry
	if err := json.Unmarshal([]byte(sampleLogLine), &entry); err != nil {
		t.Fatalf("failed to parse log line: %v", err)
	}
	if ip := ExtractIP(entry); ip != "46.185.164.94" {
		t.Errorf("expected 46.185.164.94, got %s", ip)
	}
}

func TestExtractIP_FullLogLine_EmptyForwardedFor(t *testing.T) {
	// Same line but with empty x_forwarded_for and a remote_addr set
	line := `{"remote_addr": "192.168.1.1", "x_forwarded_for": "", "path": "/test"}`
	var entry LogEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if ip := ExtractIP(entry); ip != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %s", ip)
	}
}
