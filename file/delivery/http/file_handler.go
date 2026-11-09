package file

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"larsa-files/domain"
	"larsa-files/pkg/uploadS3"
	"log"
	"strconv"

	// "log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"larsa-files/pkg/upload"

	"larsa-files/utils"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"git.larsa.io/mahdawi/microservices-commons.git/nats"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-chi/httprate"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"larsa-files/pkg/optimization"
)

const region = "us-east-1"

var (
	bucketName = os.Getenv("S3_BUCKET")
	accessKey  = os.Getenv("S3_ACCESS_KEY")
	secretKey  = os.Getenv("S3_SECRET_KEY")
	endpoint   = os.Getenv("S3_ENDPOINT")
)

type httpFileHandler struct {
	FileUsecase domain.FileUsecase
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func (fh *httpFileHandler) GetAll() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		var filter bson.M
		if query := r.URL.Query().Get("query"); query != "" {
			errQ := json.Unmarshal([]byte(query), &filter)
			if errQ != nil {
				http.Error(rw, errQ.Error(), http.StatusBadRequest)
				return
			}
		}

		res, err := fh.FileUsecase.GetAll(headerParams, r.URL.RequestURI(), filter)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadGateway)
			return
		}

		var files []domain.File
		for _, file := range res {
			utils.SetFilePath(headerParams, &file)
			files = append(files, file)
		}
		j, _ := json.Marshal(map[string][]domain.File{
			"files": files,
		})
		rw.Write([]byte(j))
	}
}

func (fh *httpFileHandler) Get() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		var filter bson.M
		if query := r.URL.Query().Get("query"); query != "" {
			errQ := json.Unmarshal([]byte(query), &filter)
			if errQ != nil {
				http.Error(rw, errQ.Error(), http.StatusBadRequest)
				return
			}
		}

		res, err := fh.FileUsecase.Get(headerParams,
			r.URL.RequestURI(),
			filter,
			r.URL.Query().Get("page"),
			r.URL.Query().Get("perPage"),
			r.URL.Query().Get("sort"),
		)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadGateway)
			return
		}

		var files []domain.File
		for _, file := range res.Files {
			utils.SetFilePath(headerParams, &file)
			files = append(files, file)
		}
		res.Files = files

		j, _ := json.Marshal(res)
		rw.Write([]byte(j))
	}
}

func (fh *httpFileHandler) GetByID() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		vars := mux.Vars(r)
		objectId, _ := primitive.ObjectIDFromHex(vars["id"])
		res, err := fh.FileUsecase.GetByID(headerParams, objectId)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}

		utils.SetFilePath(headerParams, res)
		j, _ := json.Marshal(map[string]*domain.File{
			"file": res,
		})

		rw.Write([]byte(j))
	}
}

func (fh *httpFileHandler) Store() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		currentUser, _ := r.Context().Value(common.CommonContextKey("user")).(common.User)
		// uObjID, _ := primitive.ObjectIDFromHex(currentUser.Id)

		uploadInfo, _ := r.Context().Value(common.CommonContextKey("uploadInfo")).(upload.UploadInfo)
		// storage, _ := r.Context().Value(common.CommonContextKey("storage")).(int)
		file := domain.File{
			Id:           primitive.NewObjectID(),
			Path:         uploadInfo.Path,
			OriginalName: uploadInfo.OriginalName,
			Service:      headerParams.Service,
			Expire:       headerParams.Expire,
			Variants:     []string{},
			CreatedBy:    currentUser.Id,
			CreatedAt:    time.Now(),
		}
		for _, variant := range uploadInfo.Variants {
			file.Variants = append(file.Variants, variant)
		}

		// s.UpdatedBy = currentUser.Id
		// s.CreatedBy = currentUser.Id

		res, err := fh.FileUsecase.Store(headerParams, file, r.URL.Query().Get("query"))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}

		//hls and dash
		var videoConvertOptions optimization.VideoConvertOptions
		convertOptions := headerParams.VideoConvertingOptions
		errO := json.Unmarshal([]byte(convertOptions), &videoConvertOptions)
		if errO != nil {
			fmt.Println("Decode error!", errO.Error())
		}
		if videoConvertOptions.Stream {
			// go streaming.GenerateHLSStream(headerParams, file, client, cache)
			go fh.FileUsecase.GenerateStream(headerParams, file)
		}

		utils.SetFilePath(headerParams, res)

		// j, _ := json.Marshal(map[string]*domain.File{
		// 	"file": res,
		// })
		j, _ := json.Marshal(res)

		rw.Write([]byte(j))
	}
}

