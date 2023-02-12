package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mholt/archiver/v4"
)

func OpenFile(path string) (*os.File, error) {
	return os.Open(path)
}

func RetryOpenFileWithDelay(path string) (*os.File, error) {
	file, err := OpenFile(path)
	if err != nil {
		ch := make(chan string, 1)
		ctxTimeout, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()
		go func() {
			ch <- "retry"
		}()
		select {
		case <-ctxTimeout.Done():
			return nil, err
		case <-ch:
			file, err = OpenFile(path)
			if err != nil {
				return nil, err
			}
		}
	}
	return file, nil
}

func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)

					input, err := RetryOpenFileWithDelay(event.Name)
					if err != nil {
						log.Fatal(err)
						return
					}

					format, file, err := archiver.Identify(event.Name, input)
					if err != nil {
						log.Fatal(err)
						return
					}

					handler := func(ctx context.Context, f archiver.File) error {
						log.Println("Extracting file:", f.Name())
						// err := archiver.CopyFile(f, filepath.Join(os.Getenv("OUTPUT_DIR"), f.Name()))
						// Move file to output dir
						err := os.Rename(f.Name(), filepath.Join(os.Getenv("OUTPUT_DIR"), f.Name()))
						if err != nil {
							log.Fatal(err)
							return err
						}
						return nil
					}

					if ex, ok := format.(archiver.Extractor); ok {
						ctx := context.Background()
						ex.Extract(ctx, file, nil, handler)
						if err != nil {
							log.Fatal(err)
							return
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(os.Getenv("WATCH_DIR"))
	if err != nil {
		log.Fatal(err)
	}
	<-done
}
