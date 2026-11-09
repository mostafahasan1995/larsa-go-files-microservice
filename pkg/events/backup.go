package events

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/nats"
	stan "github.com/nats-io/stan.go"

	"larsa-files/domain"
	"larsa-files/pkg/backup"
	natss "github.com/nats-io/nats.go"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	BackupSubject       = "backup:backup:files"
	RestoreSubject      = "backup:restore:files"
	DeleteSubject       = "backup:delete"
	BackupReplaySubject = "files:backup:replay"
)

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func SetupEvents(fileRepo domain.FileRepositoty) {
	env := getEnv("NATS_JETSTREAM", "false")
	if env == "false" {
		SetupEventsNats(fileRepo)
	} else {
		SetupEventsJet(fileRepo)
	}
}

func SetupEventsNats(fileRepo domain.FileRepositoty) {

	nats.Listen(BackupSubject, "files", "files", func(m *stan.Msg) {
		fmt.Println("backup event start")
		var payload = &domain.BackupEventPayload{}
		json.Unmarshal(m.Data, &payload)

		c := make(chan domain.BackupData)
		go backup.Backup(payload.Headers, payload.Data, c)
		payload.Data = <-c

		//add doc to the file collection and doc id to the payload
		if payload.Data.Status == "success" {
			file := domain.File{
				Id:           primitive.NewObjectID(),
				Path:         payload.Data.Path,
				OriginalName: payload.Data.Name,
				Service:      payload.Headers.Service,
				Expire:       payload.Headers.Expire,
				CreatedBy:    "",
				CreatedAt:    time.Now(),
			}
			res, err := fileRepo.Store(payload.Headers.Client, file)
			if err != nil {
				payload.Data.Error = err.Error()
				payload.Data.Status = "failed"

				c := make(chan domain.BackupData)
				go backup.Delete(payload.Headers, payload.Data, c)
				<-c
			} else {
				payload.Data.File = res.Id
			}
		}
		nats.Publish(BackupReplaySubject, payload)
		m.Ack()
	})

	nats.Listen(RestoreSubject, "files", "files", func(m *stan.Msg) {
		var payload = &domain.BackupEventPayload{}
		json.Unmarshal(m.Data, &payload)

		c := make(chan domain.BackupData)
		go backup.Restore(payload.Headers, payload.Data, c)
		payload.Data = <-c
		m.Ack()
	})

	nats.Listen(DeleteSubject, "files", "files", func(m *stan.Msg) {
		var payload = &domain.BackupEventPayload{}
		json.Unmarshal(m.Data, &payload)

		c := make(chan domain.BackupData)
		go backup.Delete(payload.Headers, payload.Data, c)
		payload.Data = <-c

		_, err := fileRepo.Delete(payload.Headers.Client, payload.Data.File, "")
		if err != nil {
			fmt.Println("error delete backup doc from the collection", err.Error())
		}
		m.Ack()
	})
}

func SetupEventsJet(fileRepo domain.FileRepositoty) {

	nats.ListenJet(BackupSubject, "files", "files", func(m *natss.Msg) {
		fmt.Println("backup event start")
		var payload = &domain.BackupEventPayload{}
		json.Unmarshal(m.Data, &payload)

		c := make(chan domain.BackupData)
		go backup.Backup(payload.Headers, payload.Data, c)
		payload.Data = <-c

		//add doc to the file collection and doc id to the payload
		if payload.Data.Status == "success" {
			file := domain.File{
				Id:           primitive.NewObjectID(),
				Path:         payload.Data.Path,
				OriginalName: payload.Data.Name,
				Service:      payload.Headers.Service,
				Expire:       payload.Headers.Expire,
				CreatedBy:    "",
				CreatedAt:    time.Now(),
			}
			res, err := fileRepo.Store(payload.Headers.Client, file)
			if err != nil {
				payload.Data.Error = err.Error()
				payload.Data.Status = "failed"

				c := make(chan domain.BackupData)
				go backup.Delete(payload.Headers, payload.Data, c)
				<-c
			} else {
				payload.Data.File = res.Id
			}
		}
		nats.PublishJet(BackupReplaySubject, payload)
		m.Ack()
	})

	nats.ListenJet(RestoreSubject, "files", "files", func(m *natss.Msg) {
		var payload = &domain.BackupEventPayload{}
		json.Unmarshal(m.Data, &payload)

		c := make(chan domain.BackupData)
		go backup.Restore(payload.Headers, payload.Data, c)
		payload.Data = <-c
		m.Ack()
	})

	nats.ListenJet(DeleteSubject, "files", "files", func(m *natss.Msg) {
		var payload = &domain.BackupEventPayload{}
		json.Unmarshal(m.Data, &payload)

		c := make(chan domain.BackupData)
		go backup.Delete(payload.Headers, payload.Data, c)
		payload.Data = <-c

		_, err := fileRepo.Delete(payload.Headers.Client, payload.Data.File, "")
		if err != nil {
			fmt.Println("error delete backup doc from the collection", err.Error())
		}
		m.Ack()
	})
}
