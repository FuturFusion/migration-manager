package nbdcopy

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/FuturFusion/migration-manager/internal/migratekit/progress"
)

func Run(source, destination string, size int64, targetIsClean bool, diskName string, statusCallback func(string, bool)) error {
	logger := log.WithFields(log.Fields{
		"command":     "nbdcopy",
		"source":      source,
		"destination": destination,
	})

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

	logger.Info("Running command: ", cmd)
	defer func() {
		log.Debug("stdout", stdout.String())
		if len(stderr.String()) > 0 {
			log.Error("stderr", stderr.String())
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
				log.Error("Error parsing progress: ", err)
				continue
			}

			bar.Set64(progress * size / 100)
			statusCallback(fmt.Sprintf("Importing disk %q: %02.2f%% complete", diskName, float64(progress)), false)
		}

		if err := scanner.Err(); err != nil {
			log.Error("Error reading progress: ", err)
		}
	}()

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}