func (fh *httpFileHandler) Serve() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {

		vars := mux.Vars(r)
		path := filepath.Join(vars["client"], vars["year"], vars["month"], vars["filename"])
		fileExtension := filepath.Ext(vars["filename"])

		headerParams := common.ExtractHeaderParams(r)
		if headerParams.Client == "" {
			headerParams.Client = vars["client"]
		}

		if vars["backup"] != "" {
			path = filepath.Join(vars["client"], "backups", vars["backup"], vars["year"], vars["month"], vars["filename"])
		}
		// fmt.Println("path:", path, fileExtension)
		if fileExtension != ".m4s" && fileExtension != ".m3u8" && !(fileExtension == ".mp4" && strings.HasSuffix(vars["filename"], "-init.mp4")) && !(fileExtension == ".mp4" && strings.HasSuffix(vars["filename"], "_.mp4")) {

			filter := bson.M{
				"$or": bson.A{
					bson.M{"variants": path},
					bson.M{"path": path},
				},
			}

			file, err := fh.FileUsecase.GetOne(headerParams, filter)
			if err != nil {
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}
			// fmt.Println("file:", file)
			if file.Service != "" {
				reqOpt := &common.RequestParams{
					Header:  headerParams,
					Service: file.Service,
					Method:  "POST",
					Path:    "_files",
					Data: map[string]interface{}{
						"file": file,
					},
				}
				resp, err := common.CallService(reqOpt)
				if err != nil || resp.StatusCode != 200 {
					// fmt.Println("_files error:", err, resp.StatusCode)
					http.Error(rw, "Unauthorized", http.StatusUnauthorized)
					return
				}

			}

			folder := utils.UploadPath(headerParams)

			dir := filepath.Join(folder, path)
			// fmt.Println("path:", dir)

			http.ServeFile(rw, r, dir)
		} else {
			folder := utils.UploadPath(headerParams)

			dir := filepath.Join(folder, path)
			// fmt.Println("path:", dir)

			http.ServeFile(rw, r, dir)
		}

	}
}

func (fh *httpFileHandler) Delete() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		vars := mux.Vars(r)
		ObjectID, _ := primitive.ObjectIDFromHex(vars["id"])

		currentUser, _ := r.Context().Value(common.CommonContextKey("user")).(common.User)
		// uObjID, _ := primitive.ObjectIDFromHex(currentUser.Id)

		res, err := fh.FileUsecase.Delete(headerParams, ObjectID, currentUser.Id)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}

		j, _ := json.Marshal(map[string]*domain.File{
			"file": res,
		})

		rw.Write([]byte(j))
	}
}

func (fh *httpFileHandler) Update() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {

		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		vars := mux.Vars(r)
		objectId, _ := primitive.ObjectIDFromHex(vars["id"])

		var s domain.File
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&s); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		currentUser, _ := r.Context().Value(common.CommonContextKey("user")).(common.User)
		// uObjID, _ := primitive.ObjectIDFromHex(currentUser.Id)
		s.UpdatedBy = currentUser.Id

		res, err := fh.FileUsecase.Update(headerParams, objectId, s)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}

		j, _ := json.Marshal(map[string]*domain.File{
			"file": res,
		})
		rw.Write([]byte(j))
	}
}

func (fh *httpFileHandler) UpdateV2() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		fmt.Println("storage handler")
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		// var s domain.File
		// decoder := json.NewDecoder(r.Body)
		// if err := decoder.Decode(&s); err != nil {
		// 	http.Error(rw, err.Error(), http.StatusBadRequest)
		// 	return
		// }
		// defer r.Body.Close()

		currentUser, _ := r.Context().Value(common.CommonContextKey("user")).(common.User)
		// uObjID, _ := primitive.ObjectIDFromHex(currentUser.Id)

		uploadInfo, _ := r.Context().Value(common.CommonContextKey("uploadInfo")).(upload.UploadInfo)
		// storage, _ := r.Context().Value(common.CommonContextKey("storage")).(int)

		file := domain.File{
			Path:         uploadInfo.Path,
			OriginalName: uploadInfo.OriginalName,
			Variants:     []string{},
			UpdatedBy:    currentUser.Id,
			UpdatedAt:    time.Now(),
		}
		for _, variant := range uploadInfo.Variants {
			file.Variants = append(file.Variants, variant)
		}

		// body := map[string]interface{}{}

		// json.NewDecoder(r.Body).Decode(&body)
		// body["updatedBy"] = currentUser.Id
		// body["updatedAt"] = time.Now()
		// body["path"] = uploadInfo.Path
		// body["OrginalName"] = uploadInfo.OriginalName
		// variants := []string{}
		// for _, variant := range uploadInfo.Variants {
		// 	variants = append(variants, variant)
		// }
		// body["variants"] = variants
		// s.UpdatedBy = currentUser.Id
		// s.CreatedBy = currentUser.Id

		vars := mux.Vars(r)
		id := vars["id"]
		objID, idErr := primitive.ObjectIDFromHex(id)
		if idErr != nil {
			http.Error(rw, idErr.Error(), http.StatusBadRequest)
			return
		}

		// filter := bson.M{"_id": objID}

		res, err := fh.FileUsecase.Update(headerParams, objID, file)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}

		utils.SetFilePath(headerParams, res)

		j, _ := json.Marshal(map[string]*domain.File{
			"file": res,
		})
		rw.Write([]byte(j))
	}
}

