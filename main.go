package main

import (
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/mholt/archiver/v4"
)

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
					input, err = os.Open(event.Name)
					if err != nil {
						return err
					}
					format, input, err := archiver.Identify(event.Name, input)
					if err != nil {
						return err
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
