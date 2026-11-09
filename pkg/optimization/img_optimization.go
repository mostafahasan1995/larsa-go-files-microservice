package optimization

import (
	"context"
	"encoding/json"
	"fmt"

	// "net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	natss "github.com/nats-io/nats.go"


	"git.larsa.io/mahdawi/microservices-commons.git/cache"
	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"git.larsa.io/mahdawi/microservices-commons.git/nats"
	"github.com/h2non/bimg"
	"github.com/nats-io/stan.go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"larsa-files/pkg/uploadS3"
	"larsa-files/utils"
)



type size struct {
	width  int
	height int
}

type VariantDto struct {
	Docs         []DocumentToRegenerate
	HeaderParams *common.HeaderParams
	Cache        cache.Cache
}

type DocumentToRegenerate struct {
	Variants          []string
	Id                string
	OriginalImagePath string
}

type imageSource struct {
	extension   string
	destination string
	fileName    string
	uuid        string
	savePath    string
}
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
func Optimize(headerParams *common.HeaderParams, img string) []string {

	var variants []string
	preferredSizes := getPreferredSizes(headerParams)
	preferredTypes := getPreferredTypes(headerParams)

	imgSource := getImageSourceData(img)

	// check if the file is an image
	if !isImage(imgSource.extension) {
		return []string{}

	} else {

		for _, preferredSize := range preferredSizes {
			for _, preferredType := range preferredTypes {
				imgType := mapPreferredType(preferredType)

				options := bimg.Options{
					Type:   imgType,
					Width:  preferredSize.width,
					Height: preferredSize.height,
				}

				buffer, err := bimg.Read(img)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
				}

				newImage, err := bimg.NewImage(buffer).Process(options)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
				img := bimg.NewImage(newImage)

				size, err := img.Size()
				if err != nil || preferredSize.width != size.Width || preferredSize.height != size.Height {
					fmt.Fprintln(os.Stderr, err)
				}

				width := strconv.Itoa(size.Width)
				height := strconv.Itoa(size.Height)

				variantName := imgSource.uuid + "-" + width + "x" + height + "." + img.Type()
				variantPath := imgSource.savePath + "/" + variantName
				bimg.Write(variantPath, newImage)

				absolutePath := filepath.Join(imgSource.destination, variantName)
				variants = append(variants, absolutePath)
			}
		}
		return variants

	}
}
func OptimizeS3(headerParams *common.HeaderParams, img string) []string {

	var variants []string
	preferredSizes := getPreferredSizes(headerParams)
	preferredTypes := getPreferredTypes(headerParams)

	imgSource := getImageSourceData(img)

	// check if the file is an image
	if !isImage(imgSource.extension) {
		return []string{}

	} else {

		for _, preferredSize := range preferredSizes {
			for _, preferredType := range preferredTypes {
				imgType := mapPreferredType(preferredType)

				options := bimg.Options{
					Type:   imgType,
					Width:  preferredSize.width,
					Height: preferredSize.height,
				}

				buffer, err := bimg.Read(img)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
				}

				newImage, err := bimg.NewImage(buffer).Process(options)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
				img := bimg.NewImage(newImage)

				size, err := img.Size()
				if err != nil || preferredSize.width != size.Width || preferredSize.height != size.Height {
					fmt.Fprintln(os.Stderr, err)
				}

				width := strconv.Itoa(size.Width)
				height := strconv.Itoa(size.Height)

				variantName := imgSource.uuid + "-" + width + "x" + height + "." + img.Type()
				variantPath := imgSource.savePath + "/" + variantName
				bimg.Write(variantPath, newImage)

				absolutePath := filepath.Join(imgSource.destination, variantName)
				err = uploadS3.StoreFileS3(newImage, absolutePath)
				if err != nil {
					fmt.Println(err)
				}
				variants = append(variants, absolutePath)
			}
		}
		return variants

	}
}


func getImageSourceData(img string) imageSource {
	var imgData imageSource
	imgPath := strings.Split(img, "/")
	imgDir := imgPath[0 : len(imgPath)-1]
	imgData.savePath = strings.Join(imgDir[:len(imgPath)-1], "/")
	imgData.destination = strings.Join(imgDir[1:len(imgPath)-1], "/")
	imgData.extension = filepath.Ext(img)
	imgData.fileName = imgPath[len(imgPath)-1]
	imgData.uuid = strings.TrimSuffix(imgData.fileName, filepath.Ext(imgData.fileName))
	return imgData
}

func isImage(extension string) bool {
	// an array of image extensions
	imageExtensions := []string{".jpeg", ".png", ".webp", ".jpg"}
	//check if the extension is in the array
	for _, ext := range imageExtensions {
		ext = strings.ToLower(ext)
		if ext == extension {
			return true
		}
	}
	return false
}

func getPreferredSizes(headerParams *common.HeaderParams) []size {
	var imgSizes []size
	// check if there is a preferred image dimension
	PREFERRED_IMG_SIZES, err := common.GetOptionValue("PREFERRED_IMG_SIZES", headerParams)
	if err == nil && PREFERRED_IMG_SIZES != nil && PREFERRED_IMG_SIZES != "" {
		temp := strings.Split(PREFERRED_IMG_SIZES.(string), ",")
		for _, s := range temp {
			var sizeType size
			tempSize := strings.Split(s, "x")
			sizeType.width, _ = strconv.Atoi(tempSize[0])
			sizeType.height, _ = strconv.Atoi(tempSize[1])
			imgSizes = append(imgSizes, sizeType)
		}
	} else {
		return imgSizes
	}
	return imgSizes
}

