package uploader

import (
	"io"
	"sync"

	"github.com/nehmeroumani/pill.go/clean"
)

type CloudUploader interface {
	Upload(io.Reader, string) error
}

func UploadToCloud(cloudUploader CloudUploader, file io.Reader, path string) error {
	if cloudUploader != nil {
		return cloudUploader.Upload(file, path)
	}
	return nil
}

func GoUploadToCloud(cloudUploader CloudUploader, file io.Reader, path string, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		if cloudUploader != nil {
			if err := cloudUploader.Upload(file, path); err != nil {
				clean.Error(err)
			}
		}
	}()
}
