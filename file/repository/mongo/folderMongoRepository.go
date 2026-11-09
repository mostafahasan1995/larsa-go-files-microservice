package mongo

import (
	"context"
	"encoding/json"
	"fmt"
	"larsa-files/domain"
	"math"
	"path/filepath"
	"strconv"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/cache"
	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type FolderMongoRepository struct {
	DB    *mongo.Client
	Cache cache.Cache
}

func NewMongoFolderRepository(db *mongo.Client, cache cache.Cache) *FolderMongoRepository {
	return &FolderMongoRepository{
		DB:    db,
		Cache: cache,
	}
}

// enrichFoldersWithStats populates fileCount and size for a list of folders by aggregating actual files
func (fr *FolderMongoRepository) enrichFoldersWithStats(ctx context.Context, clientID string, folders []domain.Folder) error {
	if len(folders) == 0 {
		return nil
	}

	folderIDs := make([]primitive.ObjectID, len(folders))
	folderMap := make(map[primitive.ObjectID]*domain.Folder)

	// Reset counts to 0 and build map
	for i := range folders {
		folderIDs[i] = folders[i].Id
		folders[i].FileCount = 0
		folders[i].Size = 0
		folderMap[folders[i].Id] = &folders[i]
	}

	filesCollection := fr.DB.Database(clientID).Collection("files")

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"folderId": bson.M{"$in": folderIDs},
			"trash":    bson.M{"$ne": true},
			"type":     "drive",
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":       "$folderId",
			"fileCount": bson.M{"$sum": 1},
			"size":      bson.M{"$sum": "$size"},
		}}},
	}

	cursor, err := filesCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var stats []struct {
		FolderID  primitive.ObjectID `bson:"_id"`
		FileCount int                `bson:"fileCount"`
		Size      int64              `bson:"size"`
	}

	if err := cursor.All(ctx, &stats); err != nil {
		return err
	}

	// Apply aggregated stats
	for _, stat := range stats {
		if folder, ok := folderMap[stat.FolderID]; ok {
			folder.FileCount = stat.FileCount
			folder.Size = stat.Size
		}
	}

	// Aggregate folder counts (subfolders)
	foldersCollection := fr.DB.Database(clientID).Collection("folders")
	folderPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"parentId": bson.M{"$in": folderIDs},
			"trash":    bson.M{"$ne": true},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$parentId",
			"count": bson.M{"$sum": 1},
		}}},
	}

	folderCursor, err := foldersCollection.Aggregate(ctx, folderPipeline)
	if err == nil {
		defer folderCursor.Close(ctx)
		var folderStats []struct {
			ParentID primitive.ObjectID `bson:"_id"`
			Count    int                `bson:"count"`
		}
		if err := folderCursor.All(ctx, &folderStats); err == nil {
			for _, stat := range folderStats {
				if folder, ok := folderMap[stat.ParentID]; ok {
					folder.FileCount += stat.Count
				}
			}
		}
	}

	return nil
}

// CreateFolder creates a new folder
func (fr *FolderMongoRepository) CreateFolder(clientID string, folder domain.Folder) (*domain.Folder, error) {
	collection := fr.DB.Database(clientID).Collection("folders")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	folder.CreatedAt = time.Now()
	folder.UpdatedAt = time.Now()
	folder.Trash = false
	folder.Deletable = true
	folder.Movable = true
	folder.Copyable = true

	// Generate folder path
	folder.Path = fr.generateFolderPath(clientID, folder.ParentId, folder.Name)

	res, err := collection.InsertOne(ctx, &folder)
	if err != nil {
		return nil, err
	}

	folder.Id = res.InsertedID.(primitive.ObjectID)

	// Invalidate cache
	fr.invalidateFolderCache(clientID)

	// Enrich with real-time stats
	foldersToEnrich := []domain.Folder{folder}
	if err := fr.enrichFoldersWithStats(ctx, clientID, foldersToEnrich); err != nil {
		fmt.Printf("Error enriching folder with stats in CreateFolder: %v\n", err)
	} else {
		folder = foldersToEnrich[0]
	}

	return &folder, nil
}

