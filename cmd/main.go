package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/esiqveland/replayuploader"
	"github.com/fsnotify/fsnotify"
)

var dirFlag = flag.String("dir", "", "Directory where replays show up.")
var tokenFlag = flag.String("token", "", "Token to use when uploading.")
var hashFlag = flag.String("hash", "", "Hash value to use when uploading.")
var triesFlag = flag.Int("maxtries", 5, "Max number of retries to upload a replay.")
var dataFlag = flag.String("data", "state.json", "Filename to store data between runs.")

func main() {
	flag.Parse()
	cfg := replayuploader.Config{
		Dir:      *dirFlag,
		Token:    *tokenFlag,
		Hash:     *hashFlag,
		MaxTries: *triesFlag,
		DataFile: *dataFlag,
	}

	err := cfg.HasError()
	if err != nil {
		log.Printf("Error with config: %v ", err)
		os.Exit(-1)
	}

	os.Exit(run(cfg))
}

func run(cfg replayuploader.Config) int {
	upl := replayuploader.New(cfg)

	fd, err := os.Open(cfg.DataFile)
	if err != nil {
		log.Printf("Error opening datafile: %v", err)
	} else {
		defer fd.Close()
	}
	dataFile := replayuploader.StateFile{
		Status: make(map[string]bool),
	}
	err = json.NewDecoder(fd).Decode(&dataFile)
	if err != nil {
		log.Printf("Error with statefile: %v", err)
	}

	fHandler := replayuploader.CreateFileHandler(cfg, upl, dataFile)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Error creating watcher: %v", err)
		return -1
	}
	defer watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				//log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					relPath, err := filepath.Rel(cfg.Dir, event.Name)
					if err != nil {
						log.Printf("error getting relative path for name: %v", event.Name)
						continue
					}
					handlerErr := fHandler.NewFile(relPath)
					if handlerErr != nil {
						log.Printf("Error handling file=%v: %v", event.Name, handlerErr)
					} else {
						log.Printf("Handled file successfully: %v", relPath)
					}
				}
			case err := <-watcher.Errors:
				log.Println("Watcher got error:", err)
			}
		}
	}()

	err = watcher.Add(cfg.Dir)
	if err != nil {
		log.Printf("Error watching dir '%v': %v", cfg.Dir, err)
		return -1
	} else {
		log.Printf("Now watching directory: %v", cfg.Dir)
	}
	<-done
	return 0
}
