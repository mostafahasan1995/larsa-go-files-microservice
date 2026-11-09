package configs

import (
	"os"

	"git.larsa.io/mahdawi/microservices-commons.git/nats"
	"github.com/google/uuid"
	// stan "github.com/nats-io/stan.go"
	// natss "github.com/nats-io/nats.go"

)

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
func NewNATs() any {

	env := getEnv("NATS_JETSTREAM", "false")
	if env == "false" {
		natsClient := nats.Connect("microservices", uuid.NewString(), Getenv("NATS_HOST", "localhost")+":"+Getenv("NATS_PORT", "4222"))
		return natsClient
	} else {
		natsClient := nats.ConnectJet("microservices", uuid.NewString(), Getenv("NATS_HOST", "localhost")+":"+Getenv("NATS_PORT", "4222"))
		return natsClient
	}

	

}
