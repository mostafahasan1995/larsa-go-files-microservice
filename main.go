package main

import (
	"errors"
	"fmt"
	"larsa-files/configs"
	"net/http"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/rs/cors"

	driveHandler "larsa-files/file/delivery/http"
	fileHandler "larsa-files/file/delivery/http"
	mognoFileRespositoty "larsa-files/file/repository/mongo"
	usecase "larsa-files/file/usecase"

	"larsa-files/pkg/optimization"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"git.larsa.io/mahdawi/microservices-commons.git/interval"

	"larsa-files/pkg/events"
	"larsa-files/pkg/jobs"
)

func main() {
	fmt.Println("deployment date 5/12/22")
	app, err := configs.InitializeApp()
	if err != nil {
		fmt.Println(err)
		//fmt.Fprintf(os.Stderr, "%s/n", err)
		os.Exit(1)
		return
	}

	//subscriptions
	go func() {
		interval.MaxRetries = -1
		err := interval.Do(24*60*60, func(attempet int) (bool, error) {
			// fmt.Println("fetch subscription interval")
			c := make(chan string)
			go common.FetchSubscriptions(common.ClientSubscriptionInstance, c)
			<-c
			return true, errors.New("fetch Subscriotions interval" + time.Now().String())
		})
		if err != nil {
			fmt.Println("setting fetchsubscriotions interval error:", err)
		}
	}()

	//app
	fp := mognoFileRespositoty.NewMongoFileRepository(app.Mongodb, app.Cache)
	fu := usecase.NewFileUsecase(fp)
	fileHandler.NewHttpFileHandler(app.Router, fu)

	//drive module
	folderRepo := mognoFileRespositoty.NewMongoFolderRepository(app.Mongodb, app.Cache)
	du := usecase.NewDriveUsecase(fp, folderRepo)
	driveHandler.SetupDriveRoutes(app.Router, du)

	//NATs events
	events.SetupEvents(fp)
	//Images and Videos optimization
	optimization.RegenerateVariants()
	optimization.Client = app.Mongodb
	optimization.Cache = app.Cache

	//jobs
	jobs.SetupJobs(fp)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		//Debug:            true,
		AllowedMethods: []string{"HEAD", "GET", "POST", "PATCH", "DELETE", "OPTION", "PUT"},
		//AllowedHeaders:   []string{"access-control-allow-origin", "x-client", "x-access-token", "x-service", "x-expire", "x-service-token", "x-user-id", "x-video-converting-options"},
		AllowedHeaders: []string{"Access-Control-Allow-Origin", "accept-encoding", "accept-language", "Content-Type", "authorization", "accept", "x-client", "x-access-token", "x-service", "x-expire", "x-service-token", "x-user-id", "x-video-converting-options"},
		//ExposedHeaders:   []string{"access-control-allow-origin", "x-client", "x-access-token", "x-service", "x-expire", "x-service-token", "x-user-id"},
		//ExposedHeaders: []string{"Access-Control-Allow-Origin", "Content-Type", "x-client", "x-access-token", "x-service", "x-expire", "x-service-token", "x-user-id"},
		//ExposedHeaders: []string{"Access-Control-Allow-Origin", "Content-Type", "authorization", "accept", "x-client", "x-access-token", "x-service", "x-expire", "x-service-token", "x-user-id", "x-video-converting-options"},
	})
	handler := c.Handler(app.Router)

	http.ListenAndServe("0.0.0.0:"+configs.Getenv("PORT", "3200"), handler)

}
