package uploader

import (
	"crypto/rand"
	"encoding/hex"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nehmeroumani/izero"
	"github.com/nehmeroumani/uploader/v2/gcs"
)

var (
	imageSizes                                     map[string][]*izero.ImageSize
	baseLocalUploadDirPath, baseCloudUploadDirPath string
	baseLocalUploadUrlPath, baseCloudUploadUrlPath string
	uploadToCloud, debugMode                       bool
)

func Init(BaseUploadDirPath string, BaseUploadUrlPath string, UploadToCloud bool, ImageSizes map[string][]*izero.ImageSize, DebugMode bool) {
	imageSizes = ImageSizes
	debugMode = DebugMode
	if !UploadToCloud {
		baseLocalUploadDirPath = filepath.FromSlash(BaseUploadDirPath)
		baseLocalUploadUrlPath = BaseUploadUrlPath
	} else {
		baseCloudUploadUrlPath = BaseUploadUrlPath
		baseCloudUploadDirPath = filepath.FromSlash(BaseUploadDirPath)
	}
	uploadToCloud = UploadToCloud
}

type MultipleUpload struct {
	FormData           *multipart.Form
	FileInputName     string
	ValidFileTypes     []*FileType
	ImageSizes         []string
	ImageCategory      string
	localUploadDirPath string
	cloudUploadDirPath string
	localUploadUrlPath string
	cloudUploadUrlPath string
}

func (mu *MultipleUpload) Upload() ([]*UploadedFile, []*UploadErr) {
	uploadedFiles := []*UploadedFile{}
	files := mu.FormData.File[mu.FileInputName]
	var (
		errs []*UploadErr
		wg   sync.WaitGroup
		m    sync.Mutex
	)
	wg.Add(len(files))
	for _, file := range files {
		go func(file *multipart.FileHeader) {
			defer func() {
				wg.Done()
				m.Unlock()
			}()
			if uploadedFile, upErr := mu.UploadOneFile(file); upErr == nil {
				m.Lock()
				uploadedFiles = append(uploadedFiles, uploadedFile)
			} else {
				m.Lock()
				errs = append(errs, upErr)
				if debugMode {
					upErr.Print()
				}
			}
		}(file)
	}
	wg.Wait()
	return uploadedFiles, errs
}

func (mu *MultipleUpload) UploadOneFile(fh *multipart.FileHeader) (*UploadedFile, *UploadErr) {
	uploadedFile := &UploadedFile{
		OriginalName: fh.Filename,
		Extension:    strings.ToLower(filepath.Ext(fh.Filename)),
	}

	file, err := fh.Open()
	if err != nil {
		return nil, NewUploadErr(uploadedFile, err, nil)
	}
	defer file.Close()

	uploadedFile.SetNumber(SizeNumber, fh.Size)

	// detect file type
	fileData := make([]byte, 512)
	_, err = file.Read(fileData)
	if err != nil {
		return nil, NewUploadErr(uploadedFile, err, nil)
	}

	uploadedFile.ContentType = http.DetectContentType(fileData)

	uploadedFile.FileType, err = DetectFileType(uploadedFile.ContentType, uploadedFile.Extension)
	if err != nil {
		return nil, NewUploadErr(uploadedFile, err, nil)
	}

	if !IsValidFileType(uploadedFile.FileType, mu.ValidFileTypes) {
		return nil, NewUploadErr(uploadedFile, ErrInvalidFileType, nil)
	}

	// reset reading cursor
	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, NewUploadErr(uploadedFile, err, nil)
	}

	uploadedFile.Name = generateRandomFileName(uploadedFile.Extension)

	if uploadToCloud {
		err = UploadToCloud(gcs.GetClient(), file, mu.PathOfFile(uploadedFile.Name))
	} else {
		if err = createFolderPath(mu.localUploadDirPath); err == nil {
			out, err := os.Create(filepath.Join(mu.localUploadDirPath, uploadedFile.Name))
			if err != nil {
				return nil, NewUploadErr(uploadedFile, err, nil)
			}
			defer out.Close()

			_, err = io.Copy(out, file)
		}
	}
	if err != nil {
		return nil, NewUploadErr(uploadedFile, err, nil)
	}

	if uploadedFile.FileType == ImageFileType {
		// reset reading cursor
		_, err = file.Seek(0, 0)
		if err != nil {
			return nil, NewUploadErr(uploadedFile, err, nil)
		}
		// extract image dimensions
		var imgConfig image.Config
		imgConfig, _, err = image.DecodeConfig(file)
		if err != nil {
			return nil, NewUploadErr(uploadedFile, ErrInvalidImage, nil)
		}
		uploadedFile.SetNumber(WidthNumber, int64(imgConfig.Width))
		uploadedFile.SetNumber(HeightNumber, int64(imgConfig.Height))

		if len(mu.ImageSizes) > 0 {
			// reset reading cursor
			_, err = file.Seek(0, 0)
			if err != nil {
				return nil, NewUploadErr(uploadedFile, err, nil)
			}

			// resize image
			var (
				destPath      string
				errs          map[string]error
				resizedImages map[string]*izero.ResizedImage
			)
			if !uploadToCloud {
				destPath = mu.LocalUploadDirPath()
			}
			resizedImages, errs, err = izero.ResizeImage(file, uploadedFile.Name, uploadedFile.ContentType, mu.imgCategoryTargetSizes(), destPath)
			if err != nil {
				return nil, NewUploadErr(uploadedFile, err, errs)
			}
			if uploadToCloud {
				var wg sync.WaitGroup
				errs = map[string]error{}
				wg.Add(len(resizedImages))
				for sizeName, resizedImage := range resizedImages {
					go func(sizeName string, resizedImage *izero.ResizedImage) {
						defer wg.Done()
						rImg, err := resizedImage.ToReader()
						if err == nil {
							if err = UploadToCloud(gcs.GetClient(), rImg, mu.PathOfFile(uploadedFile.Name, sizeName)); err != nil {
								errs[sizeName] = err
							}
						} else {
							errs[sizeName] = err
						}
					}(sizeName, resizedImage)
				}
				wg.Wait()

				if len(errs) > 0 {
					err = ErrUploadFailed
					return nil, NewUploadErr(uploadedFile, err, errs)
				}
			}
		}
	}
	return uploadedFile, nil
}

