package file

import (
	"errors"
	"fmt"
	"larsa-files/domain"
	"larsa-files/file/repository/mongo"
	"larsa-files/pkg/uploadS3"
	"larsa-files/utils"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DriveUsecase struct {
	FileRepo   domain.FileRepositoty
	FolderRepo *mongo.FolderMongoRepository
}

func NewDriveUsecase(fileRepo domain.FileRepositoty, folderRepo *mongo.FolderMongoRepository) *DriveUsecase {
	return &DriveUsecase{
		FileRepo:   fileRepo,
		FolderRepo: folderRepo,
	}
}

// Folder Management
func (du *DriveUsecase) EmptyTrash(headerParams *common.HeaderParams) (int64, error) {
	return du.FolderRepo.HardDeleteTrashedFolders(headerParams.Client)
}

func (du *DriveUsecase) CreateFolder(headerParams *common.HeaderParams, name string, parentID *primitive.ObjectID, userID string) (*domain.Folder, error) {
	if parentID != nil {
		depth, err := du.getFolderDepth(headerParams, *parentID)
		if err != nil {
			return nil, err
		}
		if depth >= du.getFolderDepthLimit(headerParams) {
			return nil, errors.New("maximum folder depth reached")
		}
	}

	folder := domain.Folder{
		Name:      name,
		ParentId:  parentID,
		Service:   headerParams.Service,
		CreatedBy: userID,
		UpdatedBy: userID,
	}

	return du.FolderRepo.CreateFolder(headerParams.Client, folder)
}

func (du *DriveUsecase) GetFolder(headerParams *common.HeaderParams, id primitive.ObjectID) (*domain.Folder, error) {
	return du.FolderRepo.GetFolder(headerParams.Client, id)
}

func (du *DriveUsecase) UpdateFolder(headerParams *common.HeaderParams, id primitive.ObjectID, name string, userID string) (*domain.Folder, error) {
	folder, err := du.FolderRepo.GetFolder(headerParams.Client, id)
	if err != nil {
		return nil, err
	}

	if !folder.Deletable && folder.Name != name {
		return nil, utils.NewApiError(http.StatusForbidden, "folder name is protected and cannot be changed")
	}

	folder.Name = name
	folder.UpdatedBy = userID
	folder.UpdatedAt = time.Now()

	return du.FolderRepo.UpdateFolder(headerParams.Client, id, *folder)
}

func (du *DriveUsecase) UpdateFolderProtection(headerParams *common.HeaderParams, id primitive.ObjectID, deletable, movable, copyable *bool, userID string) (*domain.Folder, error) {
	folder, err := du.FolderRepo.GetFolder(headerParams.Client, id)
	if err != nil {
		return nil, err
	}

	if deletable != nil {
		folder.Deletable = *deletable
	}
	if movable != nil {
		folder.Movable = *movable
	}
	if copyable != nil {
		folder.Copyable = *copyable
	}

	folder.UpdatedBy = userID
	folder.UpdatedAt = time.Now()

	return du.FolderRepo.UpdateFolder(headerParams.Client, id, *folder)
}

func (du *DriveUsecase) DeleteFolder(headerParams *common.HeaderParams, id primitive.ObjectID, userID string) (*domain.Folder, error) {
	// 0. Check if folder is deletable
	folder, err := du.FolderRepo.GetFolder(headerParams.Client, id)
	if err != nil {
		return nil, err
	}
	if !folder.Deletable {
		return nil, utils.NewApiError(http.StatusForbidden, "folder is not deletable")
	}

	// 1. Find all subfolder IDs recursively using BFS
	allFolderIDs := []primitive.ObjectID{id}
	queue := []primitive.ObjectID{id}

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		subFolderFilter := bson.M{
			"trash":    bson.M{"$ne": true},
			"parentId": currentID,
		}

		// Use a large perPage to get subfolders at this level
		folders, err := du.FolderRepo.GetFolders(headerParams.Client, subFolderFilter, "1", "1000")
		if err == nil && folders != nil {
			for _, f := range folders.Folders {
				allFolderIDs = append(allFolderIDs, f.Id)
				queue = append(queue, f.Id)
			}
		}
	}

	// 2. Mark all files in all these folders as trashed
	filesFilter := bson.M{
		"trash":    bson.M{"$ne": true},
		"folderId": bson.M{"$in": allFolderIDs},
	}
	for {
		// Get files in batches and mark them as trash
		files, err := du.FileRepo.GetFiles(headerParams.Client, filesFilter, "1", "100", "")
		if err != nil || files == nil || len(files.Files) == 0 {
			break
		}
		for _, file := range files.Files {
			_, _ = du.FileRepo.Delete(headerParams.Client, file.Id, userID)
		}
	}

	// 3. Mark all subfolders as trashed (except the root folder)
	for i := len(allFolderIDs) - 1; i > 0; i-- {
		_, _ = du.FolderRepo.DeleteFolder(headerParams.Client, allFolderIDs[i], userID)
	}

	// 4. Finally, mark the root folder itself as trashed and return it
	return du.FolderRepo.DeleteFolder(headerParams.Client, id, userID)
}

