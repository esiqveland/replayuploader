package replayuploader

import (
	"io"
	"net/http"
	"time"
	"mime/multipart"
	"bytes"
	"fmt"
	"log"
	"errors"
)

type Uploader interface {
	Upload(filename string, content io.Reader) error
}

type uploader struct {
	config Config
	client *http.Client
}

func (u *uploader) Upload(filename string, content io.Reader) error {
	buf := &bytes.Buffer{}

	mpWriter := multipart.NewWriter(buf)
	mpWriter.WriteField("hashkey", u.config.Hash)
	mpWriter.WriteField("hashkey", u.config.Token)
	mpWriter.WriteField("timestamp", fmt.Sprintf("%v", time.Now().Unix()))
	writ, err := mpWriter.CreateFormFile("Filedata", filename)
	if err != nil {
		return err
	}
	_, err = io.Copy(writ, content)
	if err != nil {
		return err
	}
	err = mpWriter.Close()
	if err != nil {
		return err
	}

	url := "https://sc2replaystats.com/upload_replay.php"

	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return err
	}
	res, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	log.Printf("[%v %v] %v", req.Method, url, res.Status)
	if res.StatusCode == 200 || res.StatusCode == 201 || res.StatusCode == 204 {
		return nil
	}

	return errors.New(fmt.Sprintf("Something went wrong, got StatusCode=%v", res.StatusCode))
}

func New(config Config) Uploader {
	client := &http.Client{
		Timeout: time.Second * 5,
	}
	return &uploader{
		config: config,
		client: client,
	}
}
