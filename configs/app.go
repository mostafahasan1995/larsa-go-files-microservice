package configs

import (
	"errors"
	"fmt"
	"log"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/cache"
	"git.larsa.io/mahdawi/microservices-commons.git/interval"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	stan "github.com/nats-io/stan.go"
	natss "github.com/nats-io/nats.go"
)

type AppConfig struct {
	Mongodb *mongo.Client
	Cache   cache.Cache
	Router  *mux.Router
}

var App *AppConfig

func NewApp(db *mongo.Client, cache cache.Cache, router *mux.Router) *AppConfig {
	App = &AppConfig{
		Mongodb: db,
		Cache:   cache,
		Router:  router,
	}
	return App
}

func InitializeApp() (*AppConfig, error) {
	interval.MaxRetries = -1

	// start the nas client
	natsClient := NewNATs()
	if natsClient == nil {
		err := interval.Do(2, func(attempt int) (bool, error) {
			natsClient = NewNATs()
			fmt.Println("trying to connect nats server", time.Now())
			if natsClient == nil {
				return true, errors.New("could not connect to nats server")
			}
			return true, nil
		})
		if err != nil {
			log.Fatalln("connection to nats error:", err)
			return nil, err
		} else {
			// defer natsClient.Close()
			switch client := natsClient.(type) {
			case stan.Conn:
				defer client.Close() // Close for stan.Conn
			case natss.JetStreamContext:
				// JetStreamContext does not have a Close method
				fmt.Println("JetStream connection does not require closing.")
			default:
				log.Fatalln("unknown NATS client type")
			}
			fmt.Println("connected to NATs")
		}
	} else {
		fmt.Println("connected to NATs")
	}

	client, errDB := SetUpDataBase()
	if errDB != nil {
		err := interval.Do(2, func(attempt int) (bool, error) {
			fmt.Println("trying to connect to database", time.Now())
			client, errDB = SetUpDataBase()
			return true, errDB
		})
		if err != nil {
			fmt.Println("setup Database error:", err)
			return nil, err
		}
	}
	fmt.Println("connected to Mongodb")

	interval.MaxRetries = 2
	cache, errCache := SetUpCache()
	if errCache != nil {
		err := interval.Do(2, func(attempt int) (bool, error) {
			fmt.Println("trying to connect to cache", time.Now())
			cache, errCache = SetUpCache()
			return true, errCache
		})
		if err != nil {
			fmt.Println("setup cache error:", err)
		} else {
			fmt.Println("connected to Redis")
		}
	} else {
		fmt.Println("connected to Redis")
	}

	r := NewRouter()
	App = &AppConfig{
		Mongodb: client,
		Cache:   cache,
		Router:  r,
	}
	return App, nil
}