// File Management with Folder Support
func (du *DriveUsecase) UploadFile(headerParams *common.HeaderParams, file domain.File, folderID *primitive.ObjectID, userID string) (*domain.File, error) {
	file.FolderId = folderID
	file.CreatedBy = userID
	file.UpdatedBy = userID
	file.CreatedAt = time.Now()
	file.UpdatedAt = time.Now()

	res, err := du.FileRepo.Store(headerParams.Client, file)
	if err != nil {
		return nil, err
	}
	// When we get the file back, it has the ID but we need to ensure the ID is set on the return value
	// if the repo didn't update the passed struct in place (although Store usually returns a new pointer)
	// Just return what the repo gave us.
	return res, nil
}

func (du *DriveUsecase) MoveFile(headerParams *common.HeaderParams, fileID primitive.ObjectID, newFolderID *primitive.ObjectID, userID string) (*domain.File, error) {
	file := domain.File{
		FolderId:  newFolderID,
		UpdatedBy: userID,
		UpdatedAt: time.Now(),
	}

	return du.FileRepo.Update(headerParams.Client, fileID, file, nil)
}

func (du *DriveUsecase) MoveFolder(headerParams *common.HeaderParams, folderID primitive.ObjectID, newParentID *primitive.ObjectID, userID string) (*domain.Folder, error) {
	// Check if folder is movable
	folder, err := du.FolderRepo.GetFolder(headerParams.Client, folderID)
	if err != nil {
		return nil, err
	}
	if !folder.Movable {
		return nil, utils.NewApiError(http.StatusForbidden, "folder is not movable")
	}

	if newParentID != nil {
		// Check for circular reference
		isSub, err := du.isSubfolderOf(headerParams.Client, *newParentID, folderID)
		if err != nil {
			return nil, err
		}
		if isSub || folderID == *newParentID {
			return nil, utils.NewApiError(http.StatusForbidden, "cannot move a folder into itself or its subfolders")
		}

		// Check depth
		parentDepth, err := du.getFolderDepth(headerParams, *newParentID)
		if err != nil {
			return nil, err
		}

		subfolderMaxDepth, err := du.getMaxSubfolderDepth(headerParams, folderID)
		if err != nil {
			return nil, err
		}

		if parentDepth+1+subfolderMaxDepth > du.getFolderDepthLimit(headerParams) {
			return nil, utils.NewApiError(http.StatusForbidden, "maximum folder depth reached")
		}
	}

	return du.FolderRepo.MoveFolder(headerParams.Client, folderID, newParentID, userID)
}

// Navigation and Browsing
func (du *DriveUsecase) GetFolderContents(headerParams *common.HeaderParams, folderID *primitive.ObjectID, query bson.M, page, perPage string) (*domain.DrivePagination, error) {
	return du.FolderRepo.GetFolderContents(headerParams.Client, folderID, query, page, perPage)
}

