package config

import (
	"fmt"
	"os"
)

type Config struct {
	ApiKey string
}

func AppConfig() *Config {

	path, exists := os.LookupEnv("AGENT_API_KEY")
	if !exists {
		fmt.Println("AGENT_API_KEY is not defined.")
		return nil
	}
	return &Config{
		ApiKey: path,
	}

}
