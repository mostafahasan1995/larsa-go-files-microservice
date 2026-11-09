package mongo_test

import (
	"larsa-files/domain"
	mongoFileRepsitory "larsa-files/file/repository/mongo"
	"testing"

	"git.larsa.io/mahdawi/microservices-commons.git/cache"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestGetAll(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	//redisDB, _ := redismock.NewClientMock()
	redisCache := cache.NewRedisCache()

	mt.Run("Get All", func(mt *mtest.T) {
		find := mtest.CreateCursorResponse(
			1,
			"test.files",
			mtest.FirstBatch,
			bson.D{
				{"originalName", "file1"},
				{"trash", false},
			})
		getMore := mtest.CreateCursorResponse(
			1,
			"test.files",
			mtest.NextBatch,
			bson.D{
				{"originalName", "file2"},
				{"trash", false},
			})
		killCursors := mtest.CreateCursorResponse(
			0,
			"test.files",
			mtest.NextBatch)
		mt.AddMockResponses(find, getMore, killCursors)

		repo := mongoFileRepsitory.NewMongoFileRepository(mt.Client, redisCache)
		res, err := repo.GetAll("test", "/", bson.M{})
		assert.Nil(mt, err)
		assert.Equal(mt, []domain.File{
			{OriginalName: "file1", Trash: false},
			{OriginalName: "file2", Trash: false},
		}, res)
	})

}
