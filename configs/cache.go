package configs

import (
	"fmt"
	"strconv"
	"strings"

	"git.larsa.io/mahdawi/microservices-commons.git/cache"
)

func SetUpCache() (cache.Cache, error) {
	host := Getenv("REDIS_HOST", "localhost")
	port := Getenv("REDIS_PORT", "6379")
	password := Getenv("REDIS_PASSWORD", "")
	valKeySentinelHost := Getenv("VALKEY_SENTINEL_HOST", "")
	valKeyMasterName := Getenv("VALKEY_MASTER_NAME", "")
	db, err := strconv.Atoi(Getenv("REDIS_DB", "0"))
	if err != nil {
		return nil, err
	}
	uri := host + ":" + port
	var redisCache cache.Cache
	cacheCluserMode := Getenv("REDIS_CLUSTER", "false")
	if cacheCluserMode == "true" {
		redisCache = cache.NewRedisClusterCache()
		errCache := redisCache.Connect(uri, password, db, []string{})
		if errCache != nil {
			return redisCache, errCache
		}
	} else {
		var host []string = []string{}
		redisCache = cache.NewRedisCache()
		if valKeySentinelHost != "" {
			fmt.Println("🚀 ~ funcSetUpCache ~ valKeySentinelHost:", valKeySentinelHost)
			uri = valKeyMasterName
			host = strings.Split(valKeySentinelHost, ", ")
		}
		errCache := redisCache.Connect(uri, password, db, host)
		if errCache != nil {
			return redisCache, errCache
		}
	}

	return redisCache, nil
}