func (mu *MultipleUpload) imgCategoryTargetSizes() []*izero.ImageSize {
	if categorySizes, ok := imageSizes[mu.ImageCategory]; ok {
		targetSizes := []*izero.ImageSize{}
		for _, imgSize := range categorySizes {
			for _, s := range mu.ImageSizes {
				if s == imgSize.Name {
					targetSizes = append(targetSizes, imgSize)
					break
				}
			}
		}
		return targetSizes
	}
	return nil
}

func (mu *MultipleUpload) SetLocalUploadDir(localDir string) {
	localDir = filepath.FromSlash(localDir)
	mu.localUploadDirPath = filepath.Join(baseLocalUploadDirPath, localDir)
}

func (mu *MultipleUpload) LocalUploadDirPath() string {
	return mu.localUploadDirPath
}

func (mu *MultipleUpload) SetCloudUploadDir(cloudDir string) {
	cloudDir = filepath.FromSlash(cloudDir)
	mu.cloudUploadDirPath = filepath.Join(baseCloudUploadDirPath, cloudDir)
}

func (mu *MultipleUpload) CloudUploadDirPath() string {
	return mu.cloudUploadDirPath
}

func (mu *MultipleUpload) SetUploadDir(dir string) {
	if uploadToCloud {
		mu.SetCloudUploadDir(dir)
		if baseCloudUploadUrlPath != "" {
			mu.cloudUploadUrlPath = baseCloudUploadUrlPath + "/" + strings.Replace(dir, `\`, "/", -1)
		} else {
			mu.cloudUploadUrlPath = strings.Replace(dir, `\`, "/", -1)
		}
	} else {
		mu.SetLocalUploadDir(dir)
		mu.localUploadUrlPath = baseLocalUploadUrlPath + "/" + strings.Replace(dir, `\`, "/", -1)
	}
}

func (mu *MultipleUpload) UploadDirPath() string {
	if uploadToCloud {
		return mu.cloudUploadDirPath
	} else {
		return mu.localUploadDirPath
	}
}

func (mu *MultipleUpload) UploadUrlPath() string {
	if uploadToCloud {
		return mu.cloudUploadUrlPath
	} else {
		return mu.localUploadUrlPath
	}
}
func (mu *MultipleUpload) UrlOfFile(fileName string, opts ...string) string {
	var sizeName string
	if opts != nil && len(opts) > 0 {
		sizeName = strings.ToLower(strings.TrimSpace(opts[0]))
	}
	if uploadToCloud {
		if sizeName != "" && sizeName != "original" {
			return mu.cloudUploadUrlPath + "/" + sizeName + "/" + fileName
		}
		return mu.cloudUploadUrlPath + "/" + fileName
	}
	if sizeName != "" && sizeName != "original" {
		return mu.localUploadUrlPath + "/" + sizeName + "/" + fileName
	}
	return mu.localUploadUrlPath + "/" + fileName
}

func (mu *MultipleUpload) PathOfFile(fileName string, opts ...string) string {
	var sizeName string
	if opts != nil && len(opts) > 0 {
		sizeName = strings.ToLower(strings.TrimSpace(opts[0]))
	}
	if uploadToCloud {
		if sizeName != "" && sizeName != "original" {
			return filepath.Join(mu.cloudUploadDirPath, sizeName, fileName)
		}
		return filepath.Join(mu.cloudUploadDirPath, fileName)
	}
	if sizeName != "" && sizeName != "original" {
		return filepath.Join(mu.localUploadDirPath, sizeName, fileName)
	}
	return filepath.Join(mu.localUploadDirPath, fileName)
}

func (mu *MultipleUpload) AttachmentFileURI(fileName string, opts ...string) string {
	if uploadToCloud {
		return mu.UrlOfFile(fileName, opts...)
	} else {
		return mu.PathOfFile(fileName, opts...)
	}
}

func generateRandomFileName(extension string) string {
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	return strconv.Itoa(int(time.Now().UTC().Unix())) + "-" + hex.EncodeToString(randBytes) + extension
}

func createFolderPath(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		if err = os.MkdirAll(path, 0777); err != nil {
			return err
		}
	}
	return nil
}
