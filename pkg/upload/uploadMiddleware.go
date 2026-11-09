package upload

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"larsa-files/pkg/optimization"
	"larsa-files/utils"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/google/uuid"
)

type UploadInfo struct {
	Path         string
	OriginalName string
	Size         int64
	Variants     []string
}

func UploadMiddleware() common.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			// fmt.Println("upload middleware")
			headerParams := common.ExtractHeaderParams(r)

			uploadrest, _ := r.Context().Value(common.CommonContextKey("uploadRestructions")).(Restrictions)

			folder := utils.UploadPath(headerParams)

			Client := headerParams.Client
			t := time.Now()
			year := t.Year()
			month := int(t.Month())

			dir := filepath.Join(Client, strconv.Itoa(year), strconv.Itoa(month))

			// if the service is backup update dir to the backups folders
			if headerParams.Service == "backup" {
				dir = filepath.Join(Client, "backups", "databases", strconv.Itoa(year), strconv.Itoa(month))
			}

			// Maximum upload of 10 MB files
			r.ParseMultipartForm(10 << 20)
			// Get handler for filename, size and headers
			file, handler, err := r.FormFile("file")
			if err != nil {
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}
			defer file.Close()

			// check for restructions
			if !uploadrest.isEmpty {
				num := uploadrest.Limits.FileMaxSize.Value
				unit := uploadrest.Limits.FileMaxSize.Unit
				size := fmt.Sprintf("%d%s", num, unit)
				bytes, err := utils.GetBytes(size)
				if err != nil {
					http.Error(rw, err.Error(), http.StatusBadRequest)
					return
				}

				if utils.IntegerGreater(handler.Size, bytes) {
					http.Error(rw, "file size exceed the limits", http.StatusBadRequest)
					return
				}

				// mime type check based on detected content type from filename
				acceptableExts := uploadrest.Limits.FileFormats
				detectedContentType := utils.DetectContentTypeFromName(handler.Filename)
				_, found := utils.Find(detectedContentType, acceptableExts)
				if !found {
					http.Error(rw, "not in format file", http.StatusBadRequest)
					return
				}

			}

			// Create file
			ext := filepath.Ext(handler.Filename)
			newFileName := fmt.Sprintf("%s%s", uuid.NewString(), ext)
			destination := filepath.Join(folder, dir)

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
			if _, err := io.Copy(dst, file); err != nil {
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}

			fileSource := filepath.Join(destination, newFileName)
			variants := optimization.Optimize(headerParams, fileSource)
			go optimization.VideoOptimization(headerParams, fileSource)

			path := filepath.Join(dir, newFileName)
			originalName := handler.Filename

			uploadInfo := UploadInfo{Path: path, OriginalName: originalName, Size: handler.Size, Variants: variants}
			ctx := context.WithValue(r.Context(), common.CommonContextKey("uploadInfo"), uploadInfo)
			next.ServeHTTP(rw, r.Clone(ctx))
		})
	}
}


func UploadMiddlewareS3() common.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			// fmt.Println("upload middleware")
			headerParams := common.ExtractHeaderParams(r)

			uploadrest, _ := r.Context().Value(common.CommonContextKey("uploadRestructions")).(Restrictions)

			folder := utils.UploadPath(headerParams)

			Client := headerParams.Client
			t := time.Now()
			year := t.Year()
			month := int(t.Month())

			dir := filepath.Join(Client, strconv.Itoa(year), strconv.Itoa(month))

			// if the service is backup update dir to the backups folders
			if headerParams.Service == "backup" {
				dir = filepath.Join(Client, "backups", "databases", strconv.Itoa(year), strconv.Itoa(month))
			}

			// Maximum upload of 10 MB files
			r.ParseMultipartForm(10 << 20)
			// Get handler for filename, size and headers
			file, handler, err := r.FormFile("file")
			if err != nil {
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}
			defer file.Close()

			// check for restructions
			if !uploadrest.isEmpty {
				num := uploadrest.Limits.FileMaxSize.Value
				unit := uploadrest.Limits.FileMaxSize.Unit
				size := fmt.Sprintf("%d%s", num, unit)
				bytes, err := utils.GetBytes(size)
				if err != nil {
					http.Error(rw, err.Error(), http.StatusBadRequest)
					return
				}

				if utils.IntegerGreater(handler.Size, bytes) {
					http.Error(rw, "file size exceed the limits", http.StatusBadRequest)
					return
				}

				// mime type check based on detected content type from filename
				acceptableExts := uploadrest.Limits.FileFormats
				detectedContentType := utils.DetectContentTypeFromName(handler.Filename)
				_, found := utils.Find(detectedContentType, acceptableExts)
				if !found {
					http.Error(rw, "not in format file", http.StatusBadRequest)
					return
				}

			}

			// Create file
			ext := filepath.Ext(handler.Filename)
			newFileName := fmt.Sprintf("%s%s", uuid.NewString(), ext)
			destination := filepath.Join(folder, dir)

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
			if _, err := io.Copy(dst, file); err != nil {
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}

			fileSource := filepath.Join(destination, newFileName)
			variants := optimization.OptimizeS3(headerParams, fileSource)
			go optimization.VideoOptimizationS3(headerParams, fileSource)

			path := filepath.Join(dir, newFileName)
			originalName := handler.Filename

			uploadInfo := UploadInfo{Path: path, OriginalName: originalName, Size: handler.Size, Variants: variants}
			ctx := context.WithValue(r.Context(), common.CommonContextKey("uploadInfo"), uploadInfo)
			next.ServeHTTP(rw, r.Clone(ctx))
		})
	}
}