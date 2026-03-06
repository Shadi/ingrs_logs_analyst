package parser

import (
	"fmt"
	"strings"

	"github.com/ip2location/ip2location-go/v9"
	"github.com/rs/zerolog/log"
)

type LogEntry struct {
	Time           string  `json:"time"`
	RemoteAddr     string  `json:"remote_addr"`
	XForwardedFor  string  `json:"x_forwarded_for"`
	RequestID      string  `json:"request_id"`
	Status         int     `json:"status"`
	VHost          string  `json:"vhost"`
	Path           string  `json:"path"`
	Method         string  `json:"method"`
	HTTPUserAgent  string  `json:"http_user_agent"`
	RequestTime    float64 `json:"request_time"`
	Duration       float64 `json:"duration"`
	BytesSent      int     `json:"bytes_sent"`
	RequestLength  int     `json:"request_length"`
	RequestProto   string  `json:"request_proto"`
	RequestQuery   string  `json:"request_query"`
	HTTPReferrer   string  `json:"http_referrer"`
	XHttpCfConnect string  `json:"x_http_cf_connecting_ip"`
	RemoteUser     string  `json:"remote_user"`
}

type IPLocation struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
	ISP     string `json:"isp"`
	Domain  string `json:"domain"`
}

type LogSource interface {
	ReadLogs() ([]LogEntry, error)
}

type LogSourceConfig struct {
	Source     string
	File       string
	Namespace  string
	Deployment string
	Kubeconfig string
}

func NewLogSource(cfg LogSourceConfig) (LogSource, error) {
	switch cfg.Source {
	case "file":
		if cfg.File == "" {
			return nil, fmt.Errorf("file path is required when using file source")
		}
		return &FileLogSource{Filename: cfg.File}, nil
	case "k8s":
		return &K8sLogSource{Namespace: cfg.Namespace, Deployment: cfg.Deployment, Kubeconfig: cfg.Kubeconfig}, nil
	default:
		return nil, fmt.Errorf("unknown source: %s", cfg.Source)
	}
}

type LogData struct {
	Entries []LogEntry
	ipDB    *ip2location.DB
}

func NewLogData() *LogData {
	return &LogData{}
}

func (ld *LogData) LoadFrom(source LogSource) error {
	entries, err := source.ReadLogs()
	if err != nil {
		return err
	}
	ld.Entries = entries
	return nil
}

func (ld *LogData) LoadIPDB(filename string) error {
	db, err := ip2location.OpenDB(filename)
	if err != nil {
		return err
	}
	ld.ipDB = db
	log.Info().Msg("loaded IP2Location DB")
	return nil
}

func (ld *LogData) CloseIPDB() {
	if ld.ipDB != nil {
		ld.ipDB.Close()
	}
}

func (ld *LogData) GetIPLocation(ip string) IPLocation {
	loc := IPLocation{IP: ip, Country: "Unknown", Region: "Unknown", City: "Unknown", ISP: "Unknown", Domain: "Unknown"}
	if ld.ipDB == nil || ip == "" {
		return loc
	}

	res, err := ld.ipDB.Get_all(ip)
	if err != nil {
		return loc
	}

	if res.Country_long != "" && res.Country_long != "-" && res.Country_long != "INVALID_IP_ADDRESS" {
		loc.Country = res.Country_long
	}
	if res.Region != "" && res.Region != "-" {
		loc.Region = res.Region
	}
	if res.City != "" && res.City != "-" {
		loc.City = res.City
	}
	if res.Isp != "" && res.Isp != "-" {
		loc.ISP = res.Isp
	}
	if res.Domain != "" && res.Domain != "-" {
		loc.Domain = res.Domain
	}
	return loc
}

func ExtractIP(entry LogEntry) string {
	ip := entry.XForwardedFor
	if ip == "" {
		ip = entry.RemoteAddr
	}
	if strings.Contains(ip, ",") {
		ip = strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	return ip
}
