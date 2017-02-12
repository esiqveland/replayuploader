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
	// Do a small sleep to make sure sc2 has finished fully writing this file
	// before trying to reading it.
	// Usually when running SC2 through WINE, I see at least 3 writes to a replay file after a game,
	// some of which are partial writes of the file.
	// This should make sure we see the entire file on first read.
	time.Sleep(1*time.Second)

	return fh.handle(relPath)
}


func (fh *fileHandler) handle(relPath string) error {
	absPath := filepath.Join(fh.config.Dir, relPath)
	start := time.Now()

	defer func() {
		elapsed := time.Now().Unix() - start.Unix()
		log.Printf("File=%v took %vms", relPath, elapsed)
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


// shouldUpload returns false if we should not upload this file.
// e.g. if this is a zero-length file or we have seen this file before.
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
		log.Printf("error hashing file=%v", file.Name(), err)
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