func (du *DriveUsecase) GetRecentItems(headerParams *common.HeaderParams, limit int, userID string) ([]domain.DriveItem, error) {
	return du.FolderRepo.GetRecentItems(headerParams.Client, userID, limit)
}

func (du *DriveUsecase) SearchItems(headerParams *common.HeaderParams, searchQuery string, itemType string, folderID *primitive.ObjectID) ([]domain.DriveItem, error) {
	return du.FolderRepo.SearchItems(headerParams.Client, searchQuery, itemType, folderID)
}

// Unified deep search API: search term in files and folders under all subfolders
func (du *DriveUsecase) DeepSearchItems(headerParams *common.HeaderParams, query string, folderID *primitive.ObjectID) ([]domain.DriveItem, error) {
	return du.FolderRepo.DeepSearchItems(headerParams.Client, query, folderID)
}

// Bulk Operations
func (du *DriveUsecase) BulkMove(headerParams *common.HeaderParams, itemIDs []primitive.ObjectID, newFolderID *primitive.ObjectID, userID string) domain.BulkOperationResponse {
	response := domain.BulkOperationResponse{
		Errors: make(map[string]string),
	}

	// Validation: Check if target folder exists if moving to a specific folder
	if newFolderID != nil {
		_, err := du.FolderRepo.GetFolder(headerParams.Client, *newFolderID)
		if err != nil {
			response.FailureCount = len(itemIDs)
			for _, id := range itemIDs {
				response.Errors[id.Hex()] = "target folder not found or inaccessible"
			}
			return response
		}
	}

	for _, itemID := range itemIDs {
		// Try to move as file first
		file := domain.File{
			FolderId:  newFolderID,
			UpdatedBy: userID,
			UpdatedAt: time.Now(),
		}

		_, err := du.FileRepo.Update(headerParams.Client, itemID, file, nil)
		if err == nil {
			response.SuccessCount++
			continue
		}

		// If it's not a file (or file update failed for other reasons), try as folder
		// Check for circular reference if moving a folder
		if newFolderID != nil {
			_, err = du.MoveFolder(headerParams, itemID, newFolderID, userID)
		} else {
			_, err = du.FolderRepo.MoveFolder(headerParams.Client, itemID, newFolderID, userID)
		}
		if err != nil {
			response.FailureCount++
			response.Errors[itemID.Hex()] = err.Error()
		} else {
			response.SuccessCount++
		}
	}

	return response
}

// isSubfolderOf checks if potentialChildID is a subfolder (at any depth) of potentialParentID
func (du *DriveUsecase) isSubfolderOf(clientID string, potentialChildID, potentialParentID primitive.ObjectID) (bool, error) {
	currentID := potentialChildID
	visited := make(map[primitive.ObjectID]bool)

	for currentID != primitive.NilObjectID {
		if currentID == potentialParentID {
			return true, nil
		}
		if visited[currentID] {
			return false, errors.New("circular reference detected in folder hierarchy")
		}
		visited[currentID] = true

		folder, err := du.FolderRepo.GetFolder(clientID, currentID)
		if err != nil {
			return false, nil // Assume no more parents or inaccessible
		}

		if folder.ParentId != nil {
			currentID = *folder.ParentId
		} else {
			break
		}
	}
	return false, nil
}

func (du *DriveUsecase) getFolderDepth(headerParams *common.HeaderParams, folderID primitive.ObjectID) (int, error) {
	depth := 0
	currentID := folderID
	visited := make(map[primitive.ObjectID]bool)

	for currentID != primitive.NilObjectID {
		depth++
		if visited[currentID] {
			return depth, errors.New("circular reference detected in folder hierarchy")
		}
		visited[currentID] = true

		folder, err := du.FolderRepo.GetFolder(headerParams.Client, currentID)
		if err != nil {
			return depth, nil // Stop if we can't find more parents
		}

		if folder.ParentId != nil {
			currentID = *folder.ParentId
		} else {
			break
		}
	}
	return depth, nil
}

