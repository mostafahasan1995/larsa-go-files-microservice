package configs

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var DatabaseConnection string

func SetUpDataBase() (*mongo.Client, error) {
	if os.Getenv("DB_PASSWORD") != "" {
		DatabaseConnection = fmt.Sprintf(`mongodb://%v:%v@%v:%v`, os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"))
	} else {
		DatabaseConnection = fmt.Sprintf(`mongodb://%v:%v`, os.Getenv("DB_HOST"), os.Getenv("DB_PORT"))
	}

	if os.Getenv("DB_FULL_HOST") != "" {
		DatabaseConnection = fmt.Sprintf(`mongodb://%v`, os.Getenv("DB_FULL_HOST"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(DatabaseConnection))

	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, err
	}

	return client, nil
}
