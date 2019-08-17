package uploader

import "fmt"

type UploadErr struct {
	FileName   string
	Err        error
	ResizeErrs map[string]error
}

func (upErr *UploadErr) Print() {
	fmt.Println(upErr.FileName)
	fmt.Println("Error : ", upErr.Err)
	if upErr.ResizeErrs != nil {
		for sizeName, err := range upErr.ResizeErrs {
			fmt.Println(sizeName+" : ", err)
		}
	}
}

func NewUploadErr(fileName string, err error, errs map[string]error) *UploadErr {
	return &UploadErr{FileName: fileName, Err: err, ResizeErrs: errs}
}
