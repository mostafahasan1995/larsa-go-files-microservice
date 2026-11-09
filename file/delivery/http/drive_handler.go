package file

import (
	"encoding/json"
	"larsa-files/domain"
	driveUsecase "larsa-files/file/usecase"
	"larsa-files/utils"
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type driveHandler struct {
	DriveUsecase *driveUsecase.DriveUsecase
}

func NewDriveHandler(driveUsecase *driveUsecase.DriveUsecase) *driveHandler {
	return &driveHandler{
		DriveUsecase: driveUsecase,
	}
}

// getCurrentUserID extracts the user ID from the request context
func getCurrentUserID(r *http.Request) string {
	currentUser, ok := r.Context().Value(common.CommonContextKey("user")).(common.User)
	if !ok || currentUser.Id == "" {
		return "system"
	}
	return currentUser.Id
}

// Folder Management Handlers
func (dh *driveHandler) CreateFolder() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		var request struct {
			Name     string `json:"name"`
			ParentId string `json:"parentId,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(rw, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Basic validation
		request.Name = strings.TrimSpace(request.Name)
		if request.Name == "" {
			http.Error(rw, "Folder name cannot be empty", http.StatusBadRequest)
			return
		}

		var parentID *primitive.ObjectID
		if request.ParentId != "" {
			id, err := primitive.ObjectIDFromHex(request.ParentId)
			if err != nil {
				http.Error(rw, "Invalid parentId", http.StatusBadRequest)
				return
			}
			parentID = &id
		}

		userID := getCurrentUserID(r)
		folder, err := dh.DriveUsecase.CreateFolder(headerParams, request.Name, parentID, userID)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(rw).Encode(map[string]*domain.Folder{
			"folder": folder,
		})
	}
}

func (dh *driveHandler) GetFolder() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		vars := mux.Vars(r)

		id, err := primitive.ObjectIDFromHex(vars["id"])
		if err != nil {
			http.Error(rw, "Invalid folder ID", http.StatusBadRequest)
			return
		}

		folder, err := dh.DriveUsecase.GetFolder(headerParams, id)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusNotFound)
			return
		}

		json.NewEncoder(rw).Encode(map[string]*domain.Folder{
			"folder": folder,
		})
	}
}

func (dh *driveHandler) UpdateFolder() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		vars := mux.Vars(r)

		id, err := primitive.ObjectIDFromHex(vars["id"])
		if err != nil {
			http.Error(rw, "Invalid folder ID", http.StatusBadRequest)
			return
		}

		var request struct {
			Name string `json:"name"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(rw, "Invalid request body", http.StatusBadRequest)
			return
		}

		request.Name = strings.TrimSpace(request.Name)
		if request.Name == "" {
			http.Error(rw, "Folder name cannot be empty", http.StatusBadRequest)
			return
		}

		userID := getCurrentUserID(r)
		folder, err := dh.DriveUsecase.UpdateFolder(headerParams, id, request.Name, userID)
		if err != nil {
			http.Error(rw, err.Error(), utils.GetStatusCode(err))
			return
		}

		json.NewEncoder(rw).Encode(map[string]*domain.Folder{
			"folder": folder,
		})
	}
}

