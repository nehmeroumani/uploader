package uploader

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
)

type UploadErr struct {
	File       *UploadedFile
	Err        error
	ResizeErrs map[string]error
}

func (ue *UploadErr) Print() {
	jsonData, err := json.MarshalIndent(ue, "", "\t")
	fmt.Println(color.RedString("Error : \n"))
	if err == nil {
		fmt.Println(string(jsonData))
	} else {
		fmt.Println(err)
	}
}

func NewUploadErr(file *UploadedFile, err error, errs map[string]error) *UploadErr {
	return &UploadErr{File: file, Err: err, ResizeErrs: errs}
}
