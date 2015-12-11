package rancher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/docker/libcompose/project"
)

type S3Uploader struct {
	Url string
	AccessKey string
	SecretKey string
	region string
}

func (s *S3Uploader) Name() string {
	return "S3"
}

func (s *S3Uploader) Upload(p *project.Project, name string, reader io.ReadSeeker, hash string) (string, string, error) {
	bucketName := fmt.Sprintf("%s-%s", p.Name, someHash())
	objectKey := fmt.Sprintf("%s-%s", name, hash[:12])
	
	if s.region == "" {
	   s.region = "us-east-1"
	}

	// Needed for Minio.io Connection
	s3Session := session.New()
	s3Config := &aws.Config{
        	Credentials: 	  credentials.NewStaticCredentials(s.AccessKey, s.SecretKey, ""),
	        Endpoint:         aws.String(s.Url),
	        Region:           aws.String(s.region),
	        DisableSSL:       aws.Bool(true),
	        S3ForcePathStyle: aws.Bool(true),
	}

	svc := s3.New(s3Session, s3Config)

	if err := getOrCreateBucket(svc, bucketName); err != nil {
		return "", "", err
	}

	if err := putFile(svc, bucketName, objectKey, reader); err != nil {
		return "", "", err
	}

	req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    &objectKey,
	})

	url, err := req.Presign(24 * 7 * time.Hour)
	return objectKey, url, err
}

func putFile(svc *s3.S3, bucket, object string, reader io.ReadSeeker) error {
	_, err := svc.PutObject(&s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &object,
		Body:   reader,
	})

	return err
}

func getOrCreateBucket(svc *s3.S3, bucketName string) error {
	_, err := svc.HeadBucket(&s3.HeadBucketInput{
		Bucket: &bucketName,
	})

	if reqErr, ok := err.(awserr.RequestFailure); ok && reqErr.StatusCode() == 404 {
		logrus.Infof("Creating bucket %s", bucketName)
		_, err = svc.CreateBucket(&s3.CreateBucketInput{
			Bucket: &bucketName,
		})
	}

	return err
}

func someHash() string {
	/* Should come up with some better way to do this */
	sha := sha256.New()

	wd, err := os.Getwd()
	if err == nil {
		sha.Write([]byte(wd))
	}

	for _, env := range os.Environ() {
		sha.Write([]byte(env))
	}

	return hex.EncodeToString(sha.Sum([]byte{}))[:12]
}
