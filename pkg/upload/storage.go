package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"larsa-files/domain"
	"larsa-files/pkg/uploadS3"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/cache"
	"git.larsa.io/mahdawi/microservices-commons.git/common"

	"larsa-files/utils"

	"larsa-files/configs"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/karrick/godirwalk"
)

var Client *mongo.Client

func CheckStorage(c chan int, cache cache.Cache, headerParams *common.HeaderParams) {
	folder := utils.UploadPath(headerParams)
	//new mechanism for calculting the folder size
	cacheKey := fmt.Sprintf("files:fz:c:%v", headerParams.Client)
	cacheRes, errCache := cache.GetByKey(cacheKey)

	total := 0
	if errCache != nil {
		err := godirwalk.Walk(filepath.Join(folder, headerParams.Client), &godirwalk.Options{
			Callback: func(p string, de *godirwalk.Dirent) error {
				if de.IsDir() {
					fmt.Println("dir", p)
					return nil
				}

				info, err := os.Stat(p)
				if err != nil {
					fmt.Println("error get file info", err)

				}
				x := info.Size()
				total += int(x)
				return nil
			},
			Unsorted: true,
		})
		if err != nil {
			fmt.Println(err)
		}
		cache.CacheByKey(cacheKey, total, 0)
	} else {
		json.Unmarshal([]byte(cacheRes), &total)
	}

	c <- total
}

func CalDirFilesSize(folder string, files []domain.File) int {
	total := 0
	for _, file := range files {
		path := filepath.Join(folder, file.Path)
		info, err := os.Stat(path)
		if err != nil {
			fmt.Println("error get file info", err)
			continue
		}

		x := info.Size()
		total += int(x)
	}

	return total

}

func StorageMiddleware() common.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			headerParams := common.ExtractHeaderParams(r)

			storageChan := make(chan int)
			go CheckStorage(storageChan, configs.App.Cache, headerParams)
			storage := <-storageChan

			//check client storage limit baseds on subscription
			subscription := common.ClientSubscriptionInstance.SubscriptionData[headerParams.Client]
			if subscription.Storage != "" && subscription.Storage != "-1" {
				ClientStorageLimit, err := utils.GetBytes(subscription.Storage)
				if err != nil {
					http.Error(rw, err.Error(), http.StatusBadRequest)
					return
				}

				if storage >= ClientStorageLimit {
					http.Error(rw, "exceed the client storage limit", http.StatusBadRequest)
					return
				}
			}
			ctx := context.WithValue(r.Context(), common.CommonContextKey("storage"), storage)
			next.ServeHTTP(rw, r.Clone(ctx))
		})
	}
}
func StorageMiddlewareS3() common.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			headerParams := common.ExtractHeaderParams(r)

			collection := Client.Database(headerParams.Client).Collection("files")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			filter := bson.M{}
			curr, _ := collection.Find(ctx, filter)
			defer curr.Close(ctx)
			var filsKey []string


			for curr.Next(ctx) {
				var doc domain.File
				err := curr.Decode(&doc)
				if err != nil {
					return 
				}
				filsKey = append(filsKey, doc.Path)
			}
			if err := curr.Err(); err != nil {
				return 
			}

			storage := uploadS3.CalculateSize(filsKey)

			//check client storage limit baseds on subscription
			subscription := common.ClientSubscriptionInstance.SubscriptionData[headerParams.Client]
			if subscription.Storage != "" && subscription.Storage != "-1" {
				ClientStorageLimit, err := utils.GetBytes(subscription.Storage)
				if err != nil {
					http.Error(rw, err.Error(), http.StatusBadRequest)
					return
				}

				if storage >= ClientStorageLimit {
					http.Error(rw, "exceed the client storage limit", http.StatusBadRequest)
					return
				}
			}
			ctx = context.WithValue(r.Context(), common.CommonContextKey("storage"), storage)
			next.ServeHTTP(rw, r.Clone(ctx))
		})
	}
}
