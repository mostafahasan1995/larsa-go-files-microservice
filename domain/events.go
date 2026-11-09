package domain

import (
	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BackupData struct {
	Id     primitive.ObjectID `bosn:"_id" json:"id"`
	Path   string             `bson:"path" json:"path"`
	Name   string             `bson:"name" json:"name"`
	File   primitive.ObjectID `bosn:"file" json:"file"`
	Status string             `bson:"status" json:"status"`
	Error  string             `bson:"error" json:"error"`
}

type BackupEventPayload struct {
	Headers *common.HeaderParams `json:"headers"`
	Data    BackupData           `json:"data"`
}

type StreamingPayload struct {
	Data    File                 `json:"data"`
	Headers *common.HeaderParams `json:"headers"`
	Error   error                `json:"error"`
}
