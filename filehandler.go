package replayuploader

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StateFile stores data between runs
type StateFile struct {
	// Status holds results of replays we have seen
	// Format is Checksum -> success
	Status map[string]bool `json:"status"`
}

type Config struct {
	Dir   string
	Token string
	Hash  string
	// Path to statefile
	DataFile string
	MaxTries int
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
	if c.MaxTries <= 0 {
		return errors.New("MaxTries must be >0")
	}
	if c.DataFile == "" {
		return errors.New("DataFile is empty")
	}
	return nil
}

type FileHandler interface {
	// NewFile takes a path relative to config.dir
	NewFile(relPath string) error
}

type fileHandler struct {
	config    Config
	state     StateFile
	uploader  Uploader
	filesDone map[string]bool
	lock      sync.RWMutex
}

func (fh *fileHandler) NewFile(relPath string) error {
	// Do a small sleep to make sure sc2 has finished fully writing this file
	// before trying to reading it.
	// Usually when running SC2 through WINE, I see at least 3 writes to a replay file after a game,
	// some of which are partial writes of the file.
	// This should make sure we see the entire file on first read.
	time.Sleep(1 * time.Second)

	return fh.handle(relPath)
}

func (fh *fileHandler) handle(relPath string) error {
	absPath := filepath.Join(fh.config.Dir, relPath)
	start := time.Now()

	defer func() {
		elapsed := time.Since(start)
		log.Printf("File=%v took %v", relPath, elapsed)
	}()

	fd, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer fd.Close()

	hasher := sha512.New()
	// make sure we reset file reader for other clients
	fd.Seek(0, 0)

	_, err = io.Copy(hasher, fd)
	if err != nil {
		log.Printf("error hashing file=%v: %v", fd.Name(), err)
		return err
	}
	data := hasher.Sum(nil)
	shaSum := base64.StdEncoding.EncodeToString(data)

	if fh.shouldUpload(shaSum, fd) {
		err = fh.uploader.Upload(relPath, fd)
		if err != nil {
			return err
		} else {
			fh.markCompleted(shaSum)
		}
	} else {
		log.Printf("Skipping file=%v because we have uploaded this before.", fd.Name())
	}

	return nil
}

func (fh *fileHandler) markCompleted(shaSum string) {
	fh.lock.Lock()
	defer fh.lock.Unlock()

	fh.filesDone[shaSum] = true

	fd, err := os.Create(fh.config.DataFile)
	if err != nil {
		log.Printf("Error opening data file for saving progress: %v", err)
		return
	}
	defer fd.Close()

	data := StateFile{
		Status: fh.filesDone,
	}

	err = json.NewEncoder(fd).Encode(&data)
	if err != nil {
		log.Printf("Error marshalling status file: %v", err)
	}
}

// shouldUpload returns false if we should not upload this file.
// e.g. if this is a zero-length file or we have seen this file before.
func (fh *fileHandler) shouldUpload(shaSum string, file *os.File) bool {
	fh.lock.Lock()
	defer fh.lock.Unlock()

	stat, _ := file.Stat()
	size := stat.Size()

	isDone, ok := fh.filesDone[shaSum]
	if ok && isDone {
		return false
	} else {
		fh.filesDone[shaSum] = false
		return size > 0
	}
}

func CreateFileHandler(config Config, uploader Uploader, data StateFile) FileHandler {
	log.Printf("Loaded %v files done.", len(data.Status))
	return &fileHandler{
		config:    config,
		state:     data,
		uploader:  uploader,
		filesDone: data.Status,
		lock:      sync.RWMutex{},
	}
}
