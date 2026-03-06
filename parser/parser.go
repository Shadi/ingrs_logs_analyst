package parser

import (
	"database/sql"
	"fmt"
	"os"
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

type SiteCount struct {
	Site  string `json:"site"`
	Count int    `json:"count"`
}

type PathCount struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

type IPCount struct {
	IP       string     `json:"ip"`
	Count    int        `json:"count"`
	Location IPLocation `json:"location"`
}

type PathInfo struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
	Time  string `json:"last_time"`
}

type RequestInfo struct {
	Path   string `json:"path"`
	Status int    `json:"status"`
	Time   string `json:"time"`
}

type LogSource interface {
	ReadLogs(fn func(LogEntry) error) error
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
	db     *sql.DB
	dbPath string
	ipDB   *ip2location.DB
}

func NewLogData() (*LogData, error) {
	db, dbPath, err := openDB()
	if err != nil {
		return nil, err
	}
	return &LogData{db: db, dbPath: dbPath}, nil
}

func (ld *LogData) LoadFrom(source LogSource) (int, error) {
	tx, err := ld.db.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	count := 0
	err = source.ReadLogs(func(e LogEntry) error {
		if err := insertEntry(stmt, e); err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	return count, tx.Commit()
}

func (ld *LogData) Close() {
	if ld.ipDB != nil {
		ld.ipDB.Close()
	}
	if ld.db != nil {
		ld.db.Close()
	}
	if ld.dbPath != "" {
		os.Remove(ld.dbPath)
	}
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

func (ld *LogData) TotalRequests() int {
	var count int
	ld.db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&count)
	return count
}

func (ld *LogData) TopSites() []SiteCount {
	rows, err := ld.db.Query("SELECT vhost, COUNT(*) as c FROM logs GROUP BY vhost ORDER BY c DESC")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []SiteCount
	for rows.Next() {
		var s SiteCount
		rows.Scan(&s.Site, &s.Count)
		results = append(results, s)
	}
	return results
}

func (ld *LogData) SitePaths(site string) []PathCount {
	rows, err := ld.db.Query("SELECT path, COUNT(*) as c FROM logs WHERE vhost = ? GROUP BY path ORDER BY c DESC", site)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []PathCount
	for rows.Next() {
		var p PathCount
		rows.Scan(&p.Path, &p.Count)
		results = append(results, p)
	}
	return results
}

func (ld *LogData) PathIPs(site, path string) []IPCount {
	var rows *sql.Rows
	var err error
	if site == "" {
		rows, err = ld.db.Query("SELECT ip, COUNT(*) as c FROM logs WHERE path = ? GROUP BY ip ORDER BY c DESC", path)
	} else {
		rows, err = ld.db.Query("SELECT ip, COUNT(*) as c FROM logs WHERE vhost = ? AND path = ? GROUP BY ip ORDER BY c DESC", site, path)
	}
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []IPCount
	for rows.Next() {
		var ic IPCount
		rows.Scan(&ic.IP, &ic.Count)
		ic.Location = ld.GetIPLocation(ic.IP)
		results = append(results, ic)
	}
	return results
}

func (ld *LogData) IPPaths(ip string) []PathInfo {
	rows, err := ld.db.Query("SELECT path, COUNT(*) as c, MAX(time) FROM logs WHERE ip = ? GROUP BY path ORDER BY c DESC", ip)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []PathInfo
	for rows.Next() {
		var p PathInfo
		rows.Scan(&p.Path, &p.Count, &p.Time)
		results = append(results, p)
	}
	return results
}

func (ld *LogData) StatusIPs(statusStart, statusEnd int) []IPCount {
	rows, err := ld.db.Query("SELECT ip, COUNT(*) as c FROM logs WHERE status BETWEEN ? AND ? GROUP BY ip ORDER BY c DESC", statusStart, statusEnd)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []IPCount
	for rows.Next() {
		var ic IPCount
		rows.Scan(&ic.IP, &ic.Count)
		ic.Location = ld.GetIPLocation(ic.IP)
		results = append(results, ic)
	}
	return results
}

func (ld *LogData) StatusIPDetails(ip string, statusStart, statusEnd int) []RequestInfo {
	rows, err := ld.db.Query("SELECT path, status, time FROM logs WHERE ip = ? AND status BETWEEN ? AND ? ORDER BY time DESC", ip, statusStart, statusEnd)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []RequestInfo
	for rows.Next() {
		var r RequestInfo
		rows.Scan(&r.Path, &r.Status, &r.Time)
		results = append(results, r)
	}
	return results
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