// GetFolder retrieves a folder by ID
func (fr *FolderMongoRepository) GetFolder(clientID string, id primitive.ObjectID) (*domain.Folder, error) {
	collection := fr.DB.Database(clientID).Collection("folders")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cacheKey := fmt.Sprintf("folder:g:c:%s:%s", clientID, id.Hex())
	cacheRes, cacheErr := fr.Cache.GetByKey(cacheKey)

	var folder domain.Folder
	if cacheErr != nil {
		err := collection.FindOne(ctx, bson.M{"_id": id, "trash": bson.M{"$ne": true}}).Decode(&folder)
		if err != nil {
			return nil, err
		}
		fr.Cache.CacheByKey(cacheKey, folder, 0)
	} else {
		err := json.Unmarshal([]byte(cacheRes), &folder)
		if err != nil {
			return nil, err
		}
	}

	// Enrich with real-time stats
	// We pass a slice containing the single folder
	foldersToEnrich := []domain.Folder{folder}
	if err := fr.enrichFoldersWithStats(ctx, clientID, foldersToEnrich); err != nil {
		fmt.Printf("Error enriching folder with stats in GetFolder: %v\n", err)
	} else {
		folder = foldersToEnrich[0]
	}

	return &folder, nil
}

// UpdateFolder updates a folder
func (fr *FolderMongoRepository) UpdateFolder(clientID string, id primitive.ObjectID, folder domain.Folder) (*domain.Folder, error) {
	collection := fr.DB.Database(clientID).Collection("folders")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	folder.UpdatedAt = time.Now()
	opts := options.FindOneAndUpdate().SetReturnDocument(1)

	updateQuery := bson.M{"$set": folder}
	err := collection.FindOneAndUpdate(ctx, bson.M{"_id": id, "trash": bson.M{"$ne": true}}, updateQuery, opts).Decode(&folder)
	if err != nil {
		return nil, err
	}

	// Invalidate cache
	fr.invalidateFolderCache(clientID)

	// Enrich with real-time stats
	foldersToEnrich := []domain.Folder{folder}
	if err := fr.enrichFoldersWithStats(ctx, clientID, foldersToEnrich); err != nil {
		fmt.Printf("Error enriching folder with stats in UpdateFolder: %v\n", err)
	} else {
		folder = foldersToEnrich[0]
	}

	return &folder, nil
}

