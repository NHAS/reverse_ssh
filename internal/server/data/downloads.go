package data

import (
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Download struct {
	gorm.Model

	UrlPath string `gorm:"unique"`

	CallbackAddress string
	FilePath        string
	LogLevel        string
	Goos            string
	Goarch          string
	Goarm           string
	FileType        string
	Hits            int
	Version         string
	FileSize        float64

	// when generating the template use the host header
	UseHostHeader bool

	// Where to download the file to
	WorkingDirectory string
}

func CreateDownload(file Download) error {
	// Create the Download record in the database
	return db.Create(&file).Error
}

func GetDownload(urlPath string) (Download, error) {
	var download Download
	if err := db.Where("url_path = ?", urlPath).First(&download).Error; err != nil {
		return download, err
	}

	if err := db.Model(&Download{}).Where("url_path = ?", urlPath).Update("hits", download.Hits+1).Error; err != nil {
		return download, err
	}

	return download, nil
}

func ListDownloads(filter string) (matchingFiles map[string]Download, err error) {
	_, err = filepath.Match(filter, "")
	if err != nil {
		return nil, fmt.Errorf("filter is not well formed")
	}

	matchingFiles = make(map[string]Download)

	var downloads []Download
	if err := db.Find(&downloads).Error; err != nil {
		return nil, err
	}

	for _, file := range downloads {
		if filter == "" {
			matchingFiles[file.UrlPath] = file
			continue
		}

		if match, _ := filepath.Match(filter, file.UrlPath); match {
			matchingFiles[file.UrlPath] = file
			continue
		}

		if match, _ := filepath.Match(filter, file.Goos); match {
			matchingFiles[file.UrlPath] = file
			continue
		}

		if match, _ := filepath.Match(filter, file.Goarch+file.Goarm); match {
			matchingFiles[file.UrlPath] = file
			continue
		}
	}

	return
}

func DeleteDownload(key string) error {

	// Fetch the Download record from the database based on the key
	var download Download
	if err := db.Unscoped().Clauses(clause.Returning{}).Where("url_path = ?", key).Delete(&download).Error; err != nil {
		return err
	}

	return os.Remove(download.FilePath)
}
