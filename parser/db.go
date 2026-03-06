package parser

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE logs (
	time TEXT,
	remote_addr TEXT,
	x_forwarded_for TEXT,
	request_id TEXT,
	status INTEGER,
	vhost TEXT,
	path TEXT,
	method TEXT,
	http_user_agent TEXT,
	request_time REAL,
	duration REAL,
	bytes_sent INTEGER,
	request_length INTEGER,
	request_proto TEXT,
	request_query TEXT,
	http_referrer TEXT,
	x_http_cf_connecting_ip TEXT,
	remote_user TEXT,
	ip TEXT
);
CREATE INDEX idx_vhost ON logs(vhost);
CREATE INDEX idx_path ON logs(path);
CREATE INDEX idx_ip ON logs(ip);
CREATE INDEX idx_status ON logs(status);
`

func openDB() (*sql.DB, string, error) {
	dbPath := filepath.Join(os.TempDir(), "ingrs_logs_analyst.db")
	os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open sqlite: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=OFF;"); err != nil {
		db.Close()
		os.Remove(dbPath)
		return nil, "", fmt.Errorf("failed to set pragmas: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		os.Remove(dbPath)
		return nil, "", fmt.Errorf("failed to create schema: %w", err)
	}
	return db, dbPath, nil
}

const insertSQL = `INSERT INTO logs (
	time, remote_addr, x_forwarded_for, request_id, status, vhost, path,
	method, http_user_agent, request_time, duration, bytes_sent,
	request_length, request_proto, request_query, http_referrer,
	x_http_cf_connecting_ip, remote_user, ip
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

func insertEntry(stmt *sql.Stmt, e LogEntry) error {
	vhost := e.VHost
	if vhost == "" {
		vhost = "default"
	}
	_, err := stmt.Exec(
		e.Time, e.RemoteAddr, e.XForwardedFor, e.RequestID, e.Status,
		vhost, e.Path, e.Method, e.HTTPUserAgent, e.RequestTime,
		e.Duration, e.BytesSent, e.RequestLength, e.RequestProto,
		e.RequestQuery, e.HTTPReferrer, e.XHttpCfConnect, e.RemoteUser,
		ExtractIP(e),
	)
	return err
}