func (dh *driveHandler) UpdateFolderProtection() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		vars := mux.Vars(r)

		id, err := primitive.ObjectIDFromHex(vars["id"])
		if err != nil {
			http.Error(rw, "Invalid folder ID", http.StatusBadRequest)
			return
		}

		var request struct {
			Deletable *bool `json:"isDeletable"`
			Movable   *bool `json:"isMovable"`
			Copyable  *bool `json:"isCopyable"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(rw, "Invalid request body", http.StatusBadRequest)
			return
		}

		userID := getCurrentUserID(r)
		folder, err := dh.DriveUsecase.UpdateFolderProtection(headerParams, id, request.Deletable, request.Movable, request.Copyable, userID)
		if err != nil {
			http.Error(rw, err.Error(), utils.GetStatusCode(err))
			return
		}

		if folder == nil {
			// If no updates were performed, fetch the current folder state
			folder, err = dh.DriveUsecase.GetFolder(headerParams, id)
			if err != nil {
				http.Error(rw, err.Error(), http.StatusNotFound)
				return
			}
		}

		json.NewEncoder(rw).Encode(map[string]*domain.Folder{
			"folder": folder,
		})
	}
}

func (dh *driveHandler) DeleteFolder() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		vars := mux.Vars(r)

		id, err := primitive.ObjectIDFromHex(vars["id"])
		if err != nil {
			http.Error(rw, "Invalid folder ID", http.StatusBadRequest)
			return
		}

		userID := getCurrentUserID(r)
		folder, err := dh.DriveUsecase.DeleteFolder(headerParams, id, userID)
		if err != nil {
			http.Error(rw, err.Error(), utils.GetStatusCode(err))
			return
		}

		json.NewEncoder(rw).Encode(map[string]*domain.Folder{
			"folder": folder,
		})
	}
}

func (dh *driveHandler) EmptyTrash() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		count, err := dh.DriveUsecase.EmptyTrash(headerParams)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(rw).Encode(map[string]interface{}{
			"success":      true,
			"deletedCount": count,
			"message":      "Trash emptied successfully",
		})
	}
}

// Navigation and Browsing Handlers
func (dh *driveHandler) GetFolderContents() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		vars := mux.Vars(r)

		var folderID *primitive.ObjectID
		if idStr, exists := vars["id"]; exists && idStr != "" {
			if id, err := primitive.ObjectIDFromHex(idStr); err == nil {
				folderID = &id
			}
		}

		// Parse query parameters
		query := make(bson.M)
		if searchQuery := r.URL.Query().Get("search"); searchQuery != "" {
			query["$or"] = []bson.M{
				{"originalName": bson.M{"$regex": searchQuery, "$options": "i"}},
				{"name": bson.M{"$regex": searchQuery, "$options": "i"}},
			}
		}

		page := r.URL.Query().Get("page")
		perPage := r.URL.Query().Get("perPage")

		contents, err := dh.DriveUsecase.GetFolderContents(headerParams, folderID, query, page, perPage)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Always return a valid response, even if empty
		if contents == nil {
			contents = &domain.DrivePagination{
				Items: []domain.DriveItem{},
				Pagination: common.Pagination{
					TotalPages: 0,
					PerPage:    10,
					TotalCount: 0,
				},
			}
		} else {
			// Set proper file paths for all file items in the contents
			for _, item := range contents.Items {
				if file, ok := item.(*domain.File); ok {
					utils.SetDriveFilePath(headerParams, file)
				}
			}
		}

		json.NewEncoder(rw).Encode(contents)
	}
}

// Debug endpoint to test server connectivity
func (dh *driveHandler) HealthCheck() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(map[string]string{
			"status":  "ok",
			"message": "Drive service is running",
		})
	}
}

func (dh *driveHandler) GetRecentItems() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		limit := 10
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		userID := getCurrentUserID(r)
		items, err := dh.DriveUsecase.GetRecentItems(headerParams, limit, userID)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set proper file paths for all file items
		for _, item := range items {
			if file, ok := item.(*domain.File); ok {
				utils.SetDriveFilePath(headerParams, file)
			}
		}

		json.NewEncoder(rw).Encode(map[string][]domain.DriveItem{
			"items": items,
		})
	}
}

func (dh *driveHandler) SearchItems() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		searchQuery := r.URL.Query().Get("q")
		if searchQuery == "" {
			http.Error(rw, "Search query is required", http.StatusBadRequest)
			return
		}

		itemType := r.URL.Query().Get("type")
		if itemType == "" {
			itemType = "all"
		}

		var folderID *primitive.ObjectID
		if folderIDStr := r.URL.Query().Get("folderId"); folderIDStr != "" {
			if id, err := primitive.ObjectIDFromHex(folderIDStr); err == nil {
				folderID = &id
			}
		}

		items, err := dh.DriveUsecase.SearchItems(headerParams, searchQuery, itemType, folderID)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set proper file paths for all file items
		for _, item := range items {
			if file, ok := item.(*domain.File); ok {
				utils.SetDriveFilePath(headerParams, file)
			}
		}

		json.NewEncoder(rw).Encode(map[string][]domain.DriveItem{
			"items": items,
		})
	}
}

// Deep search (files+folders recursively)
func (dh *driveHandler) DeepSearchItems() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		searchQuery := r.URL.Query().Get("q")
		var folderID *primitive.ObjectID
		if folderIDStr := r.URL.Query().Get("folderId"); folderIDStr != "" {
			if id, err := primitive.ObjectIDFromHex(folderIDStr); err == nil {
				folderID = &id
			}
		}
		items, err := dh.DriveUsecase.DeepSearchItems(headerParams, searchQuery, folderID)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set proper file paths for all file items
		for _, item := range items {
			if file, ok := item.(*domain.File); ok {
				utils.SetDriveFilePath(headerParams, file)
			}
		}

		json.NewEncoder(rw).Encode(map[string][]domain.DriveItem{"items": items})
	}
}

// File Management Handlers
func (dh *driveHandler) UploadFile() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_ = common.ExtractHeaderParams(r)

		// This would integrate with your existing file upload logic
		// For now, returning a placeholder response
		http.Error(rw, "File upload integration needed", http.StatusNotImplemented)
	}
}

func (dh *driveHandler) MoveItem() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		vars := mux.Vars(r)

		id, err := primitive.ObjectIDFromHex(vars["id"])
		if err != nil {
			http.Error(rw, "Invalid item ID", http.StatusBadRequest)
			return
		}

		var request struct {
			TargetFolderId string `json:"targetFolderId"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(rw, "Invalid request body", http.StatusBadRequest)
			return
		}

		var targetFolderID *primitive.ObjectID
		if request.TargetFolderId != "" {
			id, err := primitive.ObjectIDFromHex(request.TargetFolderId)
			if err != nil {
				http.Error(rw, "Invalid targetFolderId", http.StatusBadRequest)
				return
			}
			targetFolderID = &id
		}

		userID := getCurrentUserID(r)

		// Try to move as file first
		file, err := dh.DriveUsecase.MoveFile(headerParams, id, targetFolderID, userID)
		if err != nil {
			// If it's not a file, try as folder
			folder, err := dh.DriveUsecase.MoveFolder(headerParams, id, targetFolderID, userID)
			if err != nil {
				http.Error(rw, err.Error(), utils.GetStatusCode(err))
				return
			}
			json.NewEncoder(rw).Encode(map[string]*domain.Folder{
				"folder": folder,
			})
			return
		}

		// Set proper file path for serving
		utils.SetDriveFilePath(headerParams, file)

		json.NewEncoder(rw).Encode(map[string]*domain.File{
			"file": file,
		})
	}
}

