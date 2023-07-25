package gossip_leaderelection

import (
	"bytes"
	"log"

	"github.com/bombsimon/logrusr/v4"
	"github.com/sirupsen/logrus"
	"k8s.io/klog/v2"
)

func newLogger() *log.Logger {
	return log.New(&logw{logger: logrus.StandardLogger()}, "", 0)
}

type logw struct {
	logger logrus.FieldLogger
}

func (l *logw) Write(b []byte) (int, error) {
	b = bytes.TrimRight(b, "\n")
	switch {
	case bytes.HasPrefix(b, []byte("[DEBUG]")):
		l.logger.Debug(string(bytes.TrimPrefix(b, []byte("[DEBUG] "))))
	case bytes.HasPrefix(b, []byte("[INFO]")):
		l.logger.Info(string(bytes.TrimPrefix(b, []byte("[INFO] "))))
	case bytes.HasPrefix(b, []byte("[WARN]")):
		l.logger.Warn(string(bytes.TrimPrefix(b, []byte("[WARN] "))))
	case bytes.HasPrefix(b, []byte("[ERROR]")):
		l.logger.Error(string(bytes.TrimPrefix(b, []byte("[ERROR] "))))
	default:
		l.logger.Info(string(b))
	}
	return len(b), nil
}

func init() {
	klog.SetLogger(logrusr.New(logrus.StandardLogger()))
}
