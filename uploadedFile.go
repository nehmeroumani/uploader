package uploader

import "errors"

const (
	SizeNumber   = 0
	WidthNumber  = 1
	HeightNumber = 2
)

type UploadedFile struct {
	Name         string
	OriginalName string
	Type         string
	Numbers      []int64
}

func (uf *UploadedFile) GetNumber(index int) (int64, error) {
	if len(uf.Numbers)-1 >= index {
		return uf.Numbers[index], nil
	}
	return 0, errors.New("Index out of range")
}

func (uf *UploadedFile) SetNumber(index int, value int64) {
	if len(uf.Numbers)-1 < index {
		numbers := make([]int64, index+1)
		for i, n := range uf.Numbers {
			numbers[i] = n
		}
		uf.Numbers = numbers
	}
	uf.Numbers[index] = value
}