func (du *DriveUsecase) getMaxSubfolderDepth(headerParams *common.HeaderParams, folderID primitive.ObjectID) (int, error) {
	maxDepth := 0

	subFolderFilter := bson.M{
		"trash":    bson.M{"$ne": true},
		"parentId": folderID,
	}

	folders, err := du.FolderRepo.GetFolders(headerParams.Client, subFolderFilter, "1", "1000")
	if err != nil || folders == nil {
		return 0, nil
	}

	for _, f := range folders.Folders {
		subDepth, err := du.getMaxSubfolderDepth(headerParams, f.Id)
		if err != nil {
			return maxDepth, err
		}
		if subDepth+1 > maxDepth {
			maxDepth = subDepth + 1
		}
	}

	return maxDepth, nil
}

func (du *DriveUsecase) getFolderDepthLimit(headerParams *common.HeaderParams) int {
	limitVal, err := common.GetOptionValue("FOLDER_DEPTH_LIMIT", headerParams)
	if err != nil || limitVal == nil {
		return 15 // Default limit
	}

	var limitStr string
	switch v := limitVal.(type) {
	case string:
		limitStr = v
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 15
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return 15
	}
	return limit
}

func (du *DriveUsecase) BulkDelete(headerParams *common.HeaderParams, itemIDs []primitive.ObjectID, userID string) domain.BulkOperationResponse {
	response := domain.BulkOperationResponse{
		Errors: make(map[string]string),
	}

	for _, itemID := range itemIDs {
		// Try to delete as file first
		_, err := du.FileRepo.Delete(headerParams.Client, itemID, userID)
		if err == nil {
			response.SuccessCount++
			continue
		}

		// If it's not a file, try as folder
		folder, err := du.FolderRepo.GetFolder(headerParams.Client, itemID)
		if err == nil {
			if !folder.Deletable {
				response.FailureCount++
				response.Errors[itemID.Hex()] = "folder is not deletable"
				continue
			}
		}

		_, err = du.FolderRepo.DeleteFolder(headerParams.Client, itemID, userID)
		if err != nil {
			response.FailureCount++
			response.Errors[itemID.Hex()] = err.Error()
		} else {
			response.SuccessCount++
		}
	}

	return response
}

// Sharing and Permissions (placeholder - would need additional implementation)
func (du *DriveUsecase) ShareItem(headerParams *common.HeaderParams, itemID primitive.ObjectID, permissions []string, users []string) error {
	// TODO: Implement sharing functionality
	return nil
}

func (du *DriveUsecase) GetSharedItems(headerParams *common.HeaderParams) ([]domain.DriveItem, error) {
	// TODO: Implement shared items functionality
	return []domain.DriveItem{}, nil
}

// Enhanced file operations with folder support
func (du *DriveUsecase) GetFile(headerParams *common.HeaderParams, id primitive.ObjectID) (*domain.File, error) {
	file, err := du.FileRepo.GetByID(headerParams.Client, id)
	if err != nil {
		return nil, err
	}

	// Ensure we only expose drive files through the drive API
	if file.Type != "drive" {
		return nil, errors.New("file not found")
	}

	return file, nil
}

func (du *DriveUsecase) UpdateFile(headerParams *common.HeaderParams, id primitive.ObjectID, file domain.File, userID string) (*domain.File, error) {
	// Set updater metadata
	file.UpdatedBy = userID
	file.UpdatedAt = time.Now()

	// If client supplied a new OriginalName (name change), perform an explicit update
	// because the repository's default Update (when updateQuery == nil) only updates folderId and timestamps.
	if file.OriginalName != "" {
		setFields := bson.M{
			"originalName": file.OriginalName,
			"updatedBy":    file.UpdatedBy,
			"updatedAt":    file.UpdatedAt,
		}
		// include folderId if provided alongside name change
		if file.FolderId != nil {
			setFields["folderId"] = file.FolderId
		}
		updateQuery := bson.M{"$set": setFields}
		return du.FileRepo.Update(headerParams.Client, id, domain.File{}, updateQuery)
	}

	// Otherwise use the repository's default behavior (updates folderId and timestamps)
	return du.FileRepo.Update(headerParams.Client, id, file, nil)
}

