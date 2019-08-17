package uploader

import (
	"crypto/rand"
	"encoding/hex"
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
	"github.com/nehmeroumani/uploader/gcs"
)

var (
	imageExtensions   = []string{".jpeg", ".jpg", ".gif", ".png"}
	imageContentTypes = []string{"image/jpeg", "image/jpg", "image/gif", "image/png"}
	imageSizes        map[string][]*izero.ImageSize

	pdfContentTypes = []string{"application/pdf", "application/x-pdf", "application/acrobat", "applications/vnd.pdf", "text/pdf", "text/x-pdf"}

	documentExtensions   = []string{".doc", ".dot", ".docx", ".dotx", ".docm", ".dotm"}
	documentContentTypes = []string{"application/zip", "application/msword", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.openxmlformats-officedocument.wordprocessingml.template", "application/vnd.ms-word.document.macroEnabled.12", "application/vnd.ms-word.template.macroEnabled.12"}

	svgExtensions   = []string{".svg", ".svgz"}
	svgContentTypes = []string{"image/svg+xml", "text/xml", "text/xml; charset=utf-8", "text/plain; charset=utf-8"}

	baseLocalUploadDirPath, baseCloudUploadDirPath, baseLocalUploadUrlPath, baseCloudUploadUrlPath string
	uploadToCloud                                                                                  bool
)

func Init(BaseUploadDirPath string, BaseUploadUrlPath string, UploadToCloud bool, ImageSizes map[string][]*izero.ImageSize) {
	imageSizes = imageSizes
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
	FilesInputName     string
	FileType           string
	ImageSizes         []string
	ImageCategory      string
	localUploadDirPath string
	cloudUploadDirPath string
	localUploadUrlPath string
	cloudUploadUrlPath string
}

func (this *MultipleUpload) Upload() ([]string, []*UploadErr) {
	uploadedFiles := []string{}
	files := this.FormData.File[this.FilesInputName]
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
			if uploadedFile, upErr := this.UploadOneFile(file); upErr == nil {
				m.Lock()
				uploadedFiles = append(uploadedFiles, uploadedFile)
			} else {
				m.Lock()
				errs = append(errs, upErr)
			}
		}(file)
	}
	wg.Wait()
	return uploadedFiles, errs
}

func (this *MultipleUpload) UploadOneFile(fh *multipart.FileHeader) (string, *UploadErr) {
	file, err := fh.Open()

	if err != nil {
		return "", NewUploadErr(fh.Filename, err, nil)
	}
	defer file.Close()

	fileExtension := strings.ToLower(filepath.Ext(fh.Filename))

	fileData := make([]byte, 512)
	_, err = file.Read(fileData)
	if err != nil {
		return "", NewUploadErr(fh.Filename, err, nil)
	}

	isValidFileType, fileType, fileTypeName := isValidFileType(this.FileType, fileData, fileExtension)

	if !isValidFileType {
		return "", NewUploadErr(fh.Filename, InvalidFileType, nil)
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return "", NewUploadErr(fh.Filename, err, nil)
	}
	randomFileName := generateRandomFileName(fileExtension)
	if uploadToCloud {
		err = UploadToCloud(gcs.GetClient(), file, this.PathOfFile(randomFileName))
	} else {
		if err = createFolderPath(this.localUploadDirPath); err == nil {
			out, err := os.Create(filepath.Join(this.localUploadDirPath, randomFileName))
			if err != nil {
				return "", NewUploadErr(fh.Filename, err, nil)
			}
			defer out.Close()

			_, err = io.Copy(out, file)
		}
	}
	if err != nil {
		return "", NewUploadErr(fh.Filename, err, nil)
	}
	if fileTypeName == "image" && this.ImageSizes != nil {
		_, err = file.Seek(0, 0)
		if err != nil {
			return "", NewUploadErr(fh.Filename, err, nil)
		}
		var (
			destPath      string
			errs          map[string]error
			resizedImages map[string]*izero.ResizedImage
		)
		if !uploadToCloud {
			destPath = this.LocalUploadDirPath()
		}
		resizedImages, errs, err = izero.ResizeImage(file, randomFileName, fileType, this.imgCategoryTargetSizes(), destPath)
		if err != nil {
			return "", NewUploadErr(fh.Filename, err, errs)
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
						if err = UploadToCloud(gcs.GetClient(), rImg, this.PathOfFile(randomFileName, sizeName)); err != nil {
							errs[sizeName] = err
						}
					} else {
						errs[sizeName] = err
					}
				}(sizeName, resizedImage)
			}
			wg.Wait()
			if len(errs) > 0 {
				err = UploadFailed
				return "", NewUploadErr(fh.Filename, err, errs)
			}
		}
	}
	return randomFileName, nil
}

