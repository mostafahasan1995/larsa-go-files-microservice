package uploadS3

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"larsa-files/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const region = "us-east-1"

var (
	bucketName = os.Getenv("S3_BUCKET")
	accessKey  = os.Getenv("S3_ACCESS_KEY")
	secretKey  = os.Getenv("S3_SECRET_KEY")
	endpoint   = os.Getenv("S3_ENDPOINT")
)

func StoreFileS3(fileObject []byte, objectKey string) error {
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(region),                                         // Use any region (not strictly necessary for Larsa)
		Endpoint:         aws.String(endpoint),                                       // Larsa Object Storage URL
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""), // Larsa Access Key & Secret Key
		S3ForcePathStyle: aws.Bool(true),                                             // Force path-style URLs (typical for S3-compatible services)
	})
	if err != nil {
		return err
	}
	s3Client := s3.New(sess)

	contentType := utils.DetectContentTypeFromName(objectKey)

	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
		Body:        bytes.NewReader(fileObject),
		ACL:         aws.String("public-read"),
	})
	if err != nil {
		return err
	}

	return nil
}

func CalculateSize(filsKey []string) int {
	fmt.Println("total size : hiiiiiiiiiiiiiii")
	total := 0
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(region),                                         // Use any region (not strictly necessary for Larsa)
		Endpoint:         aws.String(endpoint),                                       // Larsa Object Storage URL
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""), // Larsa Access Key & Secret Key
		S3ForcePathStyle: aws.Bool(true),                                             // Force path-style URLs (typical for S3-compatible services)
	})
	if err != nil {
		fmt.Println("failed to create AWS session:", err)
		return 0
	}

	svc := s3.New(sess)

	// Loop through `filsKey` to get file sizes from S3
	for _, fileKey := range filsKey {
		headObj, err := svc.HeadObject(&s3.HeadObjectInput{
			Bucket: aws.String(bucketName), // Replace with your S3 bucket name
			Key:    aws.String(fileKey),
		})
		if err != nil {
			fmt.Println("failed to get file info from S3:", err)
			continue
		}
		total += int(*headObj.ContentLength)

	}
	fmt.Println("total size : ", total)

	return total
}

// GetFileFromS3 downloads a file from S3
func GetFileFromS3(objectKey string) ([]byte, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(region),
		Endpoint:         aws.String(endpoint),
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return nil, err
	}

	s3Client := s3.New(sess)

	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()

	// Read the entire file
	fileData, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, err
	}

	return fileData, nil
}