func (fh *httpFileHandler) GenerateStream() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {

		rw.Header().Set("Content-type", "application/json")
		vars := mux.Vars(r)
		id := vars["id"]

		objectId, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)

		}

		headerParams := common.ExtractHeaderParams(r)

		file, err := fh.FileUsecase.GetByID(headerParams, objectId)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusNotFound)
			return
		}

		folder := utils.UploadPath(headerParams)
		dir := filepath.Join(folder, file.Path)
		_, err = os.Stat(dir)
		if err != nil {
			http.Error(rw, "file not found", http.StatusBadRequest)
			return
		}
		fmt.Println("start generate")
		go fh.FileUsecase.GenerateStream(headerParams, *file)

		rw.Write([]byte("Stream Generation started"))
	}
}

func (fh *httpFileHandler) GenerateVariants() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		folder := utils.UploadPath(headerParams)
		var docs []optimization.DocumentToRegenerate
		res, err := fh.FileUsecase.GetAll(headerParams, "/generate-variants", bson.M{"path": bson.M{"$regex": primitive.Regex{
			Pattern: "([^\\s]+(\\.(?i)(jpg|png|webp|jpeg))$)",
			Options: "i",
		}}})
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, file := range res {
			filePathToUpload := file.Path
			parts := strings.Split(filePathToUpload, "/")
			client := parts[0]
			year := parts[1]
			month := parts[2]
			filename := parts[3]
			dir := filepath.Join(client, year, month)
			destination := filepath.Join(folder, dir)
			err1 := os.MkdirAll(destination, os.ModePerm)
			if err1 != nil {
				http.Error(rw, err1.Error(), http.StatusBadRequest)
				return
			}
			dst, err := os.Create(destination + "/" + filename)
			if err != nil {
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}
			defer dst.Close()

			var p optimization.DocumentToRegenerate
			p.Variants = append(p.Variants, file.Variants...)
			p.Id = file.Id.Hex()
			p.OriginalImagePath = file.Path
			docs = append(docs, p)
		}

		var regDto optimization.VariantDto
		regDto.HeaderParams = headerParams
		regDto.Docs = docs
		fmt.Println("publishhhhhhhhhhhhhhhhhhhhhhhhhhhhhhhh regenearete eventttttttttttttttttttttttttttttttttttttttttttttttttttttt")

		env := getEnv("NATS_JETSTREAM", "false")
		if env == "false" {
			nats.Publish("files:regenerate-variants", regDto)
		} else {
			nats.PublishJet("files:regenerate-variants", regDto)
		}

		rw.Write([]byte("Done"))
	}
}

