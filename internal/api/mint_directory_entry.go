package api

type RwxDirectoryEntry struct {
	OriginalPath string `json:"-"`
	Path         string `json:"path"`
	Type         string `json:"type"`
	Permissions  uint32 `json:"permissions"`
	FileContents string `json:"file_contents"`
}

func (e RwxDirectoryEntry) IsDir() bool {
	return e.Type == "dir"
}

func (e RwxDirectoryEntry) IsFile() bool {
	return e.Type == "file"
}
