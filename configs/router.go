package configs

import (
	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/gorilla/mux"
)

func NewRouter() *mux.Router {
	r := mux.NewRouter()
	r.Use(common.ClientSubscriptionInstance.CheckSubscription)
	return r
}
