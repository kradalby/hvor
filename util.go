package main

import (
	"os"
	"strconv"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if val, err := strconv.ParseBool(getEnv(key, "")); err == nil {
		return val
	}

	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val, err := strconv.Atoi(getEnv(key, "")); err == nil {
		return val
	}

	return fallback
}
