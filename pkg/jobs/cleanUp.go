package jobs

import (
	"fmt"
	"larsa-files/domain"
	"larsa-files/utils"
	"log"
	"path/filepath"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/robfig/cron"
	"go.mongodb.org/mongo-driver/bson"
)

func SetupJobs(fileRepo domain.FileRepositoty) {
	c := cron.New()
	c.AddFunc("@daily", func() {
		cleanUp(fileRepo)
	})
	c.Start()
}

func cleanUp(fileRepo domain.FileRepositoty) {
	for _, c := range common.ClientSubscriptionInstance.SubscriptionData {
		go cleanClientFiles(c.Id, fileRepo)
	}
}

func cleanClientFiles(clientId string, fileRepo domain.FileRepositoty) {

	headerParams := &common.HeaderParams{
		Client: clientId,
	}
	folder := utils.UploadPath(headerParams)

	query := bson.M{
		"expire": map[string]interface{}{
			"$exists": true,
			"$ne":     time.Time{},
		},
	}
	result, err := fileRepo.GetAll(clientId, "", query)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("to delete", clientId, result)

	for _, f := range result {
		if !f.Expire.IsZero() {
			now := time.Now()
			then := f.Expire
			if then.Before(now) {
				path := filepath.Join(folder, f.Path)
				fmt.Println("delete File os: ", f.Expire, path)
				// err := os.Remove(path)
				// if err == nil {
				// 	_, err = fileRepo.Delete(clientId, f.Id, "")
				// 	fmt.Println("delete File db: ", f.Expire, path, err)
				// }
			}
		}

	}
}