func (du *DriveUsecase) DeleteFile(headerParams *common.HeaderParams, id primitive.ObjectID, userID string) (*domain.File, error) {
	return du.FileRepo.Delete(headerParams.Client, id, userID)
}

// BulkRenameItems updates names for multiple files or folders; returns updated counts and per-item errors
func (du *DriveUsecase) BulkRenameItems(headerParams *common.HeaderParams, updates map[primitive.ObjectID]string, userID string) domain.BulkOperationResponse {
	response := domain.BulkOperationResponse{
		Errors: make(map[string]string),
	}

	for id, newName := range updates {
		// Try as file first
		update := bson.M{
			"$set": bson.M{
				"originalName": newName,
				"updatedBy":    userID,
				"updatedAt":    time.Now(),
			},
		}

		_, err := du.FileRepo.Update(headerParams.Client, id, domain.File{}, update)
		if err == nil {
			response.SuccessCount++
			continue
		}

		// Try as folder
		folder, err := du.FolderRepo.GetFolder(headerParams.Client, id)
		if err == nil {
			if !folder.Deletable && folder.Name != newName {
				err = utils.NewApiError(http.StatusForbidden, "folder name is protected and cannot be changed")
			} else {
				folder.Name = newName
				folder.UpdatedBy = userID
				_, err = du.FolderRepo.UpdateFolder(headerParams.Client, id, *folder)
			}
		}

		if err != nil {
			response.FailureCount++
			response.Errors[id.Hex()] = err.Error()
		} else {
			response.SuccessCount++
		}
	}

	return response
}

// Simple APIs for files and folders with pagination
func (du *DriveUsecase) GetFiles(headerParams *common.HeaderParams, filter bson.M, page, perPage string, sort string) (*domain.FilePagination, error) {
	return du.FileRepo.GetFiles(headerParams.Client, filter, page, perPage, sort)
}

// GetFilesRecursive retrieves files from a folder and all its subfolders
func (du *DriveUsecase) GetFilesRecursive(headerParams *common.HeaderParams, filter bson.M, rootFolderID *primitive.ObjectID, page, perPage string, sort string) (*domain.FilePagination, error) {
	// 1. Find all subfolder IDs recursively
	allFolderIDs := []primitive.ObjectID{*rootFolderID}

	// Helper queue for BFS
	queue := []primitive.ObjectID{*rootFolderID}

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		// Find subfolders of current folder
		subFolderFilter := bson.M{
			"trash":    bson.M{"$ne": true},
			"parentId": currentID,
		}

		// We only need IDs, but repo returns full objects.
		// For efficiency in a real production app, we might want a specific repo method
		// that only returns IDs, but reusing existing method is safer for now.
		folders, err := du.FolderRepo.GetFolders(headerParams.Client, subFolderFilter, "1", "10000")
		if err == nil && folders != nil {
			for _, f := range folders.Folders {
				allFolderIDs = append(allFolderIDs, f.Id)
				queue = append(queue, f.Id)
			}
		}
	}

	// 2. Modify filter to include all folder IDs
	// Remove single folderId constraint if it exists
	delete(filter, "folderId")

	// Add $in clause for all folder IDs
	filter["folderId"] = bson.M{"$in": allFolderIDs}

	// 3. Get files using modified filter
	return du.FileRepo.GetFiles(headerParams.Client, filter, page, perPage, sort)
}

func (du *DriveUsecase) GetFolders(headerParams *common.HeaderParams, filter bson.M, page, perPage string) (*domain.FolderPagination, error) {
	return du.FolderRepo.GetFolders(headerParams.Client, filter, page, perPage)
}

