package gcs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/nehmeroumani/pill.go/clean"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type Client struct {
	// Project ID
	ProjectID     string
	ProjectNumber string
	// GCS Client
	gcsClient        *storage.Client
	PublicBucketName string
	bucketsList      []string
}

var client *Client
var jsonKeyPath, projectID, projectNumber string

func Init(ProjectID, ProjectNumber, JSONKeyPath string) {
	projectID = ProjectID
	jsonKeyPath = JSONKeyPath
	projectNumber = ProjectNumber
	client = GetClient()
}

func GetClient() *Client {
	if jsonKeyPath == "" || projectID == "" {
		fmt.Println("Google Cloud Storage requires the json key with the project id.")
		os.Exit(0)
	}
	if client == nil {
		client = &Client{ProjectID: projectID, ProjectNumber: projectNumber}
		client.PublicBucketName = projectID + "-public"
		var err error
		ctx := context.Background()
		client.gcsClient, err = storage.NewClient(ctx, option.WithServiceAccountFile(jsonKeyPath))
		if err != nil {
			clean.Error(err)
			os.Exit(0)
		}
		client.bucketsList, err = client.BucketsList()
		if err != nil {
			clean.Error(err)
			os.Exit(0)
		}
	}
	return client
}

func (this *Client) CreateBucket(bucketName string) error {
	if !this.IsBucketExist(bucketName) {
		ctx := context.Background()
		// [START create_bucket]
		if err := this.gcsClient.Bucket(bucketName).Create(ctx, this.ProjectID, nil); err != nil {
			clean.Error(err)
			return err
		}
		if this.bucketsList == nil {
			this.bucketsList = []string{bucketName}
		} else {
			this.bucketsList = append(this.bucketsList, bucketName)
		}
		// [END create_bucket]
	}
	return nil
}

func (this *Client) IsBucketExist(bucketName string) bool {
	if this.bucketsList != nil {
		for _, bName := range this.bucketsList {
			if bName == bucketName {
				return true
			}
		}
	}
	return false
}

func (this *Client) BucketsList() ([]string, error) {
	ctx := context.Background()
	// [START list_buckets]
	var buckets []string
	it := this.gcsClient.Buckets(ctx, this.ProjectID)
	for {
		battrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			clean.Error(err)
			return nil, err
		}
		buckets = append(buckets, battrs.Name)
	}
	// [END list_buckets]
	return buckets, nil
}

func (this *Client) DeleteBucket(bucketName string) error {
	ctx := context.Background()
	// [START delete_bucket]
	if err := this.gcsClient.Bucket(bucketName).Delete(ctx); err != nil {
		clean.Error(err)
		return err
	}
	// [END delete_bucket]
	return nil
}

func (this *Client) WriteObject(file io.Reader, path string) error {
	if file != nil && path != "" {
		if bErr := this.CreateBucket(this.PublicBucketName); bErr == nil {
			ctx := context.Background()
			path = strings.TrimPrefix(path, "/")
			wc := this.gcsClient.Bucket(this.PublicBucketName).Object(path).NewWriter(ctx)
			wc.ACL = []storage.ACLRule{{Entity: storage.AllUsers, Role: storage.RoleReader}, {Entity: storage.ACLEntity("project-owners-" + this.ProjectNumber), Role: storage.RoleOwner}, {Entity: storage.ACLEntity("project-editors-" + this.ProjectNumber), Role: storage.RoleOwner}}
			wc.CacheControl = "public, max-age=15552000"
			if _, err := io.Copy(wc, file); err != nil {
				clean.Error(err)
				return err
			}
			if err := wc.Close(); err != nil {
				return err
			}
			// [END upload_file]
			return nil
		} else {
			return bErr
		}
	}
	return errors.New("invalid_data")
}

func (this *Client) Upload(file io.Reader, path string) error {
	return this.WriteObject(file, path)
}