func getPreferredTypes(headerParams *common.HeaderParams) []string {
	PREFERRED_IMG_TYPES, err := common.GetOptionValue("PREFERRED_IMG_TYPES", headerParams)
	if err == nil && PREFERRED_IMG_TYPES != nil && PREFERRED_IMG_TYPES != "" {
		imgTypes := PREFERRED_IMG_TYPES.(string)
		return strings.Split(imgTypes, ",")
	} else {
		return []string{}
	}
}

func mapPreferredType(imgType string) bimg.ImageType {
	// imgType to uppercase
	imgType = strings.ToUpper(imgType)

	switch imgType {
	case "JPEG":
		return bimg.JPEG
	case "PNG":
		return bimg.PNG
	case "WEBP":
		return bimg.WEBP
	default:
		return bimg.WEBP
	}
}
func RegenerateVariants() {
	env := getEnv("NATS_JETSTREAM", "false")

	if env == "false" {
		nats.Listen("files:regenerate-variants", "regenerate-variants", "regenerate-variants", func(msg *stan.Msg) {
			fmt.Println("regenearete eventttttttttttttttttttttttttttttttttttttttttttttttttttttt")
			var j VariantDto
			json.Unmarshal([]byte(msg.Data), &j)
			path := utils.UploadPath(j.HeaderParams)
			for _, document := range j.Docs {
				// tmpPath := path + document.OriginalImagePath
				tmpPath := filepath.Join(path, document.OriginalImagePath)
				for _, variant := range document.Variants {
					removeVariant(j.HeaderParams, variant)
				}
				
	
				variants := OptimizeS3(j.HeaderParams, tmpPath)
				updateDocument(j.HeaderParams, Client, variants, document.Id)
			}
	
			batchDeletionKey := "files:p:c:" + j.HeaderParams.Client + ":*"
			Cache.BatchDeletionKeysByPattern(batchDeletionKey, 100)
			msg.Ack()
		})
	} else {
		nats.ListenJet("files:regenerate-variants", "regenerate-variants", "regenerate-variants", func(msg *natss.Msg) {
			fmt.Println("regenearete eventttttttttttttttttttttttttttttttttttttttttttttttttttttt")
			var j VariantDto
			json.Unmarshal([]byte(msg.Data), &j)
			path := utils.UploadPath(j.HeaderParams)
			for _, document := range j.Docs {
				// tmpPath := path + document.OriginalImagePath
				tmpPath := filepath.Join(path, document.OriginalImagePath)
				for _, variant := range document.Variants {
					removeVariant(j.HeaderParams, variant)
				}
				
	
				variants := OptimizeS3(j.HeaderParams, tmpPath)
				updateDocument(j.HeaderParams, Client, variants, document.Id)
			}
	
			batchDeletionKey := "files:p:c:" + j.HeaderParams.Client + ":*"
			Cache.BatchDeletionKeysByPattern(batchDeletionKey, 100)
			msg.Ack()
		})
	}







	
}
// func RegenerateVariants() {
// 	nats.Listen("files:regenerate-variants", "regenerate-variants", "regenerate-variants", func(msg *stan.Msg) {
// 		fmt.Println("regenearete event")
// 		var j VariantDto
// 		json.Unmarshal([]byte(msg.Data), &j)
// 		path := utils.UploadPath(j.HeaderParams)
// 		for _, document := range j.Docs {
// 			// tmpPath := path + document.OriginalImagePath
// 			tmpPath := filepath.Join(path, document.OriginalImagePath)
// 			for _, variant := range document.Variants {
// 				removeVariant(j.HeaderParams, variant)
// 			}
// 			variants := Optimize(j.HeaderParams, tmpPath)
// 			updateDocument(j.HeaderParams, Client, variants, document.Id)
// 		}

// 		batchDeletionKey := "files:p:c:" + j.HeaderParams.Client + ":*"
// 		Cache.BatchDeletionKeysByPattern(batchDeletionKey, 100)
// 		msg.Ack()
// 	})
// }

func removeVariant(headerParams *common.HeaderParams, variantPath string) {
	newPath := filepath.Join(utils.UploadPath(headerParams), variantPath)
	os.Remove(newPath)
}

func updateDocument(headerParams *common.HeaderParams, client *mongo.Client, variants []string, documentId string) {
	collection := client.Database(headerParams.Client).Collection("files")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	objID, idErr := primitive.ObjectIDFromHex(documentId)
	if idErr != nil {
		fmt.Println("hex to ObjectID err", idErr.Error())
	}

	body := map[string]interface{}{}
	if len(variants) > 0 {
		body["variants"] = variants
	} else {
		body["variants"] = []string{}
	}

	filter := bson.M{"_id": objID}
	_, errUpdate := collection.UpdateOne(
		ctx,
		filter,
		bson.D{
			{"$set", body},
		},
	)
	if errUpdate != nil {
		fmt.Println("updateError", errUpdate.Error())
	}
}
