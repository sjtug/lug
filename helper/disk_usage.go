package helper

import (
	"os"
	"path"
)

// DiskUsage counts the disk usage of a directory
func DiskUsage(curPath string) (int64, error) {
	var size int64

	dir, err := os.Open(curPath)
	if err != nil {
		return size, err
	}
	defer dir.Close()

	files, err := dir.Readdir(-1)
	if err != nil {
		return size, err
	}

	for _, file := range files {
		if file.IsDir() {
			s, err := DiskUsage(path.Join(curPath, file.Name()))
			if err != nil {
				return size, err
			}
			size += s
		} else {
			size += file.Size()
		}
	}
	return size, nil
}
