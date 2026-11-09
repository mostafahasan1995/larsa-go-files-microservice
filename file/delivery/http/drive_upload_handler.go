package file

import (
	"encoding/json"
	"fmt"
	"io"
	"larsa-files/domain"
	driveUsecase "larsa-files/file/usecase"
	"larsa-files/pkg/uploadS3"
	"larsa-files/utils"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"mime/multipart"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type driveUploadHandler struct {
	DriveUsecase *driveUsecase.DriveUsecase
}

func NewDriveUploadHandler(driveUsecase *driveUsecase.DriveUsecase) *driveUploadHandler {
	return &driveUploadHandler{
		DriveUsecase: driveUsecase,
	}
}

// UploadFileToDrive uploads a file to the drive system with folder support
func (duh *driveUploadHandler) UploadFileToDrive() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		currentUser, _ := r.Context().Value(common.CommonContextKey("user")).(common.User)

		// Parse multipart form
		err := r.ParseMultipartForm(10 << 20) // 10 MB limit
		if err != nil {
			http.Error(rw, "Unable to parse multipart form", http.StatusBadRequest)
			return
		}

		// Get file from form
		fileForm, handler, err := r.FormFile("file")
		if err != nil {
			http.Error(rw, "Unable to retrieve file from form-data", http.StatusBadRequest)
			return
		}
		defer fileForm.Close()

		// Get folder ID from form
		var folderID *primitive.ObjectID
		if folderIDStr := r.FormValue("folderId"); folderIDStr != "" {
			if id, err := primitive.ObjectIDFromHex(folderIDStr); err == nil {
				folderID = &id
			}
		}
		var isProduct bool
		if isProductStr := r.FormValue("isProduct"); isProductStr != "" {
			isProduct = isProductStr == "true"
		}
		// Create file path
		Client := headerParams.Client
		t := time.Now()
		year := t.Year()
		month := int(t.Month())
		dir := filepath.Join(Client, strconv.Itoa(year), strconv.Itoa(month))

		// Sanitize original filename (avoid path traversal)
		safeFilename := filepath.Base(handler.Filename)
		ext := filepath.Ext(safeFilename)
		newFileName := fmt.Sprintf("%s%s", uuid.NewString(), ext)
		filePathVariable := filepath.Join(dir, newFileName)
		originalName := safeFilename

		// Read file data
		fileData, err := io.ReadAll(fileForm)
		if err != nil {
			http.Error(rw, "Unable to read file data", http.StatusInternalServerError)
			return
		}

		// Upload to S3
		objectKey := filePathVariable
		err = uploadS3.StoreFileS3(fileData, objectKey)
		if err != nil {
			http.Error(rw, "Failed to upload file to S3", http.StatusInternalServerError)
			return
		}

		// Create file record
		file := domain.File{
			Id:           primitive.NewObjectID(),
			Path:         filePathVariable,
			FolderId:     folderID,
			IsProduct:    isProduct,
			Size:         int64(len(fileData)),
			MimeType:     handler.Header.Get("Content-Type"),
			OriginalName: originalName,
			Service:      headerParams.Service,
			Type:         "drive", // Mark as drive file
			Expire:       headerParams.Expire,
			Variants:     []string{},
			CreatedBy:    currentUser.Id,
			UpdatedBy:    currentUser.Id,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			IsS3:         true,
		}

		// Store file in database
		userID := getCurrentUserID(r)
		res, err := duh.DriveUsecase.UploadFile(headerParams, file, folderID, userID)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set file path for response
		utils.SetDriveFilePath(headerParams, res)

		json.NewEncoder(rw).Encode(map[string]*domain.File{
			"file": res,
		})
	}
}

// UploadMultipleFilesToDrive uploads multiple files to the drive system
func (duh *driveUploadHandler) UploadMultipleFilesToDrive() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		currentUser := common.User{Id: "system"} // placeholder

		err := r.ParseMultipartForm(100 << 20) // 100 MB limit
		if err != nil {
			http.Error(rw, "Unable to parse multipart form", http.StatusBadRequest)
			return
		}

		// Accept files field: files or files[]
		fileFields := [][]*multipart.FileHeader{}
		if formFiles, ok := r.MultipartForm.File["files"]; ok {
			fileFields = append(fileFields, formFiles)
		}
		if formFiles, ok := r.MultipartForm.File["files[]"]; ok {
			fileFields = append(fileFields, formFiles)
		}

		var folderID *primitive.ObjectID
		if folderIDStr := r.FormValue("folderId"); folderIDStr != "" {
			if id, err := primitive.ObjectIDFromHex(folderIDStr); err == nil {
				folderID = &id
			}
		}
		var isProduct bool
		if isProductStr := r.FormValue("isProduct"); isProductStr != "" {
			isProduct = isProductStr == "true"
		}
		uploadedFiles := []*domain.File{}
		errors := []string{}
		for _, fields := range fileFields {
			for _, fileHeader := range fields {
				file, err := fileHeader.Open()
				if err != nil {
					errors = append(errors, "Open:"+fileHeader.Filename+":"+err.Error())
					continue
				}
				fileData, err := io.ReadAll(file)
				file.Close()
				if err != nil {
					errors = append(errors, "Read:"+fileHeader.Filename+":"+err.Error())
					continue
				}

				// Build object key like single upload
				Client := headerParams.Client
				t := time.Now()
				year := t.Year()
				month := int(t.Month())
				dir := filepath.Join(Client, strconv.Itoa(year), strconv.Itoa(month))
				safeFilename := filepath.Base(fileHeader.Filename)
				ext := filepath.Ext(safeFilename)
				newFileName := fmt.Sprintf("%s%s", uuid.NewString(), ext)
				objectKey := filepath.Join(dir, newFileName)

				// Upload to S3
				if err := uploadS3.StoreFileS3(fileData, objectKey); err != nil {
					errors = append(errors, "S3:"+fileHeader.Filename+":"+err.Error())
					continue
				}

				fileRecord := domain.File{
					OriginalName: safeFilename,
					Path:         objectKey,
					FolderId:     folderID,
					IsProduct:    isProduct,
					Size:         int64(len(fileData)),
					MimeType:     fileHeader.Header.Get("Content-Type"),
					Service:      headerParams.Service,
					Type:         "drive", // Mark as drive file
					Variants:     []string{},
					CreatedBy:    currentUser.Id,
					UpdatedBy:    currentUser.Id,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
					IsS3:         true,
				}
				userID := getCurrentUserID(r)
				res, err := duh.DriveUsecase.UploadFile(headerParams, fileRecord, folderID, userID)
				if err != nil {
					errors = append(errors, "Store:"+fileHeader.Filename+":"+err.Error())
					continue
				}
				// Set proper file path for serving
				utils.SetDriveFilePath(headerParams, res)
				uploadedFiles = append(uploadedFiles, res)
			}
		}
		json.NewEncoder(rw).Encode(map[string]interface{}{
			"files":  uploadedFiles,
			"errors": errors,
		})
	}
}

// GetFileInfo gets file information for the drive interface
func (duh *driveUploadHandler) GetFileInfo() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_ = common.ExtractHeaderParams(r)

		// This would return file information formatted for the drive interface
		// including file type icons, size formatting, etc.
		http.Error(rw, "File info endpoint not implemented", http.StatusNotImplemented)
	}
}