// DeleteFolder soft deletes a folder
func (fr *FolderMongoRepository) DeleteFolder(clientID string, id primitive.ObjectID, userID string) (*domain.Folder, error) {
	collection := fr.DB.Database(clientID).Collection("folders")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := map[string]interface{}{
		"updatedAt": time.Now(),
		"updatedBy": userID,
		"trash":     true,
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(1)

	var folder domain.Folder
	err := collection.FindOneAndUpdate(ctx, bson.M{"_id": id, "trash": bson.M{"$ne": true}}, bson.M{"$set": input}, opts).Decode(&folder)
	if err != nil {
		return nil, err
	}

	// Invalidate cache
	fr.invalidateFolderCache(clientID)

	// Enrich with real-time stats
	foldersToEnrich := []domain.Folder{folder}
	if err := fr.enrichFoldersWithStats(ctx, clientID, foldersToEnrich); err != nil {
		fmt.Printf("Error enriching folder with stats in DeleteFolder: %v\n", err)
	} else {
		folder = foldersToEnrich[0]
	}

	return &folder, nil
}

// GetFolderContents retrieves contents of a folder (files and subfolders)
func (fr *FolderMongoRepository) GetFolderContents(clientID string, folderID *primitive.ObjectID, query bson.M, page, perPage string) (*domain.DrivePagination, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filesCollection := fr.DB.Database(clientID).Collection("files")
	foldersCollection := fr.DB.Database(clientID).Collection("folders")

	// --- Get files in this folder ---
	filesFilter := bson.M{
		"trash": bson.M{"$ne": true},
		"type":  "drive", // Only get drive files
	}
	if folderID != nil {
		filesFilter["folderId"] = *folderID
	} else {
		filesFilter["$or"] = []bson.M{
			{"folderId": bson.M{"$exists": false}},
			{"folderId": nil},
		}
	}
	// Add further filters from query except folderId
	if query != nil {
		for k, v := range query {
			if k != "folderId" {
				filesFilter[k] = v
			}
		}
	}

	// Pagination parameters
	p, _ := strconv.Atoi(page)
	if p < 1 {
		p = 1
	}
	limit, _ := strconv.Atoi(perPage)
	if limit < 1 {
		limit = 10
	}

	// Count totals for files and folders
	filesTotal, err := filesCollection.CountDocuments(ctx, filesFilter)
	if err != nil {
		return nil, err
	}

	// --- Get subfolders in this folder ---
	foldersFilter := bson.M{"trash": bson.M{"$ne": true}}
	if folderID != nil {
		foldersFilter["parentId"] = *folderID
	} else {
		foldersFilter["$or"] = []bson.M{
			{"parentId": bson.M{"$exists": false}},
			{"parentId": nil},
		}
	}

	foldersTotal, err := foldersCollection.CountDocuments(ctx, foldersFilter)
	if err != nil {
		return nil, err
	}

	total := filesTotal + foldersTotal

	// Determine pagination window
	start := (p - 1) * limit
	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	if int64(start) >= total {
		// Out-of-range page, return empty
		return &domain.DrivePagination{
			Items: []domain.DriveItem{},
			Pagination: common.Pagination{
				TotalPages: float64(totalPages),
				PerPage:    int64(limit),
				TotalCount: total,
			},
		}, nil
	}

	end := start + limit
	if int64(end) > total {
		end = int(total)
	}

	// We always list folders first, then files (consistent but simple)
	var items []domain.DriveItem

	// Compute folder slice window
	folderStart := start
	if folderStart > int(foldersTotal) {
		folderStart = int(foldersTotal)
	}
	folderEnd := start + limit
	if folderEnd > int(foldersTotal) {
		folderEnd = int(foldersTotal)
	}

	// Fetch paginated folders
	if foldersTotal > 0 && folderStart < folderEnd {
		folderOpts := options.Find()
		folderOpts.SetSkip(int64(folderStart))
		folderOpts.SetLimit(int64(folderEnd - folderStart))
		foldersCursor, err := foldersCollection.Find(ctx, foldersFilter, folderOpts)
		if err != nil {
			return nil, err
		}
		var folders []domain.Folder
		if err = foldersCursor.All(ctx, &folders); err != nil {
			foldersCursor.Close(ctx)
			return nil, err
		}
		foldersCursor.Close(ctx)

		// Enrich folders with real-time stats
		if err := fr.enrichFoldersWithStats(ctx, clientID, folders); err != nil {
			fmt.Printf("Error enriching folders with stats in GetFolderContents: %v\n", err)
		}

		for i := range folders {
			items = append(items, &folders[i])
		}
	}

	// Compute remaining window for files
	remaining := end - folderEnd
	if remaining > 0 && filesTotal > 0 {
		fileStart := start - int(foldersTotal)
		if fileStart < 0 {
			fileStart = 0
		}
		fileOpts := options.Find().
			SetSkip(int64(fileStart)).
			SetLimit(int64(remaining))
		filesCursor, err := filesCollection.Find(ctx, filesFilter, fileOpts)
		if err != nil {
			return nil, err
		}
		var files []domain.File
		if err = filesCursor.All(ctx, &files); err != nil {
			filesCursor.Close(ctx)
			return nil, err
		}
		filesCursor.Close(ctx)

		for i := range files {
			items = append(items, &files[i])
		}
	}

	return &domain.DrivePagination{
		Items: items,
		Pagination: common.Pagination{
			TotalPages: float64(totalPages),
			PerPage:    int64(limit),
			TotalCount: total,
		},
	}, nil
}

// SearchItems searches for files and folders
func (fr *FolderMongoRepository) SearchItems(clientID string, searchQuery string, itemType string, folderID *primitive.ObjectID) ([]domain.DriveItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var items []domain.DriveItem

	// Search files
	if itemType == "all" || itemType == "file" {
		filesCollection := fr.DB.Database(clientID).Collection("files")
		filter := bson.M{
			"trash": bson.M{"$ne": true},
			"$or": []bson.M{
				{"originalName": bson.M{"$regex": searchQuery, "$options": "i"}},
				{"mimeType": bson.M{"$regex": searchQuery, "$options": "i"}},
			},
		}

		if folderID != nil {
			filter["folderId"] = folderID
		}

		filesCursor, err := filesCollection.Find(ctx, filter)
		if err == nil {
			var files []domain.File
			if err = filesCursor.All(ctx, &files); err == nil {
				for _, file := range files {
					items = append(items, &file)
				}
			}
			filesCursor.Close(ctx)
		}
	}

	// Search folders
	if itemType == "all" || itemType == "folder" {
		foldersCollection := fr.DB.Database(clientID).Collection("folders")
		filter := bson.M{
			"trash": bson.M{"$ne": true},
			"name":  bson.M{"$regex": searchQuery, "$options": "i"},
		}

		if folderID != nil {
			filter["parentId"] = folderID
		}

		foldersCursor, err := foldersCollection.Find(ctx, filter)
		if err == nil {
			var folders []domain.Folder
			if err = foldersCursor.All(ctx, &folders); err == nil {
				// Enrich folders with real-time stats
				if len(folders) > 0 {
					if err := fr.enrichFoldersWithStats(ctx, clientID, folders); err != nil {
						fmt.Printf("Error enriching folders with stats in SearchItems: %v\n", err)
					}
				}

				for _, folder := range folders {
					items = append(items, &folder)
				}
			}
			foldersCursor.Close(ctx)
		}
	}

	return items, nil
}

// MoveFolder moves a folder to a new parent
func (fr *FolderMongoRepository) MoveFolder(clientID string, folderID primitive.ObjectID, newParentID *primitive.ObjectID, userID string) (*domain.Folder, error) {
	collection := fr.DB.Database(clientID).Collection("folders")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get current folder
	var folder domain.Folder
	err := collection.FindOne(ctx, bson.M{"_id": folderID, "trash": bson.M{"$ne": true}}).Decode(&folder)
	if err != nil {
		return nil, err
	}

	// Generate new path based on new parent
	var newPath string
	if newParentID != nil {
		parentPath, err := fr.GetFolderPath(clientID, *newParentID)
		if err != nil {
			return nil, err
		}
		newPath = filepath.Join(parentPath, folder.Name)
	} else {
		newPath = folder.Name
	}

	// Update folder
	updateFields := bson.M{
		"parentId":  newParentID,
		"path":      newPath,
		"updatedAt": time.Now(),
		"updatedBy": userID,
	}

	updateQuery := bson.M{
		"$set": updateFields,
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(1)
	err = collection.FindOneAndUpdate(ctx, bson.M{"_id": folderID}, updateQuery, opts).Decode(&folder)
	if err != nil {
		return nil, err
	}

	// Invalidate cache
	fr.invalidateFolderCache(clientID)

	// Enrich with real-time stats
	foldersToEnrich := []domain.Folder{folder}
	if err := fr.enrichFoldersWithStats(ctx, clientID, foldersToEnrich); err != nil {
		fmt.Printf("Error enriching folder with stats in MoveFolder: %v\n", err)
	} else {
		folder = foldersToEnrich[0]
	}

	return &folder, nil
}

// GetRecentItems gets recently accessed items by combining GetFiles and GetFolders
func (fr *FolderMongoRepository) GetRecentItems(clientID string, userID string, limit int) ([]domain.DriveItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Ensure limit is sane
	if limit < 1 {
		limit = 20
	}

	filesCollection := fr.DB.Database(clientID).Collection("files")

	// Define the pipeline
	// 1. We start with files, calculate a sorting date, tag them as 'file', sort, and limit.
	// 2. We $unionWith folders, applying the same logic to them.
	// 3. We sort the combined list and take the final top N items.
	pipeline := mongo.Pipeline{
		// --- Stage 1: Filter and Prep Files ---
		{{Key: "$match", Value: bson.M{
			"trash": bson.M{"$ne": true},
			"type":  "drive", // Explicitly ensure we only get drive files if that schema exists
		}}},
		{{Key: "$addFields", Value: bson.M{
			"typeDiscriminator": "file",
			// Create a unified sort date: use updatedAt, fallback to createdAt
			"sortDate": bson.M{"$ifNull": bson.A{"$updatedAt", "$createdAt"}},
		}}},
		{{Key: "$sort", Value: bson.M{"sortDate": -1}}},
		{{Key: "$limit", Value: int64(limit)}},

		// --- Stage 2: Union with Folders ---
		{{Key: "$unionWith", Value: bson.M{
			"coll": "folders",
			"pipeline": mongo.Pipeline{
				{{Key: "$match", Value: bson.M{
					"trash": bson.M{"$ne": true},
				}}},
				{{Key: "$addFields", Value: bson.M{
					"typeDiscriminator": "folder",
					"sortDate":          bson.M{"$ifNull": bson.A{"$updatedAt", "$createdAt"}},
				}}},
				{{Key: "$sort", Value: bson.M{"sortDate": -1}}},
				{{Key: "$limit", Value: int64(limit)}},
			},
		}}},

		// --- Stage 3: Final Merge and Sort ---
		{{Key: "$sort", Value: bson.M{"sortDate": -1}}},
		{{Key: "$limit", Value: int64(limit)}},
	}

	cursor, err := filesCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Results container
	var items []domain.DriveItem

	// Pointers to folders for batch enrichment later
	var folderPtrs []*domain.Folder
	var foldersToEnrich []domain.Folder

	// Iterate raw BSON to handle polymorphism
	for cursor.Next(ctx) {
		raw := cursor.Current

		// Check the typeDiscriminator we added in the pipeline
		typeVal, err := raw.LookupErr("typeDiscriminator")
		if err != nil {
			continue // Should not happen given the pipeline
		}

		itemType, ok := typeVal.StringValueOK()
		if !ok {
			continue
		}

		if itemType == "file" {
			var file domain.File
			if err := bson.Unmarshal(raw, &file); err == nil {
				items = append(items, &file)
			}
		} else if itemType == "folder" {
			var folder domain.Folder
			if err := bson.Unmarshal(raw, &folder); err == nil {
				items = append(items, &folder)
				// Track for stats enrichment
				folderPtrs = append(folderPtrs, items[len(items)-1].(*domain.Folder))
				foldersToEnrich = append(foldersToEnrich, folder)
			}
		}
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	// Batch enrich folders with real-time stats (file counts, size)
	if len(foldersToEnrich) > 0 {
		if err := fr.enrichFoldersWithStats(ctx, clientID, foldersToEnrich); err != nil {
			fmt.Printf("Error enriching folders with stats in GetRecentItems: %v\n", err)
		} else {
			// Write the enriched stats back to the pointers in the `items` slice
			for i, enriched := range foldersToEnrich {
				if i < len(folderPtrs) {
					folderPtrs[i].FileCount = enriched.FileCount
					folderPtrs[i].Size = enriched.Size
				}
			}
		}
	}

	return items, nil
}

// Helper methods
func (fr *FolderMongoRepository) generateFolderPath(clientID string, parentID *primitive.ObjectID, folderName string) string {
	if parentID == nil {
		return folderName
	}

	// Get parent folder path
	parentFolder, err := fr.GetFolder(clientID, *parentID)
	if err != nil {
		return folderName
	}

	return parentFolder.Path + "/" + folderName
}

func (fr *FolderMongoRepository) invalidateFolderCache(clientID string) {
	// Invalidate folder-related cache keys
	patterns := []string{
		fmt.Sprintf("folder:*:c:%s:*", clientID),
		fmt.Sprintf("drive:*:c:%s:*", clientID),
	}

	for _, pattern := range patterns {
		fr.Cache.BatchDeletionKeysByPattern(pattern, 100)
	}

}
func (fr *FolderMongoRepository) DeepSearchItems(clientID string, searchQuery string, folderID *primitive.ObjectID) ([]domain.DriveItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var allFolderIDs []primitive.ObjectID
	visited := map[primitive.ObjectID]bool{}
	queue := []primitive.ObjectID{}
	if folderID != nil {
		queue = append(queue, *folderID)
	}
	// BFS for all folders
	for len(queue) > 0 {
		fid := queue[0]
		queue = queue[1:]
		if visited[fid] {
			continue
		}
		visited[fid] = true
		allFolderIDs = append(allFolderIDs, fid)
		foldersCollection := fr.DB.Database(clientID).Collection("folders")
		cur, _ := foldersCollection.Find(ctx, bson.M{"trash": bson.M{"$ne": true}, "parentId": fid})
		var subs []domain.Folder
		_ = cur.All(ctx, &subs)
		cur.Close(ctx)
		for _, f := range subs {
			queue = append(queue, f.Id)
		}
	}
	if folderID == nil {
		allFolderIDs = nil // unrestricted search
	}
	var items []domain.DriveItem
	filesCollection := fr.DB.Database(clientID).Collection("files")
	foldersCollection := fr.DB.Database(clientID).Collection("folders")
	fileFilter := bson.M{
		"trash": bson.M{"$ne": true},
		"type":  "drive", // Only search drive files
		"$or": []bson.M{
			{"originalName": bson.M{"$regex": searchQuery, "$options": "i"}},
			{"mimeType": bson.M{"$regex": searchQuery, "$options": "i"}},
		},
	}
	if allFolderIDs != nil {
		fileFilter["folderId"] = bson.M{"$in": allFolderIDs}
	}
	filesCursor, _ := filesCollection.Find(ctx, fileFilter)
	var files []domain.File
	_ = filesCursor.All(ctx, &files)
	for _, f := range files {
		items = append(items, &f)
	}
	filesCursor.Close(ctx)
	folderFilter := bson.M{
		"trash": bson.M{"$ne": true},
		"name":  bson.M{"$regex": searchQuery, "$options": "i"},
	}
	if allFolderIDs != nil {
		folderFilter["_id"] = bson.M{"$in": allFolderIDs}
	}
	foldersCursor, _ := foldersCollection.Find(ctx, folderFilter)
	var folders []domain.Folder
	_ = foldersCursor.All(ctx, &folders)

	// Enrich folders with real-time stats
	if len(folders) > 0 {
		if err := fr.enrichFoldersWithStats(ctx, clientID, folders); err != nil {
			fmt.Printf("Error enriching folders with stats in DeepSearchItems: %v\n", err)
		}
	}

	for _, f := range folders {
		items = append(items, &f)
	}
	foldersCursor.Close(ctx)
	return items, nil
}

// GetFolders returns paginated folders with filtering
func (fr *FolderMongoRepository) GetFolders(clientID string, filter bson.M, page, perPage string) (*domain.FolderPagination, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := fr.DB.Database(clientID).Collection("folders")

	// Parse pagination
	p, _ := strconv.Atoi(page)
	if p < 1 {
		p = 1
	}
	limit, _ := strconv.Atoi(perPage)
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	// Count total documents
	totalCount, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Calculate pagination
	start := (p - 1) * limit

	// Find documents with pagination
	opts := options.Find().
		SetSkip(int64(start)).
		SetLimit(int64(limit)).
		SetSort(bson.M{"createdAt": -1})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var folders []domain.Folder
	if err = cursor.All(ctx, &folders); err != nil {
		return nil, err
	}

	// Enrich folders with real-time stats
	if err := fr.enrichFoldersWithStats(ctx, clientID, folders); err != nil {
		// Log error but continue with potentially stale/zero stats
		fmt.Printf("Error enriching folders with stats: %v\n", err)
	}

	return &domain.FolderPagination{
		Folders: folders,
		Pagination: common.Pagination{
			TotalPages: math.Ceil(float64(totalCount) / float64(limit)),
			PerPage:    int64(limit),
			TotalCount: totalCount,
		},
	}, nil
}

func (fr *FolderMongoRepository) HardDeleteTrashedFolders(clientID string) (int64, error) {
	collection := fr.DB.Database(clientID).Collection("folders")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{"trash": true}
	res, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}

	// Invalidate cache
	fr.invalidateFolderCache(clientID)

	return res.DeletedCount, nil
}

// GetFolderPath returns the full path of a folder by traversing up the hierarchy
func (fr *FolderMongoRepository) GetFolderPath(clientID string, folderID primitive.ObjectID) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := fr.DB.Database(clientID).Collection("folders")

	// Build path by traversing up the hierarchy
	var pathParts []string
	currentID := folderID

	for currentID != primitive.NilObjectID {
		var folder domain.Folder
		err := collection.FindOne(ctx, bson.M{"_id": currentID, "trash": bson.M{"$ne": true}}).Decode(&folder)
		if err != nil {
			return "", err
		}

		pathParts = append([]string{folder.Name}, pathParts...)
		if folder.ParentId != nil {
			currentID = *folder.ParentId
		} else {
			currentID = primitive.NilObjectID
		}
	}

	// Join path parts
	if len(pathParts) == 0 {
		return "", fmt.Errorf("folder not found")
	}

	return filepath.Join(pathParts...), nil
}
