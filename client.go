package awos

import (
	"fmt"
	"io"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// Client interface
type Client interface {
	Get(key string) (string, error)
	GetAsReader(key string) (io.ReadCloser, error)
	Put(key string, reader io.ReadSeeker, meta map[string]string, options ...PutOptions) error
	Del(key string) error
	DelMulti(keys []string) error
	Head(key string, meta []string) (map[string]string, error)
	ListObject(key string, prefix string, marker string, maxKeys int, delimiter string) ([]string, error)
	SignURL(key string, expired int64) (string, error)
	GetAndDecompress(key string) (string, error)
	GetAndDecompressAsReader(key string) (io.ReadCloser, error)
	CompressAndPut(key string, reader io.ReadSeeker, meta map[string]string, options ...PutOptions) error
}

// Options for New method
type Options struct {
	// Required, value is one of oss/s3, case insensetive
	StorageType string
	// Required
	AccessKeyID string
	// Required
	AccessKeySecret string
	// Required
	Endpoint string
	// Required
	Bucket string
	// Optional, choose which bucket to use based on the last character of the key,
	// if bucket is 'content', shards is ['abc', 'edf'],
	// then the last character of the key with a/b/c will automatically use the content-abc bucket, and vice versa
	Shards []string
	// Only for s3-like
	Region string
	// Only for s3-like, whether to force path style URLs for S3 objects.
	S3ForcePathStyle bool
	// Only for s3-like
	SSL bool
}

// New awos Client instance
func New(options *Options) (Client, error) {
	var miossClient Client
	storageType := strings.ToLower(options.StorageType)

	if storageType == "oss" {
		client, err := oss.New(options.Endpoint, options.AccessKeyID, options.AccessKeySecret)
		if err != nil {
			return nil, err
		}

		var ossClient *OSS
		if options.Shards != nil && len(options.Shards) > 0 {
			buckets := make(map[string]*oss.Bucket)
			for _, v := range options.Shards {
				bucket, err := client.Bucket(options.Bucket + "-" + v)
				if err != nil {
					return nil, err
				}
				for i := 0; i < len(v); i++ {
					buckets[strings.ToLower(v[i:i+1])] = bucket
				}
			}

			ossClient = &OSS{
				Shards: buckets,
			}
		} else {
			bucket, err := client.Bucket(options.Bucket)
			if err != nil {
				return nil, err
			}

			ossClient = &OSS{
				Bucket: bucket,
			}
		}

		miossClient = ossClient

		return miossClient, nil
	} else if storageType == "s3" {
		var sess *session.Session

		// use minio
		if options.S3ForcePathStyle == true {
			sess = session.Must(session.NewSession(&aws.Config{
				Region:           aws.String(options.Region),
				DisableSSL:       aws.Bool(options.SSL == false),
				Credentials:      credentials.NewStaticCredentials(options.AccessKeyID, options.AccessKeySecret, ""),
				Endpoint:         aws.String(options.Endpoint),
				S3ForcePathStyle: aws.Bool(true),
			}))
		} else {
			sess = session.Must(session.NewSession(&aws.Config{
				Region:      aws.String(options.Region),
				DisableSSL:  aws.Bool(options.SSL == false),
				Credentials: credentials.NewStaticCredentials(options.AccessKeyID, options.AccessKeySecret, ""),
			}))
		}
		service := s3.New(sess)

		var awsClient *AWS
		if options.Shards != nil && len(options.Shards) > 0 {
			buckets := make(map[string]string)
			for _, v := range options.Shards {
				for i := 0; i < len(v); i++ {
					buckets[strings.ToLower(v[i:i+1])] = options.Bucket + "-" + v
				}
			}
			awsClient = &AWS{
				ShardsBucket: buckets,
				Client:       service,
			}
		} else {
			awsClient = &AWS{
				BucketName: options.Bucket,
				Client:     service,
			}
		}

		miossClient = awsClient

		return miossClient, nil
	} else {
		return nil, fmt.Errorf("Unknown StorageType:\"%s\", only supports oss,s3", options.StorageType)
	}
}
