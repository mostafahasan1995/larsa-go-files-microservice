package file

import (
	"fmt"
	"larsa-files/domain"
	"path/filepath"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"larsa-files/pkg/optimization"
	"larsa-files/pkg/streaming"
	"larsa-files/utils"
)

type fileUsecase struct {
	Repo domain.FileRepositoty
}

func NewFileUsecase(repo domain.FileRepositoty) *fileUsecase {
	return &fileUsecase{
		Repo: repo,
	}
}

func (fu *fileUsecase) GetAll(headerParams *common.HeaderParams, reqURL string, query bson.M) ([]domain.File, error) {
	return fu.Repo.GetAll(headerParams.Client, reqURL, query)
}

func (fu *fileUsecase) Get(headerParams *common.HeaderParams, reqURL string, query bson.M, page string, perPage string, sort string) (*domain.FilePagination, error) {
	return fu.Repo.Get(headerParams.Client, reqURL, query, page, perPage, sort)
}

func (fu *fileUsecase) GetByID(headerParams *common.HeaderParams, id primitive.ObjectID) (*domain.File, error) {
	return fu.Repo.GetByID(headerParams.Client, id)
}

func (fu *fileUsecase) GetOne(headerParams *common.HeaderParams, query bson.M) (*domain.File, error) {
	return fu.Repo.GetOne(headerParams.Client, query)
}

func (fu *fileUsecase) Store(headerParams *common.HeaderParams, file domain.File, query string) (*domain.File, error) {
	return fu.Repo.Store(headerParams.Client, file)
}

func (fu *fileUsecase) Delete(headerParams *common.HeaderParams, id primitive.ObjectID, userID string) (*domain.File, error) {
	return fu.Repo.Delete(headerParams.Client, id, userID)
}

func (fu *fileUsecase) Update(headerParams *common.HeaderParams, id primitive.ObjectID, file domain.File) (*domain.File, error) {
	return fu.Repo.Update(headerParams.Client, id, file, nil)
}

func (fu *fileUsecase) GenerateStream(headerParams *common.HeaderParams, file domain.File) error {

	folder := utils.UploadPath(headerParams)

	dir := filepath.Join(folder, file.Path)

	fileData := optimization.GetFileData(dir)

	fmt.Printf("File exists\n")

	//empty file variants
	_, err := fu.Repo.Update(headerParams.Client, file.Id, domain.File{}, bson.M{"$set": bson.M{"variants": []string{}}})
	if err != nil {
		fmt.Println(err)
	}

	//DASH
	go optimization.GenerateStream(headerParams, fileData, dir)

	//HLS
	mainfestFile, err := streaming.GenerateMainfestHLS(headerParams, file)
	if err != nil {
		fmt.Println(err)
	}
	for mf := range mainfestFile {
		fmt.Println("mf")

		fmt.Println(mf)
	}

	//update file variants
	_, err = fu.Repo.Update(headerParams.Client, file.Id, domain.File{}, bson.M{"$push": bson.M{"variants": mainfestFile}})
	if err != nil {
		fmt.Println(err)
	}

	return nil
}
func (fu *fileUsecase) GenerateStreamS3(headerParams *common.HeaderParams, file domain.File) error {

	folder := utils.UploadPath(headerParams)

	dir := filepath.Join(folder, file.Path)

	fileData := optimization.GetFileData(dir)

	fmt.Printf("File exists\n")

	//empty file variants
	_, err := fu.Repo.Update(headerParams.Client, file.Id, domain.File{}, bson.M{"$set": bson.M{"variants": []string{}}})
	if err != nil {
		fmt.Println(err)
	}

	//DASH
	go optimization.GenerateStreamS3(headerParams, fileData, dir)

	//HLS
	mainfestFile, err := streaming.GenerateMainfestHLSS3(headerParams, file)
	if err != nil {
		fmt.Println(err)
	}
	for mf := range mainfestFile {
		fmt.Println("mf")

		fmt.Println(mf)
	}

	//update file variants
	_, err = fu.Repo.Update(headerParams.Client, file.Id, domain.File{}, bson.M{"$push": bson.M{"variants": mainfestFile}})
	if err != nil {
		fmt.Println(err)
	}

	return nil
}

func (fu *fileUsecase) StoreS3(headerParams *common.HeaderParams, file domain.File, fileData []byte, bucketName string) (*domain.File, error) {
	return fu.Repo.StoreS3(headerParams.Client, file, fileData, bucketName)
}

// func (fu *fileUsecase) Store(headerParams *common.HeaderParams, file domain.File, query string) (*domain.File, error) {
// 	return fu.Repo.Store(headerParams.Client, file)
// }
