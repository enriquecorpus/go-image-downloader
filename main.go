package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

var (
	downloadsPath = ""
	downloadChan  = make(chan *DownloadRequest, 500)
)

type DownloadRequest struct {
	DownloadPath string
	RespChannel  chan *DownloadResponse
	ctx          context.Context
}

type DownloadResponse struct {
	Data []byte
	err  error
}

func download(req *DownloadRequest) {
	print("Downloading -----> ", req.DownloadPath)
	resp := &DownloadResponse{}
	response, err := http.Get(req.DownloadPath)
	if err != nil {
		resp.err = err
		goto end
	}

	if response.StatusCode != 200 {
		resp.err = errors.New("Received non 200 response code")
		goto end
	}
	resp.Data, err = ioutil.ReadAll(response.Body)
	if err != nil {
		resp.err = err
		goto end
	}
	_ = response.Body.Close()

end:
	select {
	case <-req.ctx.Done():
		return // context aborted, don't send response
	default:
		req.RespChannel <- resp
	}
}

func processDownload() {
	for downloadReqs := range downloadChan {
		download(downloadReqs)
	}
	print("Done")
}

func requestDownload(key string) (data []byte, er error) {
	fileName := key + ".jpg"
	downloadLink := "https://randomfox.ca/images/" + fileName
	c := make(chan *DownloadResponse)
	ctx, cancel := context.WithCancel(context.Background())
	defer close(c)
	downloadChan <- &DownloadRequest{downloadLink, c, ctx}

	var resp *DownloadResponse
	select {
	case r := <-c:
		resp = r
	case <-time.After(15 * time.Second):
		cancel()
		resp = &DownloadResponse{err: errors.New("Timeout")}
	}
	cancel() // Don't leak
	if resp.err != nil {
		fmt.Println("Error making dl ", resp.err)
		return nil, resp.err
	}

	fileFullPath := path.Join(downloadsPath, fileName)
	err := ioutil.WriteFile(fileFullPath, resp.Data, 0644)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil

}

func main() {
	p, err := os.Getwd()
	if err != nil {
		log.Println(err)
		return
	}
	go processDownload()
	downloadsPath = path.Join(p, "downloads")
	for i := 1; i <= 123; i++ {
		data, err := requestDownload(strconv.Itoa(i))
		if err != nil {
			print(err)
			continue
		}
		print(len(data))
	}

}
