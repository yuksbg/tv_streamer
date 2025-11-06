package logs

import (
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	instance *logrus.Logger
	once     sync.Once
)

// GetLogger returns a singleton instance of logrus.Logger
func GetLogger() *logrus.Logger {
	once.Do(func() {
		instance = logrus.New()
		instance.SetLevel(logrus.DebugLevel)
		instance.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	})
	return instance
}