// Copy Operations
func (du *DriveUsecase) CopyFile(headerParams *common.HeaderParams, fileID primitive.ObjectID, targetFolderID *primitive.ObjectID, userID string) (*domain.File, error) {
	// Get the original file
	originalFile, err := du.FileRepo.GetByID(headerParams.Client, fileID)
	if err != nil {
		return nil, err
	}

	// Download the original file from S3
	fileData, err := uploadS3.GetFileFromS3(originalFile.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to download file from S3: %w", err)
	}

	// Create a new path for the copied file
	Client := headerParams.Client
	t := time.Now()
	year := t.Year()
	month := int(t.Month())
	dir := filepath.Join(Client, strconv.Itoa(year), strconv.Itoa(month))

	ext := filepath.Ext(originalFile.Path)
	newFileName := fmt.Sprintf("%s%s", uuid.NewString(), ext)
	newFilePath := filepath.Join(dir, newFileName)

	// Upload the file to S3 with the new path
	err = uploadS3.StoreFileS3(fileData, newFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file to S3: %w", err)
	}

	// Create a copy with new ID, new path, and updated metadata
	copiedFile := *originalFile
	copiedFile.Id = primitive.NewObjectID()
	copiedFile.Path = newFilePath // Use the new S3 path
	copiedFile.FolderId = targetFolderID
	copiedFile.CreatedBy = userID
	copiedFile.UpdatedBy = userID
	copiedFile.CreatedAt = time.Now()
	copiedFile.UpdatedAt = time.Now()
	copiedFile.Type = "drive" // Ensure it's marked as drive file

	// Store the copied file
	return du.FileRepo.Store(headerParams.Client, copiedFile)
}

func (du *DriveUsecase) CopyFolder(headerParams *common.HeaderParams, folderID primitive.ObjectID, targetParentID *primitive.ObjectID, userID string) (*domain.Folder, error) {
	// Get the original folder
	originalFolder, err := du.FolderRepo.GetFolder(headerParams.Client, folderID)
	if err != nil {
		return nil, err
	}

	// Check if folder is copyable
	if !originalFolder.Copyable {
		return nil, errors.New("folder is not copyable")
	}

	if targetParentID != nil {
		// Check depth
		parentDepth, err := du.getFolderDepth(headerParams, *targetParentID)
		if err != nil {
			return nil, err
		}

		subfolderMaxDepth, err := du.getMaxSubfolderDepth(headerParams, folderID)
		if err != nil {
			return nil, err
		}

		if parentDepth+1+subfolderMaxDepth > du.getFolderDepthLimit(headerParams) {
			return nil, errors.New("maximum folder depth reached")
		}
	}

	// Create a copy with new ID and updated metadata
	copiedFolder := *originalFolder
	copiedFolder.Id = primitive.NewObjectID()
	copiedFolder.ParentId = targetParentID // Only change the parent reference
	copiedFolder.CreatedBy = userID
	copiedFolder.UpdatedBy = userID
	copiedFolder.CreatedAt = time.Now()
	copiedFolder.UpdatedAt = time.Now()
	// Path will be generated automatically by CreateFolder method

	// Store the copied folder
	newFolder, err := du.FolderRepo.CreateFolder(headerParams.Client, copiedFolder)
	if err != nil {
		return nil, err
	}

	// Copy all files in this folder
	filesFilter := bson.M{
		"trash":    bson.M{"$ne": true},
		"type":     "drive",
		"folderId": folderID,
	}

	files, err := du.FileRepo.GetFiles(headerParams.Client, filesFilter, "1", "1000", "") // Get up to 1000 files
	if err == nil && files != nil {
		for _, file := range files.Files {
			// Copy each file to the new folder
			_, copyErr := du.CopyFile(headerParams, file.Id, &newFolder.Id, userID)
			if copyErr != nil {
				// Log error but continue
				fmt.Printf("Error copying file %s: %v\n", file.Id.Hex(), copyErr)
			}
		}
	}

	// Copy all subfolders recursively
	foldersFilter := bson.M{
		"trash":    bson.M{"$ne": true},
		"parentId": folderID,
	}

	folders, err := du.FolderRepo.GetFolders(headerParams.Client, foldersFilter, "1", "1000") // Get up to 1000 folders
	if err == nil && folders != nil {
		for _, subFolder := range folders.Folders {
			// Recursively copy each subfolder
			_, copyErr := du.CopyFolder(headerParams, subFolder.Id, &newFolder.Id, userID)
			if copyErr != nil {
				// Log error but continue
				fmt.Printf("Error copying folder %s: %v\n", subFolder.Id.Hex(), copyErr)
			}
		}
	}

	return newFolder, nil
}

