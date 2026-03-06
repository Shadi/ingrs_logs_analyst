package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/shadi/ingrs_logs_analyst/parser"
)

type Server struct {
	data *parser.LogData
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

func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, struct {
		TotalRequests int                `json:"TotalRequests"`
		TopSites      []parser.SiteCount `json:"TopSites"`
	}{
		TotalRequests: s.data.TotalRequests(),
		TopSites:      s.data.TopSites(),
	})
}

func (s *Server) handleSitePaths(w http.ResponseWriter, r *http.Request) {
	site := r.URL.Query().Get("site")
	jsonResponse(w, s.data.SitePaths(site))
}

func (s *Server) handlePathDetails(w http.ResponseWriter, r *http.Request) {
	site := r.URL.Query().Get("site")
	path := r.URL.Query().Get("path")
	jsonResponse(w, s.data.PathIPs(site, path))
}

func (s *Server) handleIPDetails(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	jsonResponse(w, struct {
		Location parser.IPLocation `json:"location"`
		Paths    []parser.PathInfo `json:"paths"`
	}{
		Location: s.data.GetIPLocation(ip),
		Paths:    s.data.IPPaths(ip),
	})
}

func (s *Server) handleStatusDetails(w http.ResponseWriter, r *http.Request) {
	codeStr := r.URL.Query().Get("code")
	targetIP := r.URL.Query().Get("ip")

	var statusStart, statusEnd int
	if len(codeStr) == 3 && codeStr[1:] == "xx" {
		base, _ := strconv.Atoi(string(codeStr[0]))
		statusStart = base * 100
		statusEnd = statusStart + 99
	} else {
		code, _ := strconv.Atoi(codeStr)
		statusStart = code
		statusEnd = code
	}

	if targetIP == "" {
		jsonResponse(w, s.data.StatusIPs(statusStart, statusEnd))
	} else {
		jsonResponse(w, s.data.StatusIPDetails(targetIP, statusStart, statusEnd))
	}
}
