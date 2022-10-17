package file

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setupDirectory(t *testing.T, directory string) func(t *testing.T) {
	err := os.MkdirAll(directory, os.ModePerm)
	require.NoError(t, err)
	fmt.Println("setup directory:", directory)
	return func(t *testing.T) {
		os.RemoveAll(directory)
		fmt.Println("cleanup directory")
	}
}

func setupBlockFiles(directory string, start, end int64) {
	for i := start; i <= end; i++ {
		file := getFile(directory, i)
		os.WriteFile(file, []byte(fmt.Sprint("block", i)), 0644)
	}
}

func getFile(directory string, blockNum int64) string {
	return fmt.Sprintf("%s/block-%d-data", directory, blockNum)
}

func start(watcher *BlockFileWatcher, endBlockNum int64) int64 {
	watcher.Start(1, time.Microsecond)
	counter := int64(0)
	for data := range watcher.SubscribeData() {
		if data != nil && len(data.Data) > 0 {
			counter++
		}
		if data.BlockNum == endBlockNum {
			return counter
		}
	}
	return counter
}

func TestFileWatcher(t *testing.T) {
	directory := "tmp"
	teardown := setupDirectory(t, directory)
	startBlockNum := int64(1)
	endBlockNum := int64(2)

	defer teardown(t)

	t.Run("when sync via local", func(t *testing.T) {
		setupBlockFiles(directory, startBlockNum, endBlockNum)
		watcher := NewBlockFileWatcher(func(blockNum int64) string {
			return getFile(directory, blockNum)
		}, true)
		total := start(watcher, endBlockNum)
		expected := endBlockNum - startBlockNum + 1
		require.Equal(t, total, expected)
	})

	t.Run("when sync via http", func(t *testing.T) {
		setupBlockFiles(directory, startBlockNum, endBlockNum)
		http.Handle("/", http.FileServer(http.Dir(directory)))
		port := "8080"
		fmt.Printf("Serving %s on HTTP port: %s\n", directory, port)
		go func() {
			log.Fatal(http.ListenAndServe(":"+port, nil))
		}()
		watcher := NewBlockFileWatcher(func(blockNum int64) string {
			return fmt.Sprintf("http://localhost:%s/block-%d-data", port, blockNum)
		}, false)
		total := start(watcher, endBlockNum)
		expected := endBlockNum - startBlockNum + 1
		require.Equal(t, total, expected)
	})
}
