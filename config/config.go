package config

import (
	"os"
	"strconv"
)

// AppConfig holds all application configuration values.
type AppConfig struct {
	ServerPort       string
	AllowedOrigins   string
	EnableUtilSign   bool
	SignerPrivateKey string
	DefaultNetwork   string
	RpcUrl           string
	ChainId          uint64
}

// LoadConfig reads configuration from environment variables with sensible defaults.
func LoadConfig() *AppConfig {
	return &AppConfig{
		ServerPort:       getEnv("SERVER_PORT", "8080"),
		AllowedOrigins:   getEnv("ALLOWED_ORIGINS", "*"),
		EnableUtilSign:   getEnvBool("ENABLE_UTIL_SIGN", false),
		SignerPrivateKey: getEnv("SIGNER_PRIVATE_KEY", ""),
		DefaultNetwork:   getEnv("DEFAULT_NETWORK", "devNet"),
		RpcUrl:           getEnv("MILON_RPC_URL", ""),
		ChainId:          getEnvUint64("MILON_CHAIN_ID", 0),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		b, err := strconv.ParseBool(value)
		if err != nil {
			return defaultValue
		}
		return b
	}
	return defaultValue
}

func getEnvUint64(key string, defaultValue uint64) uint64 {
	if value, exists := os.LookupEnv(key); exists {
		v, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return defaultValue
		}
		return v
	}
	return defaultValue
}
