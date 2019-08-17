package uploader

import "errors"

var (
	InvalidForm     = errors.New("invalid_form")
	UploadFailed    = errors.New("upload_failed")
	InvalidFileType = errors.New("invalid_file_type")
)
