package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"github.com/shadi/ingrs_logs_analyst/parser"
)

type Server struct {
	data *parser.LogData
}

type SiteCount struct {
	Site  string `json:"site"`
	Count int    `json:"count"`
}

type PathCount struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

func New(data *parser.LogData) *Server {
	return &Server{data: data}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/site", s.handleSitePaths)
	mux.HandleFunc("/api/path", s.handlePathDetails)
	mux.HandleFunc("/api/ip", s.handleIPDetails)
	mux.HandleFunc("/api/status", s.handleStatusDetails)

	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	siteCounts := make(map[string]int)
	for _, l := range s.data.Entries {
		site := l.VHost
		if site == "" {
			site = "default"
		}
		siteCounts[site]++
	}

	var topSites []SiteCount
	for si, c := range siteCounts {
		topSites = append(topSites, SiteCount{Site: si, Count: c})
	}

	sort.Slice(topSites, func(i, j int) bool {
		return topSites[i].Count > topSites[j].Count
	})

	response := struct {
		TotalRequests int         `json:"TotalRequests"`
		TopSites      []SiteCount `json:"TopSites"`
	}{
		TotalRequests: len(s.data.Entries),
		TopSites:      topSites,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleSitePaths(w http.ResponseWriter, r *http.Request) {
	site := r.URL.Query().Get("site")

	pathCounts := make(map[string]int)
	for _, l := range s.data.Entries {
		lSite := l.VHost
		if lSite == "" {
			lSite = "default"
		}
		if lSite == site {
			pathCounts[l.Path]++
		}
	}

	var topPaths []PathCount
	for p, c := range pathCounts {
		topPaths = append(topPaths, PathCount{Path: p, Count: c})
	}

	sort.Slice(topPaths, func(i, j int) bool {
		return topPaths[i].Count > topPaths[j].Count
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(topPaths)
}

func (s *Server) handlePathDetails(w http.ResponseWriter, r *http.Request) {
	site := r.URL.Query().Get("site")
	path := r.URL.Query().Get("path")

	type IPInfo struct {
		IP       string            `json:"ip"`
		Count    int               `json:"count"`
		Location parser.IPLocation `json:"location"`
	}

	ipCounts := make(map[string]int)
	for _, l := range s.data.Entries {
		lSite := l.VHost
		if lSite == "" {
			lSite = "default"
		}
		if (site == "" || lSite == site) && l.Path == path {
			ip := parser.ExtractIP(l)
			ipCounts[ip]++
		}
	}

	var results []IPInfo
	for ip, count := range ipCounts {
		results = append(results, IPInfo{
			IP:       ip,
			Count:    count,
			Location: s.data.GetIPLocation(ip),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Count > results[j].Count
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleIPDetails(w http.ResponseWriter, r *http.Request) {
	targetIP := r.URL.Query().Get("ip")

	type PathInfo struct {
		Path  string `json:"path"`
		Count int    `json:"count"`
		Time  string `json:"last_time"`
	}

	response := struct {
		Location parser.IPLocation `json:"location"`
		Paths    []PathInfo        `json:"paths"`
	}{
		Location: s.data.GetIPLocation(targetIP),
	}

	pathCounts := make(map[string]int)
	lastTime := make(map[string]string)

	for _, l := range s.data.Entries {
		ip := parser.ExtractIP(l)
		if ip == targetIP {
			pathCounts[l.Path]++
			lastTime[l.Path] = l.Time
		}
	}

	for p, c := range pathCounts {
		response.Paths = append(response.Paths, PathInfo{Path: p, Count: c, Time: lastTime[p]})
	}

	sort.Slice(response.Paths, func(i, j int) bool {
		return response.Paths[i].Count > response.Paths[j].Count
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleStatusDetails(w http.ResponseWriter, r *http.Request) {
	codeStr := r.URL.Query().Get("code")
	targetIP := r.URL.Query().Get("ip")

	type IPStatusSummary struct {
		IP       string            `json:"ip"`
		Count    int               `json:"count"`
		Location parser.IPLocation `json:"location"`
	}

	type RequestInfo struct {
		Path   string `json:"path"`
		Status int    `json:"status"`
		Time   string `json:"time"`
	}

	isRange := false
	var rangeStart, rangeEnd int
	if len(codeStr) == 3 && codeStr[1:] == "xx" {
		isRange = true
		base, _ := strconv.Atoi(string(codeStr[0]))
		rangeStart = base * 100
		rangeEnd = rangeStart + 99
	}
	targetCode, _ := strconv.Atoi(codeStr)

	matchStatus := func(status int) bool {
		if isRange {
			return status >= rangeStart && status <= rangeEnd
		}
		return status == targetCode
	}

	if targetIP == "" {
		ipCounts := make(map[string]int)
		for _, l := range s.data.Entries {
			if matchStatus(l.Status) {
				ip := parser.ExtractIP(l)
				ipCounts[ip]++
			}
		}

		var results []IPStatusSummary
		for ip, count := range ipCounts {
			results = append(results, IPStatusSummary{
				IP:       ip,
				Count:    count,
				Location: s.data.GetIPLocation(ip),
			})
		}
		sort.Slice(results, func(i, j int) bool {
			return results[i].Count > results[j].Count
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
		return
	}

	var results []RequestInfo
	for _, l := range s.data.Entries {
		ip := parser.ExtractIP(l)
		if ip == targetIP && matchStatus(l.Status) {
			results = append(results, RequestInfo{
				Path:   l.Path,
				Status: l.Status,
				Time:   l.Time,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
