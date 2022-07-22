package gcs

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GCSClient struct {
	ProjectID        string
	ProjectNumber    string
	client           *storage.Client
	PublicBucketName string
	bucketsList      []string
}

func NewGCSClient(projectID string, projectNumber string, jsonKeyPath string) (*GCSClient, error) {
	if jsonKeyPath == "" || projectID == "" {
		return nil, fmt.Errorf("Google Cloud Storage requires the json key with the project id.")
	}
	client := &GCSClient{ProjectID: projectID, ProjectNumber: projectNumber}
	client.PublicBucketName = projectID + "-public"
	var err error
	ctx := context.Background()
	client.client, err = storage.NewClient(ctx, option.WithServiceAccountFile(jsonKeyPath))
	if err != nil {
		return nil, err
	}
	client.bucketsList, err = client.BucketsList()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (gcsc *GCSClient) CreateBucket(bucketName string) error {
	if !gcsc.DoesBucketExist(bucketName) {
		ctx := context.Background()
		// [START create_bucket]
		if err := gcsc.client.Bucket(bucketName).Create(ctx, gcsc.ProjectID, nil); err != nil {
			return err
		}
		if gcsc.bucketsList == nil {
			gcsc.bucketsList = []string{bucketName}
		} else {
			gcsc.bucketsList = append(gcsc.bucketsList, bucketName)
		}
		// [END create_bucket]
	}
	return nil
}

func (gcsc *GCSClient) DoesBucketExist(bucketName string) bool {
	if gcsc.bucketsList != nil {
		for _, bName := range gcsc.bucketsList {
			if bName == bucketName {
				return true
			}
		}
	}
	return false
}

func (gcsc *GCSClient) BucketsList() ([]string, error) {
	ctx := context.Background()
	// [START list_buckets]
	var buckets []string
	it := gcsc.client.Buckets(ctx, gcsc.ProjectID)
	for {
		battrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, battrs.Name)
	}
	// [END list_buckets]
	return buckets, nil
}

func (gcsc *GCSClient) DeleteBucket(bucketName string) error {
	ctx := context.Background()
	// [START delete_bucket]
	if err := gcsc.client.Bucket(bucketName).Delete(ctx); err != nil {
		return err
	}
	// [END delete_bucket]
	return nil
}

func (gcsc *GCSClient) WriteObject(file io.Reader, path string) error {
	if file != nil && path != "" {
		if bErr := gcsc.CreateBucket(gcsc.PublicBucketName); bErr == nil {
			ctx := context.Background()
			path = strings.TrimPrefix(path, "/")
			wc := gcsc.client.Bucket(gcsc.PublicBucketName).Object(path).NewWriter(ctx)
			wc.ACL = []storage.ACLRule{{Entity: storage.AllUsers, Role: storage.RoleReader}, {Entity: storage.ACLEntity("project-owners-" + gcsc.ProjectNumber), Role: storage.RoleOwner}, {Entity: storage.ACLEntity("project-editors-" + gcsc.ProjectNumber), Role: storage.RoleOwner}}
			wc.CacheControl = "public, max-age=15552000"
			if strings.ToLower(filepath.Ext(path)) == ".svg" {
				wc.ContentType = "image/svg+xml"
			}
			if _, err := io.Copy(wc, file); err != nil {
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

func (gcsc *GCSClient) Upload(file io.Reader, path string) error {
	return gcsc.WriteObject(file, path)
}
