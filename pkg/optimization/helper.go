package optimization

import (
	"context"
	"fmt"
	"larsa-files/domain"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/cache"
	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type fileData struct {
	extension   string
	destination string
	fileName    string
	uuid        string
	savePath    string
}

var Client *mongo.Client
var Cache cache.Cache

func IsSupportedFormat(extension string, supportedExtentions []string) bool {
	//check if the extension is in the array of supported extensions
	for _, ext := range supportedExtentions {
		ext = strings.ToLower(ext)
		if ext == extension {
			return true
		}
	}
	return false
}

func GetFileData(img string) fileData {
	var fileData fileData
	filePath := strings.Split(img, "/")
	fileDir := filePath[0 : len(filePath)-1]
	fileData.savePath = strings.Join(fileDir[:len(filePath)-1], "/")
	fileData.destination = strings.Join(fileDir[1:len(filePath)-1], "/")
	fileData.extension = filepath.Ext(img)
	fileData.fileName = filePath[len(filePath)-1]
	fileData.uuid = strings.TrimSuffix(fileData.fileName, filepath.Ext(fileData.fileName))
	return fileData
}

func removeFile(fileName string) {
	//remove the file
	err := os.Remove(fileName)
	if err != nil {
		fmt.Println("Error removing file: ", err)
	}
}

func renameFile(oldFileName string, newFileName string) {
	//rename the file
	err := os.Rename(oldFileName, newFileName)
	if err != nil {
		fmt.Println("Error renaming file: ", err)
	}
}

func updateDatabase(headerParams *common.HeaderParams, filePath string, variant []string) {
	collection := Client.Database(headerParams.Client).Collection("files")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"path": filePath}
	var file domain.File
	error := collection.FindOneAndUpdate(
		ctx,
		filter,
		// bson.M{"$set": bson.M{"variants": variant}},
		bson.M{"$push": bson.M{"variants": bson.M{"$each": variant}}},
	).Decode(&file)
	if error != nil {
		fmt.Println("Error updating database: ", error.Error())
	}

	deleteCache(headerParams.Client)
}

func deleteCache(clientID string) {
	batchDeletionKey := "files:p:c:" + clientID + ":*"

	Cache.BatchDeletionKeysByPattern(batchDeletionKey, 100)
}

func checkIfStream(headerParams *common.HeaderParams) bool {

	// check if there is a preferred image dimension
	SHOULD_GENERATE_STREAM, err := common.GetOptionValue("SHOULD_GENERATE_STREAM", headerParams)
	if err == nil && SHOULD_GENERATE_STREAM != nil {
		return true
	}

	return false
}
