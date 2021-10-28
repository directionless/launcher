package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kolide/kit/logutil"
	"github.com/kolide/launcher/pkg/contexts/ctxlog"
	"github.com/kolide/launcher/pkg/osquery/runtime"
	"github.com/pkg/errors"
)

func runInteractive(args []string) error {
	flagset := flag.NewFlagSet("launcher interactive", flag.ExitOnError)
	var (
		flOsquerydPath = flagset.String("osqueryd_path", "", "Path to the osqueryd binary to use (Default: find osqueryd in $PATH)")
		flDebug        = flagset.Bool("debug", false, "Whether or not debug logging is enabled (default: false)")
	)
	flagset.Usage = commandUsage(flagset, "launcher interactive")
	if err := flagset.Parse(args); err != nil {
		return err
	}

	logger := log.With(
		logutil.NewCLILogger(*flDebug),
		"caller", log.DefaultCaller,
	)

	ctx, cancel := context.WithCancel(ctxlog.NewContext(context.Background(), logger))
	defer cancel()

	level.Debug(logger).Log("msg", "runInteractive starting")

	osquerydPath := *flOsquerydPath
	if osquerydPath == "" {
		osquerydPath = findOsquery()
		if osquerydPath == "" {
			return errors.New("Could not find osqueryd binary")
		}
	}

	runner := runtime.LaunchUnstartedInstance(
		runtime.WithOsquerydBinary(osquerydPath),
		runtime.WithOsqueryVerbose(true),
		runtime.WithLogger(logger),

		runtime.WithStdout(os.Stdout),
		runtime.WithStderr(os.Stderr),
		runtime.WithStdin(os.Stdin),

		runtime.WithOsqueryInteractive(),
		runtime.WithOsqueryContext(ctx),
	)

	if err := runner.Start(); err != nil {
		return errors.Wrap(err, "running osquery")
	}

	fmt.Println("That was fast")

	return nil
}

func osqueryInteractive(ctx context.Context, osqueryPath string) error {
	cmd := exec.CommandContext(
		ctx,
		osqueryPath,
		"--disable_events",
		"--disable_database",
		"--ephemeral",
		"-S",
	)

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()

}