func NewHttpFileHandler(r *mux.Router, fu domain.FileUsecase) {
	handler := &httpFileHandler{
		FileUsecase: fu,
	}

	r.HandleFunc("/regenerate-variants", common.MiddlewaresHandler(
		handler.GenerateVariants(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("GET")
	r.HandleFunc("/{id}/generate-stream", common.MiddlewaresHandler(
		handler.GenerateStreamV2(),
		common.GuardMiddleware([]string{"authenticate"}),
	).ServeHTTP).Methods("GET")
	// r.HandleFunc("/{id}/generate-stream", common.MiddlewaresHandler(
	// 	handler.GenerateStream(),
	// 	common.GuardMiddleware([]string{"authenticate"}),
	// ).ServeHTTP).Methods("GET")
	r.HandleFunc("/all", handler.GetAll()).Methods("GET")
	r.HandleFunc("/", handler.Get()).Methods("GET")
	r.HandleFunc("/{id}", handler.GetByID()).Methods("GET")
	r.HandleFunc("/", common.MiddlewaresHandler(
		handler.StoreV2(),
		common.GuardMiddleware([]string{"authenticate"}),
		upload.UploadRestructionsMiddleware(),
		upload.UploadMiddlewareS3(),
	).ServeHTTP).Methods("POST")
	r.HandleFunc("/public", common.MiddlewaresHandler(
		handler.StoreV2(),
		httprate.LimitByIP(5, time.Minute),
		// upload.EnablePublicUploadMiddleware(),
		// upload.UploadMiddlewareS3(),
	).ServeHTTP).Methods("POST")

	r.HandleFunc("/file/{client}/{year}/{month}/{filename}", handler.ServeV2()).Methods("GET")
	r.HandleFunc("/file/{client}/backups/{backup}/{year}/{month}/{filename}", handler.ServeV2()).Methods("GET")
	r.HandleFunc("/{id}", common.MiddlewaresHandler(
		handler.Delete(),
		common.GuardMiddleware([]string{"authenticate", "deleteFile"}),
	).ServeHTTP).Methods("DELETE")
	r.HandleFunc("/{id}", common.MiddlewaresHandler(
		handler.Update(),
		common.GuardMiddleware([]string{"authenticate", "patchFile"}),
	).ServeHTTP).Methods("PATCH")
	r.HandleFunc("/v2/{id}", common.MiddlewaresHandler(
		handler.UpdateV3(),
		// common.GuardMiddleware([]string{"authenticate", "patchFile"}),
	).ServeHTTP).Methods("PATCH")

	// here I work
	// r.HandleFunc("/", common.MiddlewaresHandler(
	// 	handler.Store(),
	// 	common.GuardMiddleware([]string{"authenticate"}),
	// 	upload.StorageMiddleware(),
	// 	upload.UploadRestructionsMiddleware(),
	// 	upload.UploadMiddleware(),
	// ).ServeHTTP).Methods("POST")
	// r.HandleFunc("/public", common.MiddlewaresHandler(
	// 	handler.Store(),
	// 	httprate.LimitByIP(5, time.Minute),
	// 	upload.EnablePublicUploadMiddleware(),

	// 	upload.StorageMiddleware(),
	// 	// check the storage

	// 	upload.UploadRestructionsMiddleware(),
	// 	upload.UploadMiddleware(),
	// ).ServeHTTP).Methods("POST")

	// r.HandleFunc("/v2/{id}", common.MiddlewaresHandler(
	// 	handler.UpdateV2(),
	// 	common.GuardMiddleware([]string{"authenticate", "patchFile"}),
	// 	upload.StorageMiddleware(),
	// 	upload.UploadRestructionsMiddleware(),
	// 	upload.UploadMiddleware(),
	// ).ServeHTTP).Methods("PATCH")

	// new version

	// r.HandleFunc("/v2/file/{client}/{year}/{month}/{filename}", handler.ServeV2()).Methods("GET")
	// r.HandleFunc("/v2/file/{client}/backups/{backup}/{year}/{month}/{filename}", handler.Serve()).Methods("GET")

	// for test
	// r.HandleFunc("/store-s3", common.MiddlewaresHandler(
	// 	handler.StoreS3(),
	// 	common.GuardMiddleware([]string{"authenticate"}),
	// 	// upload.StorageMiddleware(),
	// 	// upload.UploadRestructionsMiddleware(),
	// 	// upload.UploadMiddleware(),
	// ).ServeHTTP).Methods("POST")

	// r.HandleFunc("/store-s3/{id}", handler.StreamFileFromS3()).Methods("GET")

}
func (fh *httpFileHandler) GenerateStreamV2() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {

		rw.Header().Set("Content-type", "application/json")
		vars := mux.Vars(r)
		id := vars["id"]

		objectId, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)

		}

		headerParams := common.ExtractHeaderParams(r)

		file, err := fh.FileUsecase.GetByID(headerParams, objectId)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusNotFound)
			return
		}

		sess, err := session.NewSession(&aws.Config{
			Region:           aws.String(region),                                         // Region (may not be needed)
			Endpoint:         aws.String(endpoint),                                       // Larsa Object Storage URL
			Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""), // Larsa Access Key & Secret Key
			S3ForcePathStyle: aws.Bool(true),                                             // Force path-style URLs (typical for S3-compatible services)
		})
		if err != nil {
			http.Error(rw, "Failed to create S3 session", http.StatusInternalServerError)
			return
		}

		// Create the S3 client
		s3Client := s3.New(sess)

		// Define GetObjectInput to fetch the file from the object storage
		fmt.Println("path get object :")
		fmt.Println(file.Path)
		input := &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(file.Path),
		}
		// rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		// Fetch the file from S3
		resp, err := s3Client.GetObject(input)
		if err != nil {
			// Log the detailed error for debugging
			log.Println("Error fetching file from S3:", err)
			http.Error(rw, "Failed to fetch file from S3", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		parts := strings.Split(file.Path, "/")
		newFileName := parts[len(parts)-1]

		fmt.Println("newFileName")
		fmt.Println(newFileName)

		folder := utils.UploadPath(headerParams)

		destination1 := filepath.Join(folder, file.Path)
		destination := filepath.Dir(destination1)
		fmt.Println("folder")
		fmt.Println(folder)

		fmt.Println("destination")
		fmt.Println(destination)
		err1 := os.MkdirAll(destination, os.ModePerm)
		if err1 != nil {
			http.Error(rw, err1.Error(), http.StatusBadRequest)
			return
		}
		dst, err := os.Create(destination + "/" + newFileName)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		defer dst.Close()

		// Copy the uploaded file to the created file on the filesystem
		if _, err := io.Copy(dst, resp.Body); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}

		dir := filepath.Join(folder, file.Path)
		_, err = os.Stat(dir)
		if err != nil {
			http.Error(rw, "file not found", http.StatusBadRequest)
			return
		}
		fmt.Println("start generate")
		go fh.FileUsecase.GenerateStreamS3(headerParams, *file)

		response := map[string]string{
			"message":     "Stream Generation started",
			"destination": destination,
		}

		// Write the response as JSON
		rw.WriteHeader(http.StatusOK)
		jsonResponse, _ := json.Marshal(response)
		rw.Write(jsonResponse)

		// rw.Write([]byte("Stream Generation started"))
	}
}
func (fh *httpFileHandler) ServeV2() http.HandlerFunc {

	return func(rw http.ResponseWriter, r *http.Request) {
		// fmt.Println("ddddddddddd: hiiiiiiiiiiiiiii")
		// var filsKey []string
		// filsKey = append(filsKey,"604674966fe156529e3e03e9/2025/1/f10d62ea-7ef7-4511-9c62-264bbc9f202b.png")
		// filsKey = append(filsKey,"604674966fe156529e3e03e9/2025/1/f10d62ea-7ef7-4511-9c62-264bbc9f202b.png")
		// filsKey = append(filsKey,"604674966fe156529e3e03e9/2025/1/ad02a04e-a8ea-4676-a6ec-bd6084eca749.jpg")

		// uploadS3.CalculateSize(filsKey)
		vars := mux.Vars(r)
		path := filepath.Join(vars["client"], vars["year"], vars["month"], vars["filename"])
		fileExtension := filepath.Ext(vars["filename"])

		headerParams := common.ExtractHeaderParams(r)
		if headerParams.Client == "" {
			headerParams.Client = vars["client"]
		}

		if vars["backup"] != "" {
			path = filepath.Join(vars["client"], "backups", vars["backup"], vars["year"], vars["month"], vars["filename"])
		}

		filterInfo := bson.M{
			"$or": bson.A{
				bson.M{"variants": path},
				bson.M{"path": path},
			},
		}
		fmt.Println("path")
		fmt.Println(path)
		fmt.Println("filterInfo")
		fmt.Println(filterInfo)

		fileInfo, err := fh.FileUsecase.GetOne(headerParams, filterInfo)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}

		if fileInfo.Service != "" {
			reqOpt := &common.RequestParams{
				Header:  headerParams,
				Service: fileInfo.Service,
				Method:  "POST",
				Path:    "_files",
				Data: map[string]interface{}{
					"file": fileInfo,
				},
			}
			resp, err := common.CallService(reqOpt)
			if err != nil || resp.StatusCode != 200 {
				// fmt.Println("_files error:", err, resp.StatusCode)
				http.Error(rw, "Unauthorized", http.StatusUnauthorized)
				return
			}

		}

		fmt.Println("fileInfo")
		fmt.Println(filterInfo)
		fmt.Println("fileInfo.path :")
		fmt.Println(fileInfo.Path)
		fmt.Println("fileInfo.isS3 :")
		fmt.Println(fileInfo.IsS3)
		if fileInfo.IsS3 {
			fmt.Println("get from s3")

			// Create the S3 session
			sess, err := session.NewSession(&aws.Config{
				Region:           aws.String(region),                                         // Region (may not be needed)
				Endpoint:         aws.String(endpoint),                                       // Larsa Object Storage URL
				Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""), // Larsa Access Key & Secret Key
				S3ForcePathStyle: aws.Bool(true),                                             // Force path-style URLs (typical for S3-compatible services)
			})
			if err != nil {
				http.Error(rw, "Failed to create S3 session", http.StatusInternalServerError)
				return
			}

			// Create the S3 client
			s3Client := s3.New(sess)

			// Define GetObjectInput to fetch the file from the object storage
			fmt.Println("path get object :")
			fmt.Println(path)
			input := &s3.GetObjectInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String(path),
			}
			// rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			// Fetch the file from S3
			resp, err := s3Client.GetObject(input)
			if err != nil {
				// Log the detailed error for debugging
				log.Println("Error fetching file from S3:", err)
				http.Error(rw, "Failed to fetch file from S3", http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			// Set Content-Type based on requested file name (ensures SVG is image/svg+xml)
			contentType := utils.DetectContentTypeFromName(vars["filename"])
			rw.Header().Set("Content-Type", contentType)

			// w.Header().Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
			rw.WriteHeader(http.StatusOK)

			// Stream the file content back to the client
			_, err = io.Copy(rw, resp.Body)
			if err != nil {
				http.Error(rw, "Failed to stream file", http.StatusInternalServerError)
				return
			}
			return
		}
		fmt.Println("get from file system")

		// fmt.Println("path:", path, fileExtension)
		if fileExtension != ".m4s" && fileExtension != ".m3u8" && !(fileExtension == ".mp4" && strings.HasSuffix(vars["filename"], "-init.mp4")) && !(fileExtension == ".mp4" && strings.HasSuffix(vars["filename"], "_.mp4")) {

			filter := bson.M{
				"$or": bson.A{
					bson.M{"variants": path},
					bson.M{"path": path},
				},
			}

			file, err := fh.FileUsecase.GetOne(headerParams, filter)
			if err != nil {
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}
			// fmt.Println("file:", file)
			if file.Service != "" {
				reqOpt := &common.RequestParams{
					Header:  headerParams,
					Service: file.Service,
					Method:  "POST",
					Path:    "_files",
					Data: map[string]interface{}{
						"file": file,
					},
				}
				resp, err := common.CallService(reqOpt)
				if err != nil || resp.StatusCode != 200 {
					// fmt.Println("_files error:", err, resp.StatusCode)
					http.Error(rw, "Unauthorized", http.StatusUnauthorized)
					return
				}

			}

			folder := utils.UploadPath(headerParams)

			dir := filepath.Join(folder, path)
			// fmt.Println("path:", dir)

			http.ServeFile(rw, r, dir)
		} else {
			folder := utils.UploadPath(headerParams)

			dir := filepath.Join(folder, path)
			// fmt.Println("path:", dir)

			http.ServeFile(rw, r, dir)
		}

	}
}
func (fh *httpFileHandler) StoreV2() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		fmt.Println("hiiiiiiiiiiiiiiiiiiiiiiiiii")
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		currentUser, _ := r.Context().Value(common.CommonContextKey("user")).(common.User)
		// here
		uploadInfo, _ := r.Context().Value(common.CommonContextKey("uploadInfo")).(upload.UploadInfo)

		// step 1 check size
		err := r.ParseMultipartForm(10 << 20) // 10 MB limit
		if err != nil {
			http.Error(rw, "Unable to parse multipart form", http.StatusBadRequest)
			return
		}

		// step 2 get file from body
		fileForm, handler, err := r.FormFile("file") // "file" is the key for the uploaded file
		if err != nil {
			http.Error(rw, "Unable to retrieve file from form-data", http.StatusBadRequest)
			return
		}
		defer fileForm.Close()

		// step 3 create path
		Client := headerParams.Client
		t := time.Now()
		year := t.Year()
		month := int(t.Month())
		dir := filepath.Join(Client, strconv.Itoa(year), strconv.Itoa(month))

		ext := filepath.Ext(handler.Filename)
		newFileName := fmt.Sprintf("%s%s", uuid.NewString(), ext)

		filePathVariable := filepath.Join(dir, newFileName)
		originalName := handler.Filename
		fmt.Println("originalName")
		fmt.Println(originalName)

		// step 4 read file data
		fileData, err := io.ReadAll(fileForm)
		if err != nil {
			http.Error(rw, "Unable to read file data", http.StatusInternalServerError)
			return
		}

		// step 6 Create a unique S3 object key
		objectKey := filePathVariable
		fmt.Println("objectKey")
		fmt.Println(objectKey)

		uploadS3.StoreFileS3(fileData, objectKey)

		file := domain.File{
			Id:           primitive.NewObjectID(),
			Path:         filePathVariable,
			OriginalName: originalName,
			Service:      headerParams.Service,
			Expire:       headerParams.Expire,
			Variants:     []string{},
			CreatedBy:    currentUser.Id,
			CreatedAt:    time.Now(),
			IsS3:         true,
		}
		fmt.Println("file")
		fmt.Println(file)

		for _, variant := range uploadInfo.Variants {
			file.Variants = append(file.Variants, variant)
		}

		// variants := optimization.OptimizeV2(headerParams, filePath)
		// fmt.Println("variantsssssssss")

		// for _, variant := range variants {
		// 	fmt.Println("variant")
		// 	fmt.Println(variant)
		// 	file.Variants = append(file.Variants, variant)
		// }

		res, err := fh.FileUsecase.Store(headerParams, file, r.URL.Query().Get("query"))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Println("res")
		fmt.Println(res)
		// for video only
		var videoConvertOptions optimization.VideoConvertOptions
		convertOptions := headerParams.VideoConvertingOptions
		errO := json.Unmarshal([]byte(convertOptions), &videoConvertOptions)
		if errO != nil {
			fmt.Println("Decode error!", errO.Error())
		}
		if videoConvertOptions.Stream {
			// go streaming.GenerateHLSStream(headerParams, file, client, cache)
			go fh.FileUsecase.GenerateStreamS3(headerParams, file)
		}

		utils.SetFilePath(headerParams, res)
		fmt.Println("res 2")
		fmt.Println(res)
		j, _ := json.Marshal(res)

		rw.Write([]byte(j))
	}
}
func (fh *httpFileHandler) UpdateV3() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		fmt.Println("storage handler v3")
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		currentUser, _ := r.Context().Value(common.CommonContextKey("user")).(common.User)

		fileForm, handler, err := r.FormFile("file") // "file" is the key for the uploaded file
		if err != nil {
			http.Error(rw, "Unable to retrieve file from form-data", http.StatusBadRequest)
			return
		}
		defer fileForm.Close()

		// step 3 create path
		Client := headerParams.Client
		t := time.Now()
		year := t.Year()
		month := int(t.Month())
		dir := filepath.Join(Client, strconv.Itoa(year), strconv.Itoa(month))

		ext := filepath.Ext(handler.Filename)
		newFileName := fmt.Sprintf("%s%s", uuid.NewString(), ext)

		filePath := filepath.Join(dir, newFileName)
		originalName := handler.Filename

		fileData, err := io.ReadAll(fileForm)
		if err != nil {
			http.Error(rw, "Unable to read file data", http.StatusInternalServerError)
			return
		}
		sess, err := session.NewSession(&aws.Config{
			Region:           aws.String(region),                                         // Use any region (not strictly necessary for Larsa)
			Endpoint:         aws.String(endpoint),                                       // Larsa Object Storage URL
			Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""), // Larsa Access Key & Secret Key
			S3ForcePathStyle: aws.Bool(true),                                             // Force path-style URLs (typical for S3-compatible services)
		})
		if err != nil {
			http.Error(rw, "error in session", http.StatusForbidden)
			return
		}
		s3Client := s3.New(sess)

		// step 6 Create a unique S3 object key
		objectKey := filePath

		// step 7 send file to s3
		contentType := utils.DetectContentTypeFromName(objectKey)

		_, err = s3Client.PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(bucketName),
			Key:         aws.String(objectKey),
			ContentType: aws.String(contentType),
			Body:        bytes.NewReader(fileData),
			ACL:         aws.String("public-read"), // You can adjust the ACL as needed
		})
		if err != nil {
			http.Error(rw, "error in send to s3", http.StatusForbidden)
			return
		}

		file := domain.File{
			Path:         filePath,
			OriginalName: originalName,
			Variants:     []string{},
			UpdatedBy:    currentUser.Id,
			UpdatedAt:    time.Now(),
			IsS3:         true,
		}
		// variants := optimization.OptimizeV2(headerParams, filePath)
		// for _, variant := range variants {
		// 	file.Variants = append(file.Variants, variant)
		// }

		vars := mux.Vars(r)
		id := vars["id"]
		objID, idErr := primitive.ObjectIDFromHex(id)
		if idErr != nil {
			http.Error(rw, idErr.Error(), http.StatusBadRequest)
			return
		}

		res, err := fh.FileUsecase.Update(headerParams, objID, file)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Println(res)
		utils.SetFilePath(headerParams, res)

		j, _ := json.Marshal(map[string]*domain.File{
			"file": res,
		})
		rw.Write([]byte(j))
	}
}

