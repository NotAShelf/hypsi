package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
)

const VERSION = "1.0"

const MESSAGE = `
hypsi %s

usage: hypsi [ <file> | <args> ]

   <file>	To set the desktop wallpaper of your focused monitor, simply provide the absolute path to your desired image file.

alternatively by sending <args>, you can:

   -json	Show the current configuration in JSON format

   -rewind	rewind config via logfile

   -webview	open with webkitgtk
`

var HYPSI_STATE APPLICATION_STATE

func main() {
	var port string

	port = os.Getenv("PORT")

	if len(port) == 0 {
		port = "3000"
	}

	iPort, _ := strconv.Atoi(port)

	// interrupt if running already
	_, _err := http.Get(fmt.Sprintf("http://localhost:%d/interrupt", iPort))

	if _err != nil {
		// probably not running
	}

	var watcher Publisher
	loadState() // last application state

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)

	signal.Notify(c, os.Interrupt)

	defer func() {
		saveState()        // save application state
		writeConfig(false) // write config file
		signal.Stop(c)     // stop the channel
		cancel()           // cancel the context
		if HYPSI_STATE.Message != "ok" {
			// show any unexpected messages
			fmt.Println(HYPSI_STATE.Message)
		}
		unloadWallpaper("all") // free memory
	}()

	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()

	UPLOADS := fmt.Sprintf("%s/wallpaper", os.Getenv("HOME"))
	// ensure the "upload" folder exists
	if _, err := os.Stat(UPLOADS); os.IsNotExist(err) {
		// create with 0755 permissions (read, write, and execute for owner, read and execute for group and others)
		err := os.MkdirAll(UPLOADS, 0755)
		if err != nil {
			log.Fatal(err) // Handle the error appropriately
		}
	}
	argsWithoutProg := os.Args[1:]

	if len(argsWithoutProg) > 0 {

		switch argsWithoutProg[0] {
		case "-json":
			fmt.Print(jsonText())

		case "-rewind":
			if len(argsWithoutProg) > 1 {
				i, err := strconv.Atoi(argsWithoutProg[1])
				if err != nil {
					fmt.Println("argument must be a number")
					return
				}
				rewind(i)
			} else {
				rewind(1)
			}
		case "-write":
			// log changes & write hyprpaper.config
			// (undocumented dev feature atm)
			writeConfig(true)

		case "-free":
			// free memory
			// (undocumented dev feature atm)
			unloadWallpaper("all")

		case "-webview":
			go api()
			gtkView(watcher)

		case "-develop":
			CWD, _ := os.Getwd()
			files := []string{"webview.html.tmpl", "page.html.tmpl"}
			for _, filename := range files {
				localFile := filepath.Join(CWD, filename)
				if _, err := os.Stat(localFile); os.IsNotExist(err) {
					data, _ := WEBFOLDER.ReadFile(fmt.Sprintf("web/%s", filename))
					f, err := os.Create(localFile)
					if err != nil {
						log.Fatal(err)
					}
					defer f.Close()
					fmt.Fprintf(f, string(data))
					fmt.Println(localFile)
				}
			}

		case "-watch":
			var watchfolder string

			if len(argsWithoutProg) > 1 {
				watchfolder = argsWithoutProg[1]
				if _, err := os.Stat(watchfolder); os.IsNotExist(err) {
					log.Fatalf("Cannot watch %s, the path does not exist", watchfolder)
				}
			} else {
				fmt.Println("... no folder specified, watch working directory")
				watchfolder, _ = os.Getwd()
			}
			os.Setenv("HYPSI_WEBVIEW", filepath.Join(watchfolder, "webview.html.tmpl"))
			os.Setenv("HYPSI_WEBPAGE", filepath.Join(watchfolder, "page.html.tmpl"))
			fmt.Printf("\n\n[ 👀 watching %s]\n", watchfolder)
			watcher = NewPathWatcher(watchfolder)
			go api()
			go watcher.observe()
			gtkView(watcher)

		default:
			readFromCLI(argsWithoutProg)
		}

	} else {
		fmt.Printf(MESSAGE, VERSION)
	}

}
