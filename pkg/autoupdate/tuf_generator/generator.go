// The tuf_generator is responsible for the build time setup of the Notary/TUF repositories.
//
// Notary (and TUF in general) are designed around the idea that a distributed binary has a local
// repos which is periodically synced with the remote repo. To enable this, the binary must start
// with a local repo. During build, we fetch the current repo, and then embed that.
//
// In the past, this fetch was done using the builder.go tools, and packaged with go-bindata. Now
// enough is in go, that we can move away from those.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kolide/kit/fsutil"
	"github.com/peterbourgon/ff/v3"
	"github.com/pkg/errors"
	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/trustpinning"
	"github.com/theupdateframework/notary/tuf/data"
)

func main() {
	fs := flag.NewFlagSet("tuf-generator", flag.ExitOnError)

	var (
		fsNotaryConfigDir = fs.String("config", "", "Initial directory to seed notary from")
		fsRemoteURL       = fs.String("remote-url", "https://notary.kolide.co", "Notary server")
		fsLocalRepo       = fs.String("dir", "", "Local Repo Directory")
		fsGUN             = fs.String("gun", "", "Notary Global Unique Name (GUN)")
	)

	if err := ff.Parse(fs, os.Args[1:],
		ff.WithEnvVarPrefix("KOLIDE_LAUNCHE_TUF_GENERATOR"),
		ff.WithConfigFileParser(ff.PlainParser),
	); err != nil {
		fmt.Fprintf(os.Stderr, "Got error parsing flags: %v\n", err)
		os.Exit(1)
	}

	missingOpt := false
	for flag, val := range map[string]string{
		"config":     *fsNotaryConfigDir,
		"remote-url": *fsRemoteURL,
		"dir":        *fsLocalRepo,
		"gun":        *fsGUN,
	} {
		if val == "" {
			fmt.Fprintf(os.Stderr, "Missing required option: %s\n", flag)
			missingOpt = true
		}
	}

	if missingOpt {
		os.Exit(1)
	}

	fmt.Printf("Setting up TUF repo for %s\n", *fsGUN)

	if err := setupRepo(*fsLocalRepo, *fsGUN, *fsRemoteURL, *fsNotaryConfigDir); err != nil {
		fmt.Fprintf(os.Stderr, "Got error: %v\n", err)
		os.Exit(1)
	}
}

func setupRepo(repoDir, gun, remoteServerURL, notaryConfigDir string) error {
	// Update the notaryConfigDir
	if err := updateNotaryDir(notaryConfigDir, remoteServerURL, gun); err != nil {
		return fmt.Errorf("bootstrap notary GUN %s: %w", gun, err)
	}

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return fmt.Errorf("make repo dir %s: %w", repoDir, err)
	}

	// Copy the notary config metadata into our assets.
	source := filepath.Join(notaryConfigDir, "tuf", gun, "metadata")
	if err := fsutil.CopyDir(source, repoDir); err != nil {
		return errors.Wrap(err, "copying TUF repo metadata")
	}

	return nil
}

func updateNotaryDir(notaryConfigDir, remoteServerURL, gun string) error {
	passwordRetrieverFn := func(key, alias string, createNew bool, attempts int) (pass string, giveUp bool, err error) {
		pass = os.Getenv(key)
		if pass == "" {
			err = fmt.Errorf("missing pass phrase env var %q", key)
		}
		return pass, giveUp, err
	}

	// Safely fetch and validate all TUF metadata from remote Notary server.
	repo, err := client.NewFileCachedRepository(
		notaryConfigDir,
		data.GUN(gun),
		remoteServerURL,
		&http.Transport{Proxy: http.ProxyFromEnvironment},
		passwordRetrieverFn,
		trustpinning.TrustPinConfig{},
	)
	if err != nil {
		return fmt.Errorf("create an instance of the TUF repository: %w", err)
	}

	if _, err := repo.GetAllTargetMetadataByName(""); err != nil {
		return fmt.Errorf("getting all target metadata: %w", err)
	}

	return nil
}
