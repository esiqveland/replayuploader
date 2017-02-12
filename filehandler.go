package replayuploader

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Dir   string
	Token string
	Hash  string
}

func (c *Config) HasError() error {
	if c.Dir == "" {
		return errors.New("dir is empty")
	}
	if c.Token == "" {
		return errors.New("token is empty")
	}
	if c.Hash == "" {
		return errors.New("hash is empty")
	}
	return nil
}

type FileHandler interface {
	// NewFile takes a path relative to config.dir
	NewFile(relPath string) error
}

type fileHandler struct {
	config   Config
	uploader Uploader
}

func (fh *fileHandler) NewFile(relPath string) error {
	absPath := filepath.Join(fh.config.Dir, relPath)
	start := time.Now()
	log.Printf("NewFile=%v", relPath)

	defer func() {
		elapsed := time.Now().Unix() - start.Unix()
		log.Printf("NewFile=%v took %vms", relPath, elapsed)
	}()

	fd, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer fd.Close()

	if shouldUpload(fd) {
		err = fh.uploader.Upload(relPath, fd)
		if err != nil {
			return err
		}
	} else {
		log.Printf("skipping file: %v", relPath)
	}

	return nil
}

func shouldUpload(file *os.File) bool {
	stat, _ := file.Stat()
	size := stat.Size()

	log.Printf("[%v] %vbytes", file.Name(), size)
	return size > 0
}

func CreateFileHandler(config Config, uploader Uploader) FileHandler {
	return &fileHandler{
		config:   config,
		uploader: uploader,
	}
}
