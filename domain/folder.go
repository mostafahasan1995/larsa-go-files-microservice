package domain

import (
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Folder represents a folder in the drive system
type Folder struct {
	Id        primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	Name      string              `bson:"name" json:"name"`
	ParentId  *primitive.ObjectID `bson:"parentId,omitempty" json:"parentId"`
	Path      string              `bson:"path" json:"path"`
	Service   string              `bson:"service" json:"service"`
	CreatedBy string              `bson:"createdBy" json:"createdBy"`
	UpdatedBy string              `bson:"updatedBy" json:"updatedBy"`
	CreatedAt time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time           `bson:"updatedAt" json:"updatedAt"`
	Trash     bool                `bson:"trash" json:"trash"`
	Deletable bool                `bson:"isDeletable" json:"isDeletable"`
	Movable   bool                `bson:"isMovable" json:"isMovable"`
	Copyable  bool                `bson:"isCopyable" json:"isCopyable"`
	FileCount int                 `bson:"-" json:"fileCount"`
	Size      int64               `bson:"-" json:"size"`
}

// DriveItem represents a common interface for files and folders
type DriveItem interface {
	GetId() primitive.ObjectID
	GetName() string
	GetPath() string
	GetParentId() *primitive.ObjectID
	GetType() string // "file" or "folder"
	GetSize() int64
	GetCreatedBy() string
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
	IsTrash() bool
	IsDeletable() bool
	IsMovable() bool
	IsCopyable() bool
}

// Implement DriveItem interface for Folder
func (f *Folder) GetId() primitive.ObjectID {
	return f.Id
}

func (f *Folder) GetName() string {
	return f.Name
}

func (f *Folder) GetPath() string {
	return f.Path
}

func (f *Folder) GetParentId() *primitive.ObjectID {
	return f.ParentId
}

func (f *Folder) GetType() string {
	return "folder"
}

func (f *Folder) GetSize() int64 {
	return f.Size
}

func (f *Folder) GetCreatedBy() string {
	return f.CreatedBy
}

func (f *Folder) GetCreatedAt() time.Time {
	return f.CreatedAt
}

func (f *Folder) GetUpdatedAt() time.Time {
	return f.UpdatedAt
}

func (f *Folder) IsTrash() bool {
	return f.Trash
}

func (f *Folder) IsDeletable() bool {
	return f.Deletable
}

func (f *Folder) IsMovable() bool {
	return f.Movable
}

func (f *Folder) IsCopyable() bool {
	return f.Copyable
}

// Implement DriveItem interface for File
func (f *File) GetId() primitive.ObjectID {
	return f.Id
}

func (f *File) GetName() string {
	return f.OriginalName
}

func (f *File) GetPath() string {
	return f.Path
}

func (f *File) GetParentId() *primitive.ObjectID {
	return f.FolderId
}

func (f *File) GetType() string {
	return "file"
}

func (f *File) GetSize() int64 {
	return f.Size
}

func (f *File) GetCreatedBy() string {
	return f.CreatedBy
}

func (f *File) GetCreatedAt() time.Time {
	return f.CreatedAt
}

func (f *File) GetUpdatedAt() time.Time {
	return f.UpdatedAt
}

func (f *File) IsTrash() bool {
	return f.Trash
}

func (f *File) IsDeletable() bool {
	return true
}

func (f *File) IsMovable() bool {
	return true
}

func (f *File) IsCopyable() bool {
	return true
}

// DrivePagination represents paginated drive items
type DrivePagination struct {
	Items      []DriveItem       `json:"items"`
	Pagination common.Pagination `json:"pagination"`
}

// FolderPagination combines a list of Folders with pagination info
type FolderPagination struct {
	Folders    []Folder          `json:"folders"`
	Pagination common.Pagination `json:"pagination"`
}

// FolderTreeNode represents a node in the folder tree
type FolderTreeNode struct {
	Folder   *Folder          `json:"folder,omitempty"`
	Files    []File           `json:"files,omitempty"`
	Children []FolderTreeNode `json:"children,omitempty"`
}

// FolderTree represents the complete folder hierarchy
type FolderTree struct {
	Root FolderTreeNode `json:"root"`
}

// DriveSearchRequest represents search parameters
type DriveSearchRequest struct {
	Query     string              `json:"query"`
	ItemType  string              `json:"itemType"` // "file", "folder", or "all"
	FolderId  *primitive.ObjectID `json:"folderId"`
	SortBy    string              `json:"sortBy"`    // "name", "date", "size", "type"
	SortOrder string              `json:"sortOrder"` // "asc", "desc"
	Page      int                 `json:"page"`
	PerPage   int                 `json:"perPage"`
}

// DriveBulkOperation represents bulk operations
type DriveBulkOperation struct {
	ItemIds  []primitive.ObjectID `json:"itemIds"`
	Action   string               `json:"action"`   // "move", "delete", "copy"
	TargetId *primitive.ObjectID  `json:"targetId"` // For move/copy operations
}

// BulkOperationResponse represents the result of a bulk operation
type BulkOperationResponse struct {
	SuccessCount int               `json:"successCount"`
	FailureCount int               `json:"failureCount"`
	Errors       map[string]string `json:"errors,omitempty"` // map[itemId]errorMessage
}

// Share represents sharing permissions
type Share struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	ItemId      primitive.ObjectID `bson:"itemId" json:"itemId"`
	ItemType    string             `bson:"itemType" json:"itemType"`       // "file" or "folder"
	SharedWith  string             `bson:"sharedWith" json:"sharedWith"`   // User ID
	Permissions []string           `bson:"permissions" json:"permissions"` // ["read", "write", "admin"]
	SharedBy    string             `bson:"sharedBy" json:"sharedBy"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	ExpiresAt   *time.Time         `bson:"expiresAt,omitempty" json:"expiresAt"`
	IsActive    bool               `bson:"isActive" json:"isActive"`
}
