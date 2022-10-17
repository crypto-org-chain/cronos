package file

import (
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"
)

var errNotExist = errors.New("file not exist")

type fileDownloader interface {
	GetData(path string) ([]byte, error)
}

type localFileDownloader struct{}

func (d *localFileDownloader) GetData(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = errNotExist
		}
		return nil, err
	}
	return data, nil
}

type httpFileDownloader struct{}

func (d *httpFileDownloader) GetData(path string) ([]byte, error) {
	resp, err := http.Get(path) //nolint
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if resp.StatusCode == http.StatusNotFound {
			return nil, errNotExist
		}
		return nil, errors.New(resp.Status)
	}
	return ioutil.ReadAll(resp.Body)
}

type BlockData struct {
	BlockNum int64
	Data     []byte
}

type BlockFileWatcher struct {
	getFilePath func(blockNum int64) string
	downloader  fileDownloader
	chData      chan *BlockData
	chError     chan error
	chDone      chan bool
	startLock   *sync.Mutex
}

func NewBlockFileWatcher(
	getFilePath func(blockNum int64) string,
	isLocal bool,
) *BlockFileWatcher {
	w := &BlockFileWatcher{
		getFilePath: getFilePath,
		chData:      make(chan *BlockData),
		chError:     make(chan error),
		startLock:   new(sync.Mutex),
	}
	if isLocal {
		w.downloader = new(localFileDownloader)
	} else {
		w.downloader = new(httpFileDownloader)
	}
	return w
}

func (w *BlockFileWatcher) SubscribeData() <-chan *BlockData {
	return w.chData
}

func (w *BlockFileWatcher) SubscribeError() <-chan error {
	return w.chError
}

func (w *BlockFileWatcher) Start(
	blockNum int64,
	interval time.Duration,
) {
	w.startLock.Lock()
	defer w.startLock.Unlock()
	if w.chDone != nil {
		return
	}

	w.chDone = make(chan bool)
	go func() {
		for {
			select {
			case <-w.chDone:
				return

			default:
				path := w.getFilePath(blockNum)
				data, err := w.downloader.GetData(path)
				if err != nil {
					if err != errNotExist {
						// avoid blocked by error when not subscribe
						select {
						case w.chError <- err:
						default:
						}
					}
				} else {
					w.chData <- &BlockData{
						BlockNum: blockNum,
						Data:     data,
					}
					blockNum++
				}
				time.Sleep(interval)
			}
		}
	}()
}

func (w *BlockFileWatcher) Close() {
	w.startLock.Lock()
	defer w.startLock.Unlock()
	if w.chDone != nil {
		close(w.chDone)
		w.chDone = nil
	}
}
