package configs

import "os"

type ServiceConfig struct {
	Resource       string
	PluralName     string
	SingularName   string
	CollectionName string
	CacheKeyPrefix string
}

var ServiceName = "files"

func Getenv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