func (fh *httpFileHandler) StoreS3() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		// Retrieve the current user from context
		currentUser, _ := r.Context().Value(common.CommonContextKey("user")).(common.User)

		// Parse the multipart form (if the file is sent using `multipart/form-data`)
		err := r.ParseMultipartForm(10 << 20) // 10 MB limit
		if err != nil {
			http.Error(rw, "Unable to parse multipart form", http.StatusBadRequest)
			return
		}

		// Retrieve the file from the form-data
		file, header, err := r.FormFile("file") // "file" is the key for the uploaded file
		if err != nil {
			http.Error(rw, "Unable to retrieve file from form-data", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Create a File object
		fileObject := domain.File{
			Id:           primitive.NewObjectID(),
			OriginalName: header.Filename, // Get the original filename from the header
			Service:      headerParams.Service,
			Expire:       headerParams.Expire,
			CreatedBy:    currentUser.Id,
			CreatedAt:    time.Now(),
		}

		// Read the file data
		fileData, err := io.ReadAll(file)
		if err != nil {
			http.Error(rw, "Unable to read file data", http.StatusInternalServerError)
			return
		}

		// Get the bucket name from the query parameter
		// bucketName := r.URL.Query().Get("bucket")

		// Call Usecase for S3 storage
		res, err := fh.FileUsecase.StoreS3(headerParams, fileObject, fileData, "bucketName")
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Respond with the result
		j, _ := json.Marshal(res)
		rw.WriteHeader(http.StatusOK)
		rw.Write(j)
	}
}

