package main

import (
	"flag"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/kolide/kit/logutil"
	"github.com/kolide/kit/version"
	"github.com/kolide/launcher/pkg/osquery/table"
	"github.com/kolide/osquery-go"
	"github.com/pkg/errors"
)

func runTables(args []string) error {
	flagset := flag.NewFlagSet("launcher tables", flag.ExitOnError)

	var (
		flSocketPath = flagset.String("socket", "", "")
		flTimeout    = flagset.Int("timeout", 0, "")
		flVerbose    = flagset.Bool("verbose", false, "")
		flVersion    = flagset.Bool("version", false, "Print  version and exit")
		_            = flagset.Int("interval", 0, "")
	)

	flagset.Usage = commandUsage(flagset, "launcher tables")
	if err := flagset.Parse(args); err != nil {
		return err
	}

	if *flVersion {
		version.PrintFull()
		return nil
	}

	logger := logutil.NewServerLogger(*flVerbose)

	level.Info(logger).Log(
		"msg", "Launcher tables starting up",
		"version", version.Version().Version,
		"revision", version.Version().Revision,
	)

	timeout := time.Duration(*flTimeout) * time.Second

	// allow for osqueryd to create the socket path
	time.Sleep(2 * time.Second)

	// create an extension server
	server, err := osquery.NewExtensionManagerServer(
		"com.kolide.standalone_extension",
		*flSocketPath,
		osquery.ServerTimeout(timeout),
	)
	if err != nil {
		return errors.Wrap(err, "creating osquery extension server")
	}

	client, err := osquery.NewClient(*flSocketPath, timeout)
	if err != nil {
		return errors.Wrap(err, "creating osquery extension client")
	}

	var plugins []osquery.OsqueryPlugin
	for _, tablePlugin := range table.PlatformTables(client, logger, "osqueryd") {
		plugins = append(plugins, tablePlugin)
	}
	server.RegisterPlugin(plugins...)

	return server.Run()

}