// Bulk Operations Handlers
func (dh *driveHandler) BulkMove() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		var request domain.DriveBulkOperation
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(rw, "Invalid request body", http.StatusBadRequest)
			return
		}

		if request.Action != "move" {
			http.Error(rw, "Invalid action", http.StatusBadRequest)
			return
		}

		userID := getCurrentUserID(r)
		response := dh.DriveUsecase.BulkMove(headerParams, request.ItemIds, request.TargetId, userID)

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
	}
}

func (dh *driveHandler) BulkDelete() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		var request domain.DriveBulkOperation
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(rw, "Invalid request body", http.StatusBadRequest)
			return
		}

		if request.Action != "delete" {
			http.Error(rw, "Invalid action", http.StatusBadRequest)
			return
		}

		userID := getCurrentUserID(r)
		response := dh.DriveUsecase.BulkDelete(headerParams, request.ItemIds, userID)

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
	}
}
func (dh *driveHandler) BulkRename() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		var request struct {
			Items []struct {
				Id   string `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(rw, "Invalid request body", http.StatusBadRequest)
			return
		}

		if len(request.Items) == 0 {
			http.Error(rw, "No items provided", http.StatusBadRequest)
			return
		}

		updates := make(map[primitive.ObjectID]string)
		for _, it := range request.Items {
			if it.Id == "" || it.Name == "" {
				http.Error(rw, "Each item must include id and name", http.StatusBadRequest)
				return
			}
			oid, err := primitive.ObjectIDFromHex(it.Id)
			if err != nil {
				http.Error(rw, "Invalid item ID: "+it.Id, http.StatusBadRequest)
				return
			}
			updates[oid] = it.Name
		}

		userID := getCurrentUserID(r)
		response := dh.DriveUsecase.BulkRenameItems(headerParams, updates, userID)

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
	}
}

// File Operations Handlers
func (dh *driveHandler) GetFile() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		vars := mux.Vars(r)

		id, err := primitive.ObjectIDFromHex(vars["id"])
		if err != nil {
			http.Error(rw, "Invalid file ID", http.StatusBadRequest)
			return
		}

		file, err := dh.DriveUsecase.GetFile(headerParams, id)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusNotFound)
			return
		}

		// Set proper file path for serving
		utils.SetDriveFilePath(headerParams, file)

		json.NewEncoder(rw).Encode(map[string]*domain.File{
			"file": file,
		})
	}
}

func (dh *driveHandler) UpdateFile() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		vars := mux.Vars(r)

		id, err := primitive.ObjectIDFromHex(vars["id"])
		if err != nil {
			http.Error(rw, "Invalid file ID", http.StatusBadRequest)
			return
		}

		// Fetch the existing file
		existingFile, err := dh.DriveUsecase.GetFile(headerParams, id)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusNotFound)
			return
		}

		// Parse only the fields to be updated
		var requestBody struct {
			Name     string  `json:"name,omitempty"`
			FolderId *string `json:"folderId,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(rw, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Set changes - preserve all other fields!
		if requestBody.Name != "" {
			existingFile.OriginalName = requestBody.Name
		}
		if requestBody.FolderId != nil && *requestBody.FolderId != "" {
			folderId, err := primitive.ObjectIDFromHex(*requestBody.FolderId)
			if err == nil {
				existingFile.FolderId = &folderId
			}
		}
		existingFile.UpdatedAt = time.Now()
		// (handle UpdatedBy etc. as needed)

		userID := getCurrentUserID(r)
		updatedFile, err := dh.DriveUsecase.UpdateFile(headerParams, id, *existingFile, userID)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set proper file path for serving
		utils.SetDriveFilePath(headerParams, updatedFile)

		json.NewEncoder(rw).Encode(map[string]*domain.File{
			"file": updatedFile,
		})
	}
}

