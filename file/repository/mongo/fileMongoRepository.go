package mongo

import (
	"os"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"larsa-files/domain"
	"log"
	"math"

	// "net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"git.larsa.io/mahdawi/microservices-commons.git/cache"
	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type fileMongoRepository struct {
	DB    *mongo.Client
	Cache cache.Cache
}

func NewMongoFileRepository(db *mongo.Client, cache cache.Cache) *fileMongoRepository {
	return &fileMongoRepository{
		DB:    db,
		Cache: cache,
	}
}

func (fp *fileMongoRepository) GetAll(clientID string, reqURL string, query bson.M) ([]domain.File, error) {
	cacheKey := fmt.Sprintf("files:p:c:%v:all:%v", clientID, reqURL)

	// 1. Try to get from Cache
	cachRes, err := fp.Cache.GetByKey(cacheKey)
	if err == nil {
		var res []domain.File
		// If unmarshal works, return immediately.
		// If it fails, ignore the error and proceed to DB (fallback).
		if jsonErr := json.Unmarshal([]byte(cachRes), &res); jsonErr == nil {
			return res, nil
		}
	}

	// 2. Setup Database Context
	collection := fp.DB.Database(clientID).Collection("files")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 3. Build Filter Logic
	filter := bson.M{"trash": bson.M{"$ne": true}}
	if query != nil {
		query["trash"] = bson.M{"$ne": true}

		// Handle _id conversion logic
		if v, ok := query["_id"]; ok {
			strID, strOk := v.(string)
			if !strOk {
				vmap, _ := query["_id"].(map[string]interface{})
				if _, ok := vmap["$eq"]; ok {
					strID, _ := vmap["$eq"].(string)
					objID, _ := primitive.ObjectIDFromHex(strID)
					query["_id"] = objID
				} else if _, ok := vmap["$in"]; ok {
					arrIdstr, _ := vmap["$in"].([]interface{})
					var arrIdObj []primitive.ObjectID
					for _, id := range arrIdstr {
						strID, _ := id.(string)
						objID, _ := primitive.ObjectIDFromHex(strID)
						arrIdObj = append(arrIdObj, objID)
					}
					query["_id"] = bson.M{"$in": arrIdObj}
				}
			} else {
				objID, _ := primitive.ObjectIDFromHex(strID)
				query["_id"] = objID
			}
		}
		filter = query
	}

	// 4. Execute Query
	var res []domain.File
	curr, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer curr.Close(ctx)

	for curr.Next(ctx) {
		var doc domain.File
		err := curr.Decode(&doc)
		if err != nil {
			return nil, err
		}
		res = append(res, doc)
	}

	if err := curr.Err(); err != nil {
		return nil, err
	}

	// Only cache if we actually found results (optional, depending on your business logic)
	// Passing '0' usually implies no expiration or default expiration.
	if len(res) > 0 {
		fp.Cache.CacheByKey(cacheKey, res, 0)
	}

	return res, nil
}

func (fp *fileMongoRepository) Get(clientID string, reqURL string, query bson.M, page string, perPage string, sort string) (*domain.FilePagination, error) {

	cacheKey := "files:p:c:" + clientID + ":" + reqURL + ":" + sort
	cacheRes, _ := fp.Cache.GetByKey(cacheKey)

	if true {
		collection := fp.DB.Database(clientID).Collection("files")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		filter := bson.M{"trash": bson.M{"$ne": true}}
		if query != nil {
			query["trash"] = bson.M{"$ne": true}
			if v, ok := query["_id"]; ok {
				strID, strOk := v.(string)
				if !strOk {
					vmap, _ := query["_id"].(map[string]interface{})
					if _, ok := vmap["$eq"]; ok {
						strID, _ := vmap["$eq"].(string)
						objID, _ := primitive.ObjectIDFromHex(strID)
						query["_id"] = objID
					} else if _, ok := vmap["$in"]; ok {
						arrIdstr, _ := vmap["$in"].([]interface{})
						var arrIdObj []primitive.ObjectID
						for _, id := range arrIdstr {
							strID, _ := id.(string)
							objID, _ := primitive.ObjectIDFromHex(strID)
							arrIdObj = append(arrIdObj, objID)
						}
						query["_id"] = bson.M{"$in": arrIdObj}
					}
				} else {
					objID, _ := primitive.ObjectIDFromHex(strID)
					query["_id"] = objID
				}
			}
			filter = query
		}

		totalCount, errCount := collection.CountDocuments(ctx, filter, nil)

		if errCount != nil {
			return nil, errCount
		}
		options := options.Find()
		_, limit := common.Paginator(page, perPage, options)

		if sort != "" {
			var sortBson bson.M
			err := json.Unmarshal([]byte(sort), &sortBson)
			if err != nil {
				// if not a json object, try to use it as a field name for ascending sort
				options.SetSort(bson.M{sort: 1})
			} else {
				options.SetSort(sortBson)
			}
		}

		totalpages := math.Ceil(float64(totalCount) / float64(limit))

		cur, err := collection.Find(ctx, filter, options)
		if err != nil {
			return nil, err
		}
		defer cur.Close(ctx)
		result := []domain.File{}
		for cur.Next(ctx) {
			var file domain.File
			err := cur.Decode(&file)
			if err != nil {
				return nil, err
			}
			result = append(result, file)

		}
		if err := cur.Err(); err != nil {
			return nil, err
		}
		r := domain.FilePagination{
			Files: result,
			Pagination: common.Pagination{
				TotalPages: totalpages,
				PerPage:    limit,
				TotalCount: totalCount,
			},
		}

		fp.Cache.CacheByKey(cacheKey, r, 0)

		return &r, nil
	} else {
		var r domain.FilePagination
		err := json.Unmarshal([]byte(cacheRes), &r)
		if err != nil {
			return nil, err
		}
		return &r, nil
	}
}

func (fp *fileMongoRepository) GetByID(clientID string, id primitive.ObjectID) (*domain.File, error) {
	collection := fp.DB.Database(clientID).Collection("files")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s := new(domain.File)

	cacheKey := "files:g:c:" + clientID + ":" + id.Hex()
	cacheRes, cacheErr := fp.Cache.GetByKey(cacheKey)
	if cacheErr != nil {
		err := collection.FindOne(ctx, bson.M{"_id": id, "trash": bson.M{"$ne": true}}).Decode(&s)
		if err != nil {
			return nil, err
		}
		fp.Cache.CacheByKey(cacheKey, s, 0)
	} else {
		err := json.Unmarshal([]byte(cacheRes), &s)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (fp *fileMongoRepository) GetOne(clientID string, query bson.M) (*domain.File, error) {
	collection := fp.DB.Database(clientID).Collection("files")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s := new(domain.File)

	reqURL, _ := json.Marshal(query)
	cacheKey := "files:p:c:" + clientID + ":" + string(reqURL)
	if query == nil {
		return nil, errors.New("query is empty")
	}
	query["trash"] = bson.M{"$ne": true}

	cacheRes, cacheErr := fp.Cache.GetByKey(cacheKey)
	if cacheErr != nil {
		err := collection.FindOne(ctx, query).Decode(&s)
		if err != nil {
			return nil, err
		}
		fp.Cache.CacheByKey(cacheKey, s, 0)
	} else {
		err := json.Unmarshal([]byte(cacheRes), &s)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (fp *fileMongoRepository) Store(clientID string, s domain.File) (*domain.File, error) {

	collection := fp.DB.Database(clientID).Collection("files")
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	s.Trash = false

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := collection.InsertOne(ctx, &s)

	if err != nil {
		return nil, err
	}

	s.Id = res.InsertedID.(primitive.ObjectID)

	// Invalidate cache
	// Invalidate file listing cache to include new file
	batchDeletionKey := "files:p:c:" + clientID + ":*"
	fp.Cache.BatchDeletionKeysByPattern(batchDeletionKey, 100)

	// Also invalidate folder and drive cache to reflect new file immediately
	folderCachePatterns := []string{
		fmt.Sprintf("folder:*:c:%s:*", clientID),
		fmt.Sprintf("drive:*:c:%s:*", clientID),
	}
	for _, pattern := range folderCachePatterns {
		fp.Cache.BatchDeletionKeysByPattern(pattern, 100)
	}

	return &s, nil
}

func (fp *fileMongoRepository) StoreS3(clientID string, s domain.File, fileData []byte, bucketName string) (*domain.File, error) {
	// Initialize MongoDB collection
	collection := fp.DB.Database(clientID).Collection("files")
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	s.Trash = false

	// Set up Larsa Object Storage (S3-compatible) session
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-east-1"),                                                                                  // Use any region (not strictly necessary for Larsa)
		Endpoint:         aws.String("https://prod-bucket-larsa.nbg1.your-objectstorage.com"),                                      // Larsa Object Storage URL
		Credentials:      credentials.NewStaticCredentials(os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), ""), // Larsa Access Key & Secret Key
		S3ForcePathStyle: aws.Bool(true),                                                                                           // Force path-style URLs (typical for S3-compatible services)
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := collection.InsertOne(ctx, &s)
	if err != nil {
		return nil, err
	}

	// Set the MongoDB ID in the file object
	s.Id = res.InsertedID.(primitive.ObjectID)

	s3Client := s3.New(sess)

	// Create a unique S3 object key
	objectKey := clientID + "/" + res.InsertedID.(primitive.ObjectID).Hex()
	// contentType := http.DetectContentType(fileData)
	// Upload file to Larsa Object Storage (S3-compatible)
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		ContentType: aws.String("application/octet-stream"),
		Body:        bytes.NewReader(fileData),
		ACL:         aws.String("public-read"), // You can adjust the ACL as needed
	})
	if err != nil {
		return nil, err
	}

	// Store the file URL in the file metadata
	s.Path = "https://" + "larsa-test.hel1.your-objectstorage.com/" + bucketName + "/" + objectKey

	log.Println("Pathssssssssssssssssssssss : ")
	log.Println(s.Path)

	// Insert file metadata into MongoDB

	// Perform cache invalidation
	batchDeletionKey := "files:p:c:" + clientID + ":*"
	fp.Cache.BatchDeletionKeysByPattern(batchDeletionKey, 100)

	// Also invalidate folder and drive cache to reflect new file immediately
	folderCachePatterns := []string{
		fmt.Sprintf("folder:*:c:%s:*", clientID),
		fmt.Sprintf("drive:*:c:%s:*", clientID),
	}
	for _, pattern := range folderCachePatterns {
		fp.Cache.BatchDeletionKeysByPattern(pattern, 100)
	}

	return &s, nil
}

func (fp *fileMongoRepository) GetFileS3(clientID string, fileID string, bucketName string) (*domain.File, string, error) {
	// Initialize MongoDB collection
	collection := fp.DB.Database(clientID).Collection("files")

	// Convert fileID string to ObjectID
	objectID, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return nil, "", err
	}

	// Find file metadata in MongoDB
	var file domain.File
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&file)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, "", err // File not found
		}
		return nil, "", err
	}

	// Extract S3 key from file metadata
	s3Key := file.Path[len("https://"+bucketName+"larsa-test.hel1.your-objectstorage.com/"):] // Remove bucket URL prefix

	// Set up Larsa Object Storage (S3-compatible) session
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-east-1"),                                                                                  // Region (may not be needed)
		Endpoint:         aws.String("https://prod-bucket-larsa.nbg1.your-objectstorage.com"),                                      // Larsa Object Storage URL
		Credentials:      credentials.NewStaticCredentials(os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), ""), // Larsa Access Key & Secret Key
		S3ForcePathStyle: aws.Bool(true),                                                                                           // Force path-style URLs (typical for S3-compatible services)
	})
	if err != nil {
		return nil, "", err
	}

	s3Client := s3.New(sess)

	// Generate a pre-signed URL for the file in Larsa Object Storage
	req, _ := s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
	})

	// Set an expiration time for the pre-signed URL
	preSignedURL, err := req.Presign(15 * time.Minute) // Valid for 15 minutes
	if err != nil {
		return nil, "", err
	}

	return &file, preSignedURL, nil
}