func (fh *httpFileHandler) StreamFileFromS3() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract parameters from URL (assuming you are using Gorilla Mux)
		vars := mux.Vars(r)
		fileID := vars["id"]

		// Assuming that "fileID" will be used as the object key in your bucket
		// For example, you might need to construct the full key using the fileID
		bucketName := "bucketName" // replace with the actual bucket name
		key := fmt.Sprintf("%s/%s", "604674966fe156529e3e03e9/", fileID)

		// Initialize S3 session
		sess, err := session.NewSession(&aws.Config{
			Region:           aws.String("us-east-1"),                                                                                  // Region (may not be needed)
			Endpoint:         aws.String("https://prod-bucket-larsa.nbg1.your-objectstorage.com"),                                      // Larsa Object Storage URL
			Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""), // Larsa Access Key & Secret Key
			S3ForcePathStyle: aws.Bool(true),                                                                                           // Force path-style URLs (typical for S3-compatible services)
		})
		if err != nil {
			http.Error(w, "Failed to create S3 session", http.StatusInternalServerError)
			return
		}

		// Create the S3 client
		s3Client := s3.New(sess)

		// Define GetObjectInput to fetch the file from the object storage
		input := &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		}

		// Fetch the file from S3
		resp, err := s3Client.GetObject(input)
		if err != nil {
			// Log the detailed error for debugging
			log.Println("Error fetching file from S3:", err)
			http.Error(w, "Failed to fetch file from S3", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		// Set the headers for the response
		// w.Header().Set("Content-Type", "application/octet-stream")
		// w.Header().Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
		w.WriteHeader(http.StatusOK)

		// Stream the file content back to the client
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			http.Error(w, "Failed to stream file", http.StatusInternalServerError)
			return
		}
	}
}
func (fh *httpFileHandler) GenerateVariantsV2() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		var docs []optimization.DocumentToRegenerate
		res, err := fh.FileUsecase.GetAll(headerParams, "/generate-variants", bson.M{"path": bson.M{"$regex": primitive.Regex{
			Pattern: "([^\\s]+(\\.(?i)(jpg|png|webp|jpeg))$)",
			Options: "i",
		}}})
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, file := range res {
			var p optimization.DocumentToRegenerate
			p.Variants = append(p.Variants, file.Variants...)
			p.Id = file.Id.Hex()
			p.OriginalImagePath = file.Path
			docs = append(docs, p)
		}

		var regDto optimization.VariantDto
		regDto.HeaderParams = headerParams
		regDto.Docs = docs

		env := getEnv("NATS_JETSTREAM", "false")
		if env == "false" {
			nats.Publish("files:regenerate-variants", regDto)
		} else {
			nats.PublishJet("files:regenerate-variants", regDto)
		}

		rw.Write([]byte("Done"))
	}
}