func (dh *driveHandler) DeleteFile() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		vars := mux.Vars(r)

		id, err := primitive.ObjectIDFromHex(vars["id"])
		if err != nil {
			http.Error(rw, "Invalid file ID", http.StatusBadRequest)
			return
		}

		userID := getCurrentUserID(r)
		file, err := dh.DriveUsecase.DeleteFile(headerParams, id, userID)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(rw).Encode(map[string]*domain.File{
			"file": file,
		})
	}
}

// Simple APIs for files and folders with pagination and filtering
func (dh *driveHandler) GetFiles() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		// Parse query parameters
		page := r.URL.Query().Get("page")
		perPage := r.URL.Query().Get("perPage")
		folderId := r.URL.Query().Get("folderId")
		search := r.URL.Query().Get("search")

		// Build filter - only get drive files
		filter := bson.M{
			"trash": bson.M{"$ne": true},
			"type":  "drive", // Only get files uploaded through drive module
		}

		// Add folder filter if provided
		if folderId != "" {
			if id, err := primitive.ObjectIDFromHex(folderId); err == nil {
				filter["folderId"] = id
			}
		} else {
			// If no folderId, get files in root (null or missing folderId)
			filter["$or"] = []bson.M{
				{"folderId": nil},
				{"folderId": bson.M{"$exists": false}},
			}
		}

		// Add search filter if provided
		if search != "" {
			// avoid inserting the same map into itself (which creates a cyclic filter)
			// create a shallow copy of the base filter and combine it with the search clause
			baseFilter := bson.M{}
			for k, v := range filter {
				baseFilter[k] = v
			}
			searchClause := bson.M{
				"$or": []bson.M{
					{"originalName": bson.M{"$regex": search, "$options": "i"}},
					{"mimeType": bson.M{"$regex": search, "$options": "i"}},
				},
			}
			filter = bson.M{
				"$and": []bson.M{
					baseFilter,
					searchClause,
				},
			}
		}

		files, err := dh.DriveUsecase.GetFiles(headerParams, filter, page, perPage, r.URL.Query().Get("sort"))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set proper file paths for serving
		for i := range files.Files {
			utils.SetDriveFilePath(headerParams, &files.Files[i])
		}

		json.NewEncoder(rw).Encode(files)
	}
}

