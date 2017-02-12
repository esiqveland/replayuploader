package replayuploader

import (
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
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
	memoize  map[string]string
	lock     sync.RWMutex
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

	if fh.shouldUpload(fd) {
		err = fh.uploader.Upload(relPath, fd)
		if err != nil {
			return err
		}
	} else {
		log.Printf("skipping file: %v", relPath)
	}

	return nil
}

func (fh *fileHandler) shouldUpload(file *os.File) bool {
	fh.lock.Lock()
	defer fh.lock.Unlock()

	hash := sha512.New()
	stat, _ := file.Stat()
	size := stat.Size()

	// make sure we reset file reader for other clients
	defer file.Seek(0, 0)

	_, err := io.Copy(hash, file)
	if err != nil {
		log.Printf("error hashing file=%v", file.Name())
		return false
	}
	data := hash.Sum(nil)
	shaSum := base64.StdEncoding.EncodeToString(data)

	log.Printf("[%v] %v %vbytes", file.Name(), shaSum, size)

	_, ok := fh.memoize[shaSum]
	if ok {
		log.Printf("Skipping file=%v we have seen this before", file.Name())
		return false
	} else {
		fh.memoize[shaSum] = file.Name()
		return size > 0
	}
}

func CreateFileHandler(config Config, uploader Uploader) FileHandler {
	return &fileHandler{
		config:   config,
		uploader: uploader,
		memoize:  make(map[string]string),
		lock:     sync.RWMutex{},
	}
}
