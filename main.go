package main

import (
	"net/http"
	"os"

	flags "github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/shadi/ingrs_logs_analyst/parser"
	"github.com/shadi/ingrs_logs_analyst/server"
)

type opts struct {
	IPDB       string `short:"i" long:"ipdb" default:"IP2LOCATION-DB11.BIN" description:"path to the IP2Location database file"`
	Source     string `short:"s" long:"source" default:"k8s" choice:"file" choice:"k8s" description:"log source"`
	File       string `short:"f" long:"file" default:"access.log" description:"path to the log file (used with --source=file)"`
	Namespace  string `short:"n" long:"namespace" default:"ingress-nginx" description:"kubernetes namespace (used with --source=k8s)"`
	Deployment string `short:"d" long:"deployment" default:"ingress-nginx-controller" description:"kubernetes deployment name (used with --source=k8s)"`
	Kubeconfig string `short:"k" long:"kubeconfig" default:"~/.kube/config" description:"path to kubeconfig file (used with --source=k8s)"`
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	var o opts
	if _, err := flags.Parse(&o); err != nil {
		os.Exit(1)
	}

	logSource, err := parser.NewLogSource(parser.LogSourceConfig{
		Source:     o.Source,
		File:       o.File,
		Namespace:  o.Namespace,
		Deployment: o.Deployment,
		Kubeconfig: o.Kubeconfig,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create log source")
	}

	data, err := parser.NewLogData()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database")
	}
	defer data.Close()

	count, err := data.LoadFrom(logSource)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load logs")
	}
	log.Info().Int("entries", count).Msg("loaded log entries")

	if err := data.LoadIPDB(o.IPDB); err != nil {
		log.Warn().Err(err).Msg("failed to load IP2Location DB, IP info will be limited")
	}

	srv := server.New(data)
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	log.Info().Str("addr", ":8080").Msg("server starting")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}
