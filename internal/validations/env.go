package validations

import "os"

func GetEnvWithDefault(envName, defaultValue string) string {
	if value := os.Getenv(envName); value != "" {
		return value
	}
	return defaultValue
}

func GetEnvOrDie(envName string) string {
	value := os.Getenv(envName)
	if value == "" {
		panic("Environment variable " + envName + " is not set")
	}
	return value
}
