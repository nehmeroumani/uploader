package uploader

type FileType struct {
	Name         string
	ContentTypes []string
	Extensions   []string
}

func (ft *FileType) IsValidExtension(ext string) bool{
	for _, e := range ft.Extensions{
		if e == ext{
			return true
		} 
	}
	return false
}

func (ft *FileType) IsValidContentType(contentType string) bool{
	for _, ct := range ft.ContentTypes{
		if ct == contentType{
			return true
		} 
	}
	return false
}

func (ft *FileType) IsValid(contentType string, ext string) bool{
	return ft.IsValidContentType(contentType) && ft.IsValidExtension(ext)
}

var (
	ImageFileType = &FileType{
		Name:         "image",
		ContentTypes: []string{"image/jpeg", "image/jpg", "image/gif", "image/png"},
		Extensions:   []string{".jpeg", ".jpg", ".gif", ".png"},
	}
	AudioFileType = &FileType{
		Name:         "audio",
		ContentTypes: []string{"audio/flac", "audio/webm", "audio/mpegurl", "text/plain", "audio/mp4", "audio/mpeg", "audio/ogg", "audio/x-scpls", "audio/wav"},
		Extensions:   []string{".flac", ".m3u", ".m3u8", ".m4a", ".m4b", ".mp3", ".ogg", ".opus", ".pls", ".wav"},
	}
	VideoFileType = &FileType{
		Name:         "video",
		ContentTypes: []string{"video/mp4", "application/mp4", "application/x-mpegurl", "video/mp2t", "video/3gpp", "video/quicktime", "video/x-msvideo", "video/x-ms-wmv"},
		Extensions:   []string{".mp4", ".m3u8", ".ts", ".3gp", ".mov", ".avi", ".wmv", ".ogv", ".m4a", ".m4p", ".m4b", ".m4r", ".m4v"},
	}
	SvgFileType = &FileType{
		Name:         "svg",
		ContentTypes: []string{"image/svg+xml", "text/xml", "text/xml; charset=utf-8", "text/plain; charset=utf-8"},
		Extensions:   []string{".svg", ".svgz"},
	}
	DocumentFileType = &FileType{
		Name:         "document",
		ContentTypes: []string{"application/zip", "application/msword", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.openxmlformats-officedocument.wordprocessingml.template", "application/vnd.ms-word.document.macroEnabled.12", "application/vnd.ms-word.template.macroEnabled.12"},
		Extensions:   []string{".doc", ".dot", ".docx", ".dotx", ".docm", ".dotm"},
	}
	PdfFileType = &FileType{
		Name:         "pdf",
		ContentTypes: []string{"application/pdf", "application/x-pdf", "application/acrobat", "applications/vnd.pdf", "text/pdf", "text/x-pdf"},
		Extensions:   []string{".pdf"},
	}

	AllFileTypes = []*FileType{
		ImageFileType,
		AudioFileType,
		VideoFileType,
		SvgFileType,
		DocumentFileType,
		PdfFileType,
	}
)

func DetectFileType(contentType string, extension string) (*FileType, error) {
	for _, fileType := range AllFileTypes{
		if fileType.IsValid(contentType, extension){
			return fileType, nil
		}
	}
	return nil, ErrUnsulportedFileType
}

func IsValidFileType(fileType *FileType, validFileTypes []*FileType) bool {
	for _, validFileType := range validFileTypes{
		if fileType == validFileType{
			return true
		}
	}
	return false
}