func (this *MultipleUpload) imgCategoryTargetSizes() []*izero.ImageSize {
	if categorySizes, ok := imageSizes[this.ImageCategory]; ok {
		targetSizes := []*izero.ImageSize{}
		for _, imgSize := range categorySizes {
			for _, s := range this.ImageSizes {
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

func (this *MultipleUpload) SetLocalUploadDir(localDir string) {
	localDir = filepath.FromSlash(localDir)
	this.localUploadDirPath = filepath.Join(baseLocalUploadDirPath, localDir)
}

func (this *MultipleUpload) LocalUploadDirPath() string {
	return this.localUploadDirPath
}

func (this *MultipleUpload) SetCloudUploadDir(cloudDir string) {
	cloudDir = filepath.FromSlash(cloudDir)
	this.cloudUploadDirPath = filepath.Join(baseCloudUploadDirPath, cloudDir)
}

func (this *MultipleUpload) CloudUploadDirPath() string {
	return this.cloudUploadDirPath
}

func (this *MultipleUpload) SetUploadDir(dir string) {
	if uploadToCloud {
		this.SetCloudUploadDir(dir)
		if baseCloudUploadUrlPath != "" {
			this.cloudUploadUrlPath = baseCloudUploadUrlPath + "/" + strings.Replace(dir, `\`, "/", -1)
		} else {
			this.cloudUploadUrlPath = strings.Replace(dir, `\`, "/", -1)
		}
	} else {
		this.SetLocalUploadDir(dir)
		this.localUploadUrlPath = baseLocalUploadUrlPath + "/" + strings.Replace(dir, `\`, "/", -1)
	}
}

func (this *MultipleUpload) UploadDirPath() string {
	if uploadToCloud {
		return this.cloudUploadDirPath
	} else {
		return this.localUploadDirPath
	}
}

func (this *MultipleUpload) UploadUrlPath() string {
	if uploadToCloud {
		return this.cloudUploadUrlPath
	} else {
		return this.localUploadUrlPath
	}
}
func (this *MultipleUpload) UrlOfFile(fileName string, opts ...string) string {
	var sizeName string
	if opts != nil && len(opts) > 0 {
		sizeName = strings.ToLower(strings.TrimSpace(opts[0]))
	}
	if uploadToCloud {
		if sizeName != "" && sizeName != "original" {
			return this.cloudUploadUrlPath + "/" + sizeName + "/" + fileName
		}
		return this.cloudUploadUrlPath + "/" + fileName
	}
	if sizeName != "" && sizeName != "original" {
		return this.localUploadUrlPath + "/" + sizeName + "/" + fileName
	}
	return this.localUploadUrlPath + "/" + fileName
}

func (this *MultipleUpload) PathOfFile(fileName string, opts ...string) string {
	var sizeName string
	if opts != nil && len(opts) > 0 {
		sizeName = strings.ToLower(strings.TrimSpace(opts[0]))
	}
	if uploadToCloud {
		if sizeName != "" && sizeName != "original" {
			return filepath.Join(this.cloudUploadDirPath, sizeName, fileName)
		}
		return filepath.Join(this.cloudUploadDirPath, fileName)
	}
	if sizeName != "" && sizeName != "original" {
		return filepath.Join(this.localUploadDirPath, sizeName, fileName)
	}
	return filepath.Join(this.localUploadDirPath, fileName)
}

func (this *MultipleUpload) AttachmentFileURI(fileName string, opts ...string) string {
	if uploadToCloud {
		return this.UrlOfFile(fileName, opts...)
	} else {
		return this.PathOfFile(fileName, opts...)
	}
}

func generateRandomFileName(extension string) string {
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	return strconv.Itoa(int(time.Now().UTC().Unix())) + "-" + hex.EncodeToString(randBytes) + extension
}

func detectContentType(fileData []byte) string {
	if fileData != nil {
		filetype := http.DetectContentType(fileData)
		return filetype
	}
	return ""
}

func isValidFileType(requiredFileTypesRaw string, fileData []byte, fileExtension string) (bool, string, string) {
	isValidExtension := false
	isValidContentType := false
	fileType := detectContentType(fileData)
	fileTypeName := ""
	requiredFileTypesRaw = strings.ToLower(strings.Replace(requiredFileTypesRaw, " ", "", -1))
	requiredFileTypes := strings.Split(requiredFileTypesRaw, "|")
	for _, requiredFileType := range requiredFileTypes {
		switch requiredFileType {
		case "image":
			fileTypeName = "image"
			for _, imageExtension := range imageExtensions {
				if imageExtension == fileExtension {
					isValidExtension = true
					break
				}
			}
			if isValidExtension {
				for _, imageContentType := range imageContentTypes {
					if fileType == imageContentType {
						isValidContentType = true
						break
					}
				}
			}
		case "document":
			fileTypeName = "document"
			for _, documentExtension := range documentExtensions {
				if documentExtension == fileExtension {
					isValidExtension = true
					break
				}
			}
			if isValidExtension {
				for _, documentContentType := range documentContentTypes {
					if fileType == documentContentType {
						isValidContentType = true
						break
					}
				}
			}
		case "svg":
			fileTypeName = "svg"
			for _, svgExtension := range svgExtensions {
				if svgExtension == fileExtension {
					isValidExtension = true
					break
				}
			}
			if isValidExtension {
				for _, svgContentType := range svgContentTypes {
					if fileType == svgContentType {
						isValidContentType = true
						break
					}
				}
			}
		case "pdf":
			fileTypeName = "pdf"
			if fileExtension == ".pdf" {
				isValidExtension = true
			}
			if isValidExtension {
				for _, pdfContentType := range pdfContentTypes {
					if fileType == pdfContentType {
						isValidContentType = true
						break
					}
				}
			}
		}

		if isValidExtension {
			break
		}
	}
	return isValidContentType && isValidExtension, fileType, fileTypeName
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
