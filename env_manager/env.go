package env_manager

import "os"

type EnvManager struct {
	ConfigDir string
	LogDir    string
}

func NewEnvManager() *EnvManager {
	logDir, ok := os.LookupEnv("URUFLOW_LOG_DIR")
	if !ok {
		panic("Environment variable 'URUFLOW_LOG_DIR' is not set")
	}
	configDir, ok := os.LookupEnv("URUFLOW_CONFIG_DIR")
	if !ok {
		panic("Environment variable 'URUFLOW_CONFIG_DIR' is not set")
	}
	return &EnvManager{ConfigDir: configDir, LogDir: logDir}
}