func (du *DriveUsecase) BulkCopy(headerParams *common.HeaderParams, itemIDs []primitive.ObjectID, targetFolderID *primitive.ObjectID, userID string) domain.BulkOperationResponse {
	response := domain.BulkOperationResponse{
		Errors: make(map[string]string),
	}

	// Validation: Check if target folder exists if copying to a specific folder
	if targetFolderID != nil {
		_, err := du.FolderRepo.GetFolder(headerParams.Client, *targetFolderID)
		if err != nil {
			response.FailureCount = len(itemIDs)
			for _, id := range itemIDs {
				response.Errors[id.Hex()] = "target folder not found or inaccessible"
			}
			return response
		}
	}

	for _, itemID := range itemIDs {
		// Try to copy as file first
		_, err := du.CopyFile(headerParams, itemID, targetFolderID, userID)
		if err == nil {
			response.SuccessCount++
			continue
		}

		if targetFolderID != nil {
			_, err = du.CopyFolder(headerParams, itemID, targetFolderID, userID)
		} else {
			// Copying to root
			_, err = du.CopyFolder(headerParams, itemID, nil, userID)
		}
		if err != nil {
			response.FailureCount++
			response.Errors[itemID.Hex()] = err.Error()
		} else {
			response.SuccessCount++
		}
	}
	return response
}

// GetFolderTree returns the complete folder hierarchy
func (du *DriveUsecase) GetFolderTree(headerParams *common.HeaderParams, rootFolderID *primitive.ObjectID, maxDepth int) (*domain.FolderTree, error) {
	// Build the tree starting from the root
	rootNode, err := du.buildFolderTreeNode(headerParams, rootFolderID, 0, maxDepth)
	if err != nil {
		return nil, err
	}

	return &domain.FolderTree{
		Root: rootNode,
	}, nil
}

// buildFolderTreeNode recursively builds a folder tree node
func (du *DriveUsecase) buildFolderTreeNode(headerParams *common.HeaderParams, folderID *primitive.ObjectID, currentDepth, maxDepth int) (domain.FolderTreeNode, error) {
	node := domain.FolderTreeNode{}

	// If we have a specific folder, get its details
	if folderID != nil {
		folder, err := du.FolderRepo.GetFolder(headerParams.Client, *folderID)
		if err != nil {
			return node, err
		}
		node.Folder = folder
	}

	// Get files in this folder
	filesFilter := bson.M{
		"trash": bson.M{"$ne": true},
		"type":  "drive",
	}
	if folderID != nil {
		filesFilter["folderId"] = *folderID
	} else {
		filesFilter["$or"] = []bson.M{
			{"folderId": bson.M{"$exists": false}},
			{"folderId": nil},
		}
	}

	files, err := du.FileRepo.GetFiles(headerParams.Client, filesFilter, "1", "1000", "") // Get up to 1000 files
	if err == nil {
		node.Files = files.Files
	}

	// Get subfolders if we haven't reached max depth
	if currentDepth < maxDepth {
		foldersFilter := bson.M{
			"trash": bson.M{"$ne": true},
		}
		if folderID != nil {
			foldersFilter["parentId"] = *folderID
		} else {
			foldersFilter["$or"] = []bson.M{
				{"parentId": bson.M{"$exists": false}},
				{"parentId": nil},
			}
		}

		folders, err := du.FolderRepo.GetFolders(headerParams.Client, foldersFilter, "1", "1000") // Get up to 1000 folders
		if err == nil {
			// Recursively build children
			for _, folder := range folders.Folders {
				childNode, err := du.buildFolderTreeNode(headerParams, &folder.Id, currentDepth+1, maxDepth)
				if err == nil {
					node.Children = append(node.Children, childNode)
				}
			}
		}
	}

	return node, nil
}
