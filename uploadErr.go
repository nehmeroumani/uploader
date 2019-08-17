package uploader

import (
	"fmt"

	"github.com/fatih/color"
)

type UploadErr struct {
	FileName   string
	Err        error
	ResizeErrs map[string]error
}

func (upErr *UploadErr) Print() {
	fmt.Println(color.RedString("File name : "), upErr.FileName)
	fmt.Println(color.RedString("Error : "), upErr.Err)
	if upErr.ResizeErrs != nil {
		for sizeName, err := range upErr.ResizeErrs {
			fmt.Println(color.RedString(sizeName+" : "), err)
		}
	}
}

func NewUploadErr(fileName string, err error, errs map[string]error) *UploadErr {
	return &UploadErr{FileName: fileName, Err: err, ResizeErrs: errs}
}
