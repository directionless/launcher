package actions

import (
	"context"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/log/level"
)

func DeleteFiles(ctx context.Context, logger log.Logger, in actionInput) (actionOutput, error) {
	logger = log.With(logger, "action", "DeleteFiles")
	out := NewActionOutput()
	filesRaw, ok := in.Data["files"]
	if !ok {
		// TODO: Should this log? Error? Note that nothing happened in the output?
		out.State = actionNoop
		out.Reason = "No input file"
		return out, nil
	}

	files, ok := filesRaw.([]string)
	if !ok {
		out.State = actionNoop
		out.Reason = "Input files not array"
		return out, nil
	}

	deletedFiles := make([]string, len(files))
	for _, f := range files {
		fileInfo, err := os.Stat(f)
		switch {
		case os.IsNotExist(err):
			level.Debug(logger).Log("msg", "file does not exist", "path", f)
			continue
		case err != nil:
			level.Debug(logger).Log("msg", "error statting file", "path", f, "err", err)
			continue
		case !fileInfo.IsDir():
			level.Debug(logger).Log("msg", "file is a directory", "path", f)
			continue
		}

		// FIXME: check absolute

		if err := os.Remove(f); err != nil {
			// TODO: check for *PathError, which is non-existant
			level.Debug(logger).Log("msg", "error removing file", "path", f, "err", err)
			continue
		}

		deletedFiles = append(deletedFiles, f)
	}

	out.Data["files"] = deletedFiles
	return out, nil
}
