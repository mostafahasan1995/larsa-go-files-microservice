package domain

import (
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FilePagination struct {
	Files      []File            `json:"files"`
	Pagination common.Pagination `json:"pagination"`
}

type File struct {
	Id           primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	Path         string              `bson:"path" json:"path"`
	FolderId     *primitive.ObjectID `bson:"folderId,omitempty" json:"folderId"`
	Size         int64               `bson:"size" json:"size"`
	MimeType     string              `bson:"mimeType" json:"mimeType"`
	Variants     []string            `bson:"variants" json:"variants"`
	OriginalName string              `bson:"originalName" json:"originalName"`
	Service      string              `bson:"service" json:"service"`
	Type         string              `bson:"type,omitempty" json:"type,omitempty"` // "drive", "upload", etc.
	Expire       time.Time           `bson:"expire, omitempty" json:"expire"`
	IsS3         bool                `bson:"isS3,omitempty" json:"isS3,omitempty"`
	Trash        bool                `bson:"trash" json:"trash"`
	IsProduct    bool                `bson:"isProduct" json:"isProduct"`
	UpdatedBy    string              `bson:"updatedBy" json:"updatedBy"`
	CreatedBy    string              `bson:"createdBy" json:"createdBy"`
	CreatedAt    time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time           `bson:"updatedAt" json:"updatedAt"`
}

type FileRepositoty interface {
	Store(string, File) (*File, error)
	StoreS3(string, File, []byte, string) (*File, error)
	Update(string, primitive.ObjectID, File, bson.M) (*File, error)
	Delete(string, primitive.ObjectID, string) (*File, error)
	GetByID(string, primitive.ObjectID) (*File, error)
	GetOne(string, bson.M) (*File, error)
	GetAll(string, string, bson.M) ([]File, error)
	Get(string, string, bson.M, string, string, string) (*FilePagination, error)
	GetFiles(string, bson.M, string, string, string) (*FilePagination, error)
}

type FileUsecase interface {
	Store(*common.HeaderParams, File, string) (*File, error)
	StoreS3(*common.HeaderParams, File, []byte, string) (*File, error)
	Update(*common.HeaderParams, primitive.ObjectID, File) (*File, error)
	Delete(*common.HeaderParams, primitive.ObjectID, string) (*File, error)
	GetByID(*common.HeaderParams, primitive.ObjectID) (*File, error)
	GetOne(*common.HeaderParams, bson.M) (*File, error)
	GetAll(*common.HeaderParams, string, bson.M) ([]File, error)
	Get(*common.HeaderParams, string, bson.M, string, string, string) (*FilePagination, error)
	GenerateStream(*common.HeaderParams, File) error
	GenerateStreamS3(*common.HeaderParams, File) error
}
