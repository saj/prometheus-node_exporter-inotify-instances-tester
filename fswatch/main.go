package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
)

func main() {
	log.SetFlags(log.Lshortfile)

	if len(os.Args) < 2 {
		log.Fatal("expects path argument")
	}
	name := os.Args[1]

	// e2e test driver IPC and synchronisation:
	//
	// fd 3 is a readable pipe used by the e2e test driver to terminate fswatch
	// after a test.  The e2e test driver executes fswatch as root; this pipe
	// bridges an unprivileged driver with a privileged fswatch.
	//
	// fd 4 is a writable pipe used by fswatch to indicate to the e2e test
	// driver that the inotify instance is open.  The driver will block until
	// receives this indication.  In our case, it is not sufficient to simply
	// close the pipe because sudo(8) will dup the fd before it forks us.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		f := os.NewFile(3, "pipe")
		if f == nil {
			return
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if scanner.Text() == "die" {
				cancel()
				return
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("C&C pipe: %v", err)
		}
	}()

	ready := func() {
		f := os.NewFile(4, "pipe")
		if f == nil {
			return
		}
		fmt.Fprintln(f, "ready")
		f.Close()
	}

	if err := watch(ctx, name, ready); err != nil {
		log.Fatal(err)
	}
}

func watch(ctx context.Context, name string, ready func()) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		defer close(done)
		ready()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			case <-ctx.Done():
				return
			}
		}
	}()

	if err := watcher.Add(name); err != nil {
		return err
	}
	<-done
	return nil
}
