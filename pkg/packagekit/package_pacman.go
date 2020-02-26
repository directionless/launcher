package packagekit

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/kolide/launcher/pkg/packagekit/internal"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
)

func PackagePacman(ctx context.Context, w io.Writer, po *PackageOptions) error {
	ctx, span := trace.StartSpan(ctx, "packagekit.PackagePkg")
	defer span.End()
	//logger := log.With(ctxlog.FromContext(ctx), "caller", "packagekit.PackagePacman")

	if err := isDirectory(po.Root); err != nil {
		return err
	}

	tmpDir, err := ioutil.TempDir("/tmp", "packaging-pacman-tmp")
	if err != nil {
		return errors.Wrap(err, "making TempDir")
	}
	defer os.RemoveAll(tmpDir)

	pkgbuildTemplateBytes, err := internal.Asset("internal/assets/PKGBUILD")
	if err != nil {
		return errors.Wrap(err, "getting go-bindata PKGBUILD")
	}

	var templateData = struct {
		Opts *PackageOptions
	}{
		Opts: po,
	}

	pkgbuildTemplate, err := template.New("PKGBUILD").Parse(string(pkgbuildTemplateBytes))
	if err != nil {
		return errors.Wrap(err, "not able to parse PKGBUILD template")
	}

	pkgbuildWrite, err := os.Create(filepath.Join(tmpDir, "PKGBUILD"))
	if err != nil {
		return errors.Wrap(err, "opening PKGBUILD for writing")
	}
	defer pkgbuildWrite.Close()

	if err := pkgbuildTemplate.ExecuteTemplate(pkgbuildWrite, "PKGBUILD", templateData); err != nil {
		return errors.Wrap(err, "executing pkgbuildTemplate")
	}
	pkgbuildWrite.Close()

	dockerArgs := []string{
		"-v", fmt.Sprintf("%s:/pkgsrc", po.Root),
		"-v", fmt.Sprintf("%s:/pkgscripts", po.Scripts),
		"-v", fmt.Sprintf("%s:/pkgtmp", tmpDir),
		"--entrypoint", "''", // FIXME // override this, to ensure more compatibility with the plain command line
		"-it",                // FIXME
		"whynothugo/makepkg", // FIXME
		"bash",               // FIXME
	}

	fmt.Println("pausing for execution")
	fmt.Printf("docker run ")
	for _, arg := range dockerArgs {
		fmt.Printf(" %s", arg)
	}
	fmt.Printf("\n")

	bio := bufio.NewReader(os.Stdin)
	_, _, _ = bio.ReadLine()

	//cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	return nil

}