func (dh *driveHandler) GetFolders() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		// Parse query parameters
		page := r.URL.Query().Get("page")
		perPage := r.URL.Query().Get("perPage")
		parentId := r.URL.Query().Get("parentId")
		search := r.URL.Query().Get("search")

		// Build filter
		filter := bson.M{"trash": bson.M{"$ne": true}}

		// Add parent filter if provided
		if parentId != "" {
			if id, err := primitive.ObjectIDFromHex(parentId); err == nil {
				filter["parentId"] = id
			}
		} else {
			// If no parentId, get folders in root (null or missing parentId)
			filter["$or"] = []bson.M{
				{"parentId": nil},
				{"parentId": bson.M{"$exists": false}},
			}
		}

		// Add search filter if provided
		if search != "" {
			// avoid inserting the same map into itself (which creates a cyclic filter)
			baseFilter := bson.M{}
			for k, v := range filter {
				baseFilter[k] = v
			}
			searchClause := bson.M{"name": bson.M{"$regex": search, "$options": "i"}}
			filter = bson.M{"$and": []bson.M{baseFilter, searchClause}}
		}

		folders, err := dh.DriveUsecase.GetFolders(headerParams, filter, page, perPage)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(rw).Encode(folders)
	}
}

// Copy Operations Handlers
func (dh *driveHandler) CopyItem() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)
		vars := mux.Vars(r)

		id, err := primitive.ObjectIDFromHex(vars["id"])
		if err != nil {
			http.Error(rw, "Invalid item ID", http.StatusBadRequest)
			return
		}

		var request struct {
			TargetFolderId string `json:"targetFolderId"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(rw, "Invalid request body", http.StatusBadRequest)
			return
		}

		var targetFolderID *primitive.ObjectID
		if request.TargetFolderId != "" {
			id, err := primitive.ObjectIDFromHex(request.TargetFolderId)
			if err != nil {
				http.Error(rw, "Invalid targetFolderId", http.StatusBadRequest)
				return
			}
			targetFolderID = &id
		}

		userID := getCurrentUserID(r)

		// Try to copy as file first
		file, err := dh.DriveUsecase.CopyFile(headerParams, id, targetFolderID, userID)
		if err != nil {
			// If it's not a file, try as folder
			folder, err := dh.DriveUsecase.CopyFolder(headerParams, id, targetFolderID, userID)
			if err != nil {
				http.Error(rw, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(rw).Encode(map[string]*domain.Folder{
				"folder": folder,
			})
			return
		}

		// Set proper file path for serving
		utils.SetDriveFilePath(headerParams, file)

		json.NewEncoder(rw).Encode(map[string]*domain.File{
			"file": file,
		})
	}
}

func (dh *driveHandler) BulkCopy() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		var request domain.DriveBulkOperation
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(rw, "Invalid request body", http.StatusBadRequest)
			return
		}

		if request.Action != "copy" {
			http.Error(rw, "Invalid action", http.StatusBadRequest)
			return
		}

		userID := getCurrentUserID(r)
		response := dh.DriveUsecase.BulkCopy(headerParams, request.ItemIds, request.TargetId, userID)

		rw.WriteHeader(http.StatusOK)
		json.NewEncoder(rw).Encode(response)
	}
}

// GetFolderTree returns the complete folder hierarchy with files and subfolders
func (dh *driveHandler) GetFolderTree() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		// Get root folder ID from query parameter (optional)
		var rootFolderID *primitive.ObjectID
		if rootIDStr := r.URL.Query().Get("rootId"); rootIDStr != "" {
			if id, err := primitive.ObjectIDFromHex(rootIDStr); err == nil {
				rootFolderID = &id
			}
		}

		// Get max depth from query parameter (optional, default: 10)
		maxDepth := 10
		if depthStr := r.URL.Query().Get("maxDepth"); depthStr != "" {
			if depth, err := strconv.Atoi(depthStr); err == nil && depth > 0 {
				maxDepth = depth
			}
		}

		tree, err := dh.DriveUsecase.GetFolderTree(headerParams, rootFolderID, maxDepth)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set proper file paths for all files in the tree
		dh.setTreeFilePaths(headerParams, &tree.Root)

		json.NewEncoder(rw).Encode(tree)
	}
}

// setTreeFilePaths recursively sets proper file paths for all files in the tree
func (dh *driveHandler) setTreeFilePaths(headerParams *common.HeaderParams, node *domain.FolderTreeNode) {
	// Set file paths for files in current node
	if len(node.Files) > 0 {
		// Convert to pointers for SetDriveFilePath
		filePointers := make([]*domain.File, len(node.Files))
		for i := range node.Files {
			filePointers[i] = &node.Files[i]
		}
		utils.SetDriveFilePath(headerParams, filePointers...)
	}

	// Recursively process children
	for i := range node.Children {
		dh.setTreeFilePaths(headerParams, &node.Children[i])
	}
}

// FilterFiles allows advanced filtering of files with a POST request
func (dh *driveHandler) FilterFiles() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		headerParams := common.ExtractHeaderParams(r)

		// Define a struct to parse the request body
		var filterRequest struct {
			Page      string   `json:"page"`
			PerPage   string   `json:"perPage"`
			Search    string   `json:"search"`
			FolderId  string   `json:"folderId"`
			MimeType  []string `json:"mimeType"`
			IsProduct *bool    `json:"isProduct"`
			Type      string   `json:"type"`
			Recursive bool     `json:"recursive"`
		}

		// Decode the request body
		if err := json.NewDecoder(r.Body).Decode(&filterRequest); err != nil {
			http.Error(rw, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Build filter - only get drive files
		filter := bson.M{
			"trash": bson.M{"$ne": true},
			"type":  "drive", // Only get files uploaded through drive module
		}

		// Add folder filter if provided
		if filterRequest.FolderId != "" {
			if id, err := primitive.ObjectIDFromHex(filterRequest.FolderId); err == nil {
				filter["folderId"] = id
			}
		} else {
			// If no folderId, get files in root (null or missing folderId)
			filter["$or"] = []bson.M{
				{"folderId": nil},
				{"folderId": bson.M{"$exists": false}},
			}
		}

		// Add search filter if provided
		if filterRequest.Search != "" {
			// avoid inserting the same map into itself (which creates a cyclic filter)
			baseFilter := bson.M{}
			for k, v := range filter {
				baseFilter[k] = v
			}
			searchClause := bson.M{
				"$or": []bson.M{
					{"originalName": bson.M{"$regex": filterRequest.Search, "$options": "i"}},
					{"mimeType": bson.M{"$regex": filterRequest.Search, "$options": "i"}},
				},
			}
			filter = bson.M{
				"$and": []bson.M{
					baseFilter,
					searchClause,
				},
			}
		}

		// Add mime type filter if provided
		if len(filterRequest.MimeType) > 0 {
			filter["mimeType"] = bson.M{"$in": filterRequest.MimeType}
		}

		// Add isProduct filter if provided
		if filterRequest.IsProduct != nil {
			filter["isProduct"] = *filterRequest.IsProduct
		}

		// Add type filter if provided
		if filterRequest.Type != "" {
			filter["type"] = filterRequest.Type
		}

		// Get files with filter
		var files *domain.FilePagination
		var err error
		sort := r.URL.Query().Get("sort")
		if filterRequest.Recursive && filterRequest.FolderId != "" {
			if id, errHex := primitive.ObjectIDFromHex(filterRequest.FolderId); errHex == nil {
				files, err = dh.DriveUsecase.GetFilesRecursive(headerParams, filter, &id, filterRequest.Page, filterRequest.PerPage, sort)
			} else {
				// Fallback if ID invalid
				files, err = dh.DriveUsecase.GetFiles(headerParams, filter, filterRequest.Page, filterRequest.PerPage, sort)
			}
		} else {
			files, err = dh.DriveUsecase.GetFiles(headerParams, filter, filterRequest.Page, filterRequest.PerPage, sort)
		}

		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set proper file paths for serving
		for i := range files.Files {
			utils.SetDriveFilePath(headerParams, &files.Files[i])
		}

		json.NewEncoder(rw).Encode(files)
	}
}
