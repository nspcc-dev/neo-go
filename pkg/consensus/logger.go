package consensus

import (
	"go.uber.org/zap"
)

func getLogger() (*zap.Logger, error) {
	cc := zap.NewDevelopmentConfig()
	cc.DisableCaller = true
	cc.DisableStacktrace = true
	cc.Encoding = "console"

	log, err := cc.Build()
	if err != nil {
		return nil, err
	}

	return log.With(zap.String("module", "dbft")), nil
}
