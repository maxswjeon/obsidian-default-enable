package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/syndtr/goleveldb/leveldb"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: obsidian-default-enable <workspace ID>")
	}

	workspaceID := os.Args[1]
	if len(workspaceID) != 16 {
		log.Fatal("Workspace ID must be 16 characters long")
	}

	for _, char := range workspaceID {
		if !unicode.IsDigit(char) && (char < 'a' || char > 'f') {
			log.Fatal("Workspace ID must be a hex string")
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	dbPath := homeDir + "/.config/obsidian/Local Storage/leveldb"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Println("Obsidian is not initialized")

		log.Println("Running Obsidian once to initialize it")

		cmd := exec.Command("obsidian", "--no-sandbox")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatal(err)
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		outputRecieved := make(chan int64)
		go func() {
			stdoutScanner := bufio.NewScanner(stdout)
			stderrScanner := bufio.NewScanner(stderr)

			stdoutLine := ""
			stderrLine := ""
			for {
				hasStdout := stdoutScanner.Scan()
				hasStderr := stderrScanner.Scan()

				if !hasStdout && !hasStderr {
					break
				}

				outputRecieved <- time.Now().Unix()
				if hasStdout {
					stdoutLine += stdoutScanner.Text()
					for strings.Contains(stdoutLine, "\n") {
						fmt.Fprintf(os.Stdout, "[OBSIDIAN] %s\n", stdoutLine)
						stdoutLine = strings.Join(strings.Split(stdoutLine, "\n"), "\n")
					}
				}

				if hasStderr {
					stderrLine += stderrScanner.Text()
					for strings.Contains(stderrLine, "\n") {
						fmt.Fprintf(os.Stderr, "[OBSIDIAN] %s\n", stderrLine)
						stderrLine = strings.Join(strings.Split(stderrLine, "\n"), "\n")
					}
				}
			}

			if err := stdoutScanner.Err(); err != nil {
				log.Printf("Error reading stdout: %v\n", err)
			}
			if err := stderrScanner.Err(); err != nil {
				log.Printf("Error reading stderr: %v\n", err)
			}
		}()

		go func() {
			for {
				select {
				case <-outputRecieved:
					cancel()
					ctx, cancel = context.WithCancel(context.Background())
				case <-time.After(10 * time.Second):
					if err := cmd.Process.Kill(); err != nil {
						log.Fatal("failed to kill: ", err)
					}
					return
				case <-ctx.Done():
					return
				}
			}
		}()

		cmd.Start()
		cmd.Wait()

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			log.Fatal("Obsidian failed to initialize")
		}
	}

	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if db.Put([]byte("_app://obsidian.md/enable-plugin-"+workspaceID), []byte("true"), nil) != nil {
		log.Fatal(err)
	}

	log.Println("Enabled default plugins for workspace", workspaceID)
}
