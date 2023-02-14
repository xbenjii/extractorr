package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/joho/godotenv"
	"github.com/mholt/archiver/v4"
)

/**
 * Open a file and retry if it fails.
 * @param path the path to the file to open
 * @return the file or an error
 */
func OpenFile(path string) (*os.File, error) {
	return os.Open(path)
}

/**
 * Retry a function until it succeeds or the number of attempts is reached.
 * @param attempts the number of attempts to make
 * @param sleep the number of seconds to sleep between attempts
 * @param fn the function to call
 * @return the result of the function or an error
 */
func retry[T any](attempts int, sleep int, fn func() (T, error)) (result T, err error) {
	for i := 0; i < attempts; i++ {
		if i > 0 {
			log.Println("retrying after error:", err)
			time.Sleep(time.Duration(sleep) * time.Second)
			sleep *= 2
		}
		result, err = fn()
		if err == nil {
			return result, nil
		}
	}
	return result, fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

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

					// TODO: Fix retry after success
					input, err := retry(5, 5, func() (*os.File, error) {
						return OpenFile(event.Name)
					})

					if err != nil {
						log.Print("open file:", err)
						return
					}

					// Identify what file format we're using
					format, file, err := archiver.Identify(event.Name, input)
					if err != nil {
						log.Print("identify file:", err)
						return
					}

					inputFilename := filepath.Base(event.Name)
					outputPath := strings.TrimSuffix(inputFilename, filepath.Ext(inputFilename))
					fullOutputPath := filepath.Join(os.Getenv("OUTPUT_DIR"), outputPath)

					handler := func(ctx context.Context, f archiver.File) error {
						log.Println("Extracting file:", f.NameInArchive)

						// If it's a directory we need to create it in the destination
						if f.IsDir() {
							if _, err := os.Stat(filepath.Join(fullOutputPath, f.NameInArchive)); os.IsNotExist(err) {
								err := os.MkdirAll(filepath.Join(fullOutputPath, f.NameInArchive), 0755)
								if err != nil {
									log.Print(err)
								}
							}
							return err
						} else {
							// Open source file inside archive for reading
							src, err := f.Open()
							if err != nil {
								log.Print(err)
								return err
							}
							defer src.Close()

							// Create destination file to write to
							dest, err := os.Create(filepath.Join(fullOutputPath, f.NameInArchive))
							if err != nil {
								log.Print(err)
								return err
							}
							defer dest.Close()

							// Copy data from source to destination
							_, err = io.Copy(dest, src)

							return err
						}
					}

					// Create the output directory if it doesn't exist
					if _, err := os.Stat(fullOutputPath); os.IsNotExist(err) {
						err := os.MkdirAll(fullOutputPath, 0755)
						if err != nil {
							log.Print(err)
							return
						}
					}

					// Extract the file
					if ex, ok := format.(archiver.Extractor); ok {
						ctx := context.Background()
						ex.Extract(ctx, file, nil, handler)
						if err != nil {
							log.Print(err)
							return
						}
						log.Println("Extracted file:", event.Name)
						input.Close()

						// Delete the file
						if os.Getenv("DELETE_FILE") == "yes" {
							err = os.Remove(event.Name)
							log.Println("Deleted file:", event.Name)
							if err != nil {
								log.Print(err)
								return
							}
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Fatal("error:", err)
			}
		}
	}()

	err = watcher.Add(os.Getenv("WATCH_DIR"))
	if err != nil {
		log.Fatal(err)
	}
	<-done
}
