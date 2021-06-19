package uploader

import "errors"

var (
	ErrInvalidForm         = errors.New("invalid_form")
	ErrUploadFailed        = errors.New("upload_failed")
	ErrInvalidFileType     = errors.New("invalid_file_type")
	ErrInvalidImage        = errors.New("invalid_image")
	ErrUnsulportedFileType = errors.New("unsupported_file_type")
)
