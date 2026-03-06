package parser

import (
	"encoding/json"
	"fmt"
	"testing"
)

func newTestLogData(t *testing.T) *LogData {
	t.Helper()
	ld, err := NewLogData()
	if err != nil {
		t.Fatalf("failed to create LogData: %v", err)
	}
	t.Cleanup(func() { ld.Close() })
	return ld
}

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

func (m *mockLogSource) ReadLogs(fn func(LogEntry) error) error {
	if m.err != nil {
		return m.err
	}
	for _, e := range m.entries {
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

func TestLoadFrom_Success(t *testing.T) {
	entries := []LogEntry{
		{RemoteAddr: "1.2.3.4", Path: "/", Status: 200},
		{RemoteAddr: "5.6.7.8", Path: "/api", Status: 200},
	}
	ld := newTestLogData(t)
	count, err := ld.LoadFrom(&mockLogSource{entries: entries})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 entries, got %d", count)
	}
	if ld.TotalRequests() != 2 {
		t.Errorf("expected TotalRequests 2, got %d", ld.TotalRequests())
	}
}

func TestLoadFrom_Error(t *testing.T) {
	ld := newTestLogData(t)
	_, err := ld.LoadFrom(&mockLogSource{err: fmt.Errorf("read error")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetIPLocation_NilDB(t *testing.T) {
	ld := newTestLogData(t)
	loc := ld.GetIPLocation("1.2.3.4")
	if loc.Country != "Unknown" {
		t.Errorf("expected Unknown country, got %s", loc.Country)
	}
	if loc.IP != "1.2.3.4" {
		t.Errorf("expected IP 1.2.3.4, got %s", loc.IP)
	}
}

func TestGetIPLocation_EmptyIP(t *testing.T) {
	ld := newTestLogData(t)
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
	ld := newTestLogData(t)
	if ld.TotalRequests() != 0 {
		t.Errorf("expected 0 entries, got %d", ld.TotalRequests())
	}
}

func TestClose_NilDB(t *testing.T) {
	ld := newTestLogData(t)
	ld.Close() // should not panic
}

func TestTopSites(t *testing.T) {
	ld := newTestLogData(t)
	entries := []LogEntry{
		{VHost: "a.com", Path: "/", Status: 200, RemoteAddr: "1.1.1.1"},
		{VHost: "a.com", Path: "/x", Status: 200, RemoteAddr: "1.1.1.1"},
		{VHost: "b.com", Path: "/", Status: 200, RemoteAddr: "2.2.2.2"},
	}
	ld.LoadFrom(&mockLogSource{entries: entries})
	sites := ld.TopSites()
	if len(sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(sites))
	}
	if sites[0].Site != "a.com" || sites[0].Count != 2 {
		t.Errorf("expected a.com:2, got %s:%d", sites[0].Site, sites[0].Count)
	}
}

func TestSitePaths(t *testing.T) {
	ld := newTestLogData(t)
	entries := []LogEntry{
		{VHost: "a.com", Path: "/foo", Status: 200, RemoteAddr: "1.1.1.1"},
		{VHost: "a.com", Path: "/foo", Status: 200, RemoteAddr: "1.1.1.1"},
		{VHost: "a.com", Path: "/bar", Status: 200, RemoteAddr: "1.1.1.1"},
	}
	ld.LoadFrom(&mockLogSource{entries: entries})
	paths := ld.SitePaths("a.com")
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0].Path != "/foo" || paths[0].Count != 2 {
		t.Errorf("expected /foo:2, got %s:%d", paths[0].Path, paths[0].Count)
	}
}

func TestPathIPs(t *testing.T) {
	ld := newTestLogData(t)
	entries := []LogEntry{
		{VHost: "a.com", Path: "/foo", Status: 200, XForwardedFor: "10.0.0.1"},
		{VHost: "a.com", Path: "/foo", Status: 200, XForwardedFor: "10.0.0.1"},
		{VHost: "a.com", Path: "/foo", Status: 200, XForwardedFor: "10.0.0.2"},
	}
	ld.LoadFrom(&mockLogSource{entries: entries})
	ips := ld.PathIPs("a.com", "/foo")
	if len(ips) != 2 {
		t.Fatalf("expected 2 IPs, got %d", len(ips))
	}
	if ips[0].IP != "10.0.0.1" || ips[0].Count != 2 {
		t.Errorf("expected 10.0.0.1:2, got %s:%d", ips[0].IP, ips[0].Count)
	}
}

func TestIPPaths(t *testing.T) {
	ld := newTestLogData(t)
	entries := []LogEntry{
		{VHost: "a.com", Path: "/a", Status: 200, RemoteAddr: "1.1.1.1", Time: "2026-01-01T00:00:00Z"},
		{VHost: "a.com", Path: "/a", Status: 200, RemoteAddr: "1.1.1.1", Time: "2026-01-02T00:00:00Z"},
		{VHost: "a.com", Path: "/b", Status: 200, RemoteAddr: "1.1.1.1", Time: "2026-01-01T00:00:00Z"},
	}
	ld.LoadFrom(&mockLogSource{entries: entries})
	paths := ld.IPPaths("1.1.1.1")
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0].Path != "/a" || paths[0].Count != 2 {
		t.Errorf("expected /a:2, got %s:%d", paths[0].Path, paths[0].Count)
	}
	if paths[0].Time != "2026-01-02T00:00:00Z" {
		t.Errorf("expected last time 2026-01-02, got %s", paths[0].Time)
	}
}

func TestStatusIPs(t *testing.T) {
	ld := newTestLogData(t)
	entries := []LogEntry{
		{VHost: "a.com", Path: "/", Status: 404, RemoteAddr: "1.1.1.1"},
		{VHost: "a.com", Path: "/", Status: 403, RemoteAddr: "1.1.1.1"},
		{VHost: "a.com", Path: "/", Status: 200, RemoteAddr: "2.2.2.2"},
	}
	ld.LoadFrom(&mockLogSource{entries: entries})
	ips := ld.StatusIPs(400, 499)
	if len(ips) != 1 {
		t.Fatalf("expected 1 IP, got %d", len(ips))
	}
	if ips[0].IP != "1.1.1.1" || ips[0].Count != 2 {
		t.Errorf("expected 1.1.1.1:2, got %s:%d", ips[0].IP, ips[0].Count)
	}
}

func TestStatusIPDetails(t *testing.T) {
	ld := newTestLogData(t)
	entries := []LogEntry{
		{VHost: "a.com", Path: "/a", Status: 500, RemoteAddr: "1.1.1.1", Time: "2026-01-01T00:00:00Z"},
		{VHost: "a.com", Path: "/b", Status: 502, RemoteAddr: "1.1.1.1", Time: "2026-01-02T00:00:00Z"},
		{VHost: "a.com", Path: "/c", Status: 200, RemoteAddr: "1.1.1.1", Time: "2026-01-03T00:00:00Z"},
	}
	ld.LoadFrom(&mockLogSource{entries: entries})
	details := ld.StatusIPDetails("1.1.1.1", 500, 599)
	if len(details) != 2 {
		t.Fatalf("expected 2 results, got %d", len(details))
	}
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
	line := `{"remote_addr": "192.168.1.1", "x_forwarded_for": "", "path": "/test"}`
	var entry LogEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if ip := ExtractIP(entry); ip != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %s", ip)
	}
}

func TestFullLogLineInDB(t *testing.T) {
	var entry LogEntry
	if err := json.Unmarshal([]byte(sampleLogLine), &entry); err != nil {
		t.Fatalf("failed to parse log line: %v", err)
	}

	ld := newTestLogData(t)
	count, err := ld.LoadFrom(&mockLogSource{entries: []LogEntry{entry}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	sites := ld.TopSites()
	if len(sites) != 1 || sites[0].Site != "webui.s13h.xyz" {
		t.Errorf("expected webui.s13h.xyz, got %v", sites)
	}

	paths := ld.SitePaths("webui.s13h.xyz")
	if len(paths) != 1 || paths[0].Path != "/_app/version.json" {
		t.Errorf("expected /_app/version.json, got %v", paths)
	}

	ips := ld.PathIPs("webui.s13h.xyz", "/_app/version.json")
	if len(ips) != 1 || ips[0].IP != "46.185.164.94" {
		t.Errorf("expected 46.185.164.94, got %v", ips)
	}
}
