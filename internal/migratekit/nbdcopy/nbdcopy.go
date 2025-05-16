package nbdcopy

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/FuturFusion/migration-manager/internal/migratekit/progress"
)

func Run(message string, source string, destination string, size int64, targetIsClean bool, diskName string, statusCallback func(string, bool)) error {
	log := slog.With(
		slog.String("command", "nbdcopy"),
		slog.String("source", source),
		slog.String("destination", destination),
	)

	progressRead, progressWrite, err := os.Pipe()
	if err != nil {
		return err
	}
	defer progressRead.Close()

	args := []string{
		"--progress=3",
		source,
		destination,
	}

	if targetIsClean {
		args = append(args, "--destination-is-zero")
	}

	cmd := exec.Command(
		"nbdcopy",
		args...,
	)

	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.ExtraFiles = []*os.File{progressWrite}

	log.Info("Running command", slog.String("args", cmd.String()))
	defer func() {
		log.Debug("Command ended", slog.String("stdout", stdout.String()))
		if len(stderr.String()) > 0 {
			log.Error("Command errored", slog.String("stderr", stderr.String()))
		}
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	// Close the parent's copy of progressWrite
	// See: https://github.com/golang/go/issues/4261
	progressWrite.Close()

	bar := progress.DataProgressBar("Full copy", size)
	go func() {
		scanner := bufio.NewScanner(progressRead)
		for scanner.Scan() {
			progressParts := strings.Split(scanner.Text(), "/")
			progress, err := strconv.ParseInt(progressParts[0], 10, 64)
			if err != nil {
				log.Error("Error parsing progress: ", slog.Any("error", err))
				continue
			}

			bar.Set64(progress * size / 100)
			statusCallback(fmt.Sprintf("%s %q: %02.2f%% complete", message, diskName, float64(progress)), false)
		}

		if err := scanner.Err(); err != nil {
			log.Error("Error reading progress: ", slog.Any("error", err))
		}
	}()

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}