func (sp *fileMongoRepository) Delete(clientID string, id primitive.ObjectID, uID string) (*domain.File, error) {
	collection := sp.DB.Database(clientID).Collection("files")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s := new(domain.File)
	// s.UpdatedAt = time.Now()
	// s.UpdatedBy = uID
	// s.Trash = true

	input := map[string]interface{}{
		"updatedAt": time.Now(),
		"updatedBy": uID,
		"trash":     true,
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(1)
	err := collection.FindOneAndUpdate(ctx, bson.M{"_id": id, "trash": bson.M{"$ne": true}}, bson.M{"$set": input}, opts).Decode(&s)
	if err != nil {
		return nil, err
	}

	// Invalidate caches for this file and any paginated file listings
	cacheKey := "files:g:c:" + clientID + ":" + id.Hex()
	sp.Cache.DeleteKey(cacheKey)
	cacheDeletionKeyPattern := "files:p:c:" + clientID + ":*"
	sp.Cache.BatchDeletionKeysByPattern(cacheDeletionKeyPattern, 100)

	// Invalidate folder listing cache
	folderCacheKey := fmt.Sprintf("folder:*:c:%s:*", clientID)
	sp.Cache.BatchDeletionKeysByPattern(folderCacheKey, 100)
	// Invalidate drive contents cache
	driveCacheKey := fmt.Sprintf("drive:*:c:%s:*", clientID)
	sp.Cache.BatchDeletionKeysByPattern(driveCacheKey, 100)

	return s, nil
}

func (sp *fileMongoRepository) Update(clientID string, id primitive.ObjectID, file domain.File, updateQuery bson.M) (*domain.File, error) {
	collection := sp.DB.Database(clientID).Collection("files")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get existing file to track folder changes
	var existingFile domain.File
	err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&existingFile)
	if err != nil {
		return nil, err
	}

	file.UpdatedAt = time.Now()
	opts := options.FindOneAndUpdate().SetReturnDocument(1)
	if updateQuery == nil {
		// Only update specific fields, not the entire file struct
		updateFields := bson.M{
			"folderId":  file.FolderId,
			"updatedAt": file.UpdatedAt,
		}
		if file.UpdatedBy != "" {
			updateFields["updatedBy"] = file.UpdatedBy
		}
		updateQuery = bson.M{"$set": updateFields}
	}
	var updated domain.File
	err = collection.FindOneAndUpdate(ctx, bson.M{"_id": id, "trash": bson.M{"$ne": true}}, updateQuery, opts).Decode(&updated)
	if err != nil {
		return nil, err
	}

	// Update folder counts if folder changed
	// Only proceed if the file was not in trash and update succeeded
	if !existingFile.Trash {
		oldFolderID := existingFile.FolderId
		newFolderID := updated.FolderId

		// Check if folder changed
		folderChanged := false
		if oldFolderID == nil && newFolderID != nil {
			folderChanged = true
		} else if oldFolderID != nil && newFolderID == nil {
			folderChanged = true
		} else if oldFolderID != nil && newFolderID != nil && *oldFolderID != *newFolderID {
			folderChanged = true
		}

		if folderChanged {
			// Invalidate folder-related caches
			folderCacheKey := fmt.Sprintf("folder:*:c:%s:*", clientID)
			sp.Cache.BatchDeletionKeysByPattern(folderCacheKey, 100)
			driveCacheKey := fmt.Sprintf("drive:*:c:%s:*", clientID)
			sp.Cache.BatchDeletionKeysByPattern(driveCacheKey, 100)
		}
	}

	// Invalidate caches for this file and any paginated file listings
	cacheKey := "files:g:c:" + clientID + ":" + id.Hex()
	sp.Cache.DeleteKey(cacheKey)
	cacheDeletionKeyPattern := "files:p:c:" + clientID + ":*"
	sp.Cache.BatchDeletionKeysByPattern(cacheDeletionKeyPattern, 100)

	// Invalidate folder and drive contents cache (essential for move/rename to reflect immediately)
	folderCachePatterns := []string{
		fmt.Sprintf("folder:*:c:%s:*", clientID),
		fmt.Sprintf("drive:*:c:%s:*", clientID),
	}
	for _, pattern := range folderCachePatterns {
		sp.Cache.BatchDeletionKeysByPattern(pattern, 100)
	}

	return &updated, nil
}

// GetFiles returns paginated files with filtering (wrapper around Get method)
func (fp *fileMongoRepository) GetFiles(clientID string, filter bson.M, page, perPage string, sort string) (*domain.FilePagination, error) {
	return fp.Get(clientID, "", filter, page, perPage, sort)
}