// sess, err := session.NewSession(&aws.Config{
// 	Region:           aws.String(region),                                         // Region (may not be needed)
// 	Endpoint:         aws.String(endpoint),                                       // Larsa Object Storage URL
// 	Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""), // Larsa Access Key & Secret Key
// 	S3ForcePathStyle: aws.Bool(true),                                             // Force path-style URLs (typical for S3-compatible services)
// })
// if err != nil {
// 	http.Error(rw, "Failed to create S3 session", http.StatusInternalServerError)
// 	return
// }

// // Create the S3 client
// s3Client := s3.New(sess)

// // Define GetObjectInput to fetch the file from the object storage
// fmt.Println("path get object :")
// fmt.Println(file.Path)
// input := &s3.GetObjectInput{
// 	Bucket: aws.String(bucketName),
// 	Key:    aws.String(file.Path),
// }
// // rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
// // Fetch the file from S3
// resp, err := s3Client.GetObject(input)
// if err != nil {
// 	// Log the detailed error for debugging
// 	log.Println("Error fetching file from S3:", err)
// 	http.Error(rw, "Failed to fetch file from S3", http.StatusInternalServerError)
// 	return
// }
// defer resp.Body.Close()

// parts := strings.Split(file.Path, "/")
// newFileName := parts[len(parts)-1]
