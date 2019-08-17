package uploader

type UploadErr struct {
	FileName   string
	Err        error
	ResizeErrs map[string]error
}

func NewUploadErr(fileName string, err error, errs map[string]error) *UploadErr {
	return &UploadErr{FileName: fileName, Err: err, ResizeErrs: errs}
}
