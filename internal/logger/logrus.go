package logger

import (
	"io"
	"log/slog"

	sloghook "github.com/shogo82148/logrus-slog-hook"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.AddHook(sloghook.New(slog.Default().Handler()))
	logrus.SetFormatter(sloghook.NewFormatter())
	logrus.SetOutput(io.Discard)
}

func SlogBackedLogrus() *logrus.Logger {
	return logrus.StandardLogger()
}
