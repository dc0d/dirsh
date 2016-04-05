//see LICENSE
package main

import (
	"os"
	"path/filepath"
	"sync"

	log "github.com/Sirupsen/logrus"
)

func listFilesInside(searchDir string) []string {
	fileList := []string{}
	err := filepath.Walk(searchDir, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}

		fileList = append(fileList, path)
		return nil
	})
	if err != nil {
		log.Error(err)
	}

	return fileList
}

func init() {
	log.SetFormatter(&log.TextFormatter{
		ForceColors: true,
	})
}

var (
	workerRegistry = new(sync.WaitGroup)
)

const (
	//XRealIP +
	XRealIP = "X-Real-IP"

	//XForwardedFor +
	XForwardedFor = "X-Forwarded-For"
)
