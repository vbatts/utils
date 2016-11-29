package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"sync"

	"github.com/BurntSushi/toml"
)

var (
	flSyncDir    = flag.String("dir", "", "directory to sync to (this flag overrides the sync_dir in the configuration file)")
	flConfigFile = flag.String("c", path.Join(os.Getenv("HOME"), ".slackware-sync.toml"), "config file for the sync")
	flThreads    = flag.Int("t", 1, "threads to fetch with")
	flQuiet      = flag.Bool("q", false, "less output")
)

func main() {
	flag.Parse()
	var config GeneralConfig
	if _, err := toml.DecodeFile(*flConfigFile, &config); err != nil {
		log.Fatal(err)
	}

	if len(*flSyncDir) > 0 {
		config.SyncDir = *flSyncDir
	}
	if *flThreads > 1 {
		config.Threads = *flThreads
	}

	if _, err = EnsureDirExists(config.SyncDir); err != nil {
		log.Fatal(err)
	}

	workers := make(chan int, config.Threads)
	wg := sync.WaitGroup{}
	for name, mirror := range config.Mirrors {
		if !mirror.Enabled {
			continue
		}
		uri, err := url.Parse(mirror.URL)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		dest := path.Join(config.SyncDir, uri.Host, uri.Path)
		if _, err = EnsureDirExists(dest); err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		wg.Add(1)
		rsyncFunc := func() {
			if config.Threads > 1 {
				workers <- 1
			}
			defer func() {
				if config.Threads > 1 {
					<-workers
				}
				wg.Done()
			}()
			cmd := exec.Command("rsync", "-avPHS", "--delete", uri.String(), dest+"/")
			cmd.Stderr = os.Stderr // we'll want to see errors, regardless
			if !*flQuiet {
				cmd.Stdout = os.Stdout
			}

			if err = cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "%q: %s", name, err)
			}
		}
		if config.Threads > 1 {
			go rsyncFunc()
		} else {
			rsyncFunc()
		}
	}
	wg.Wait()
}

func EnsureDirExists(path string) (os.FileInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return stat, err
		}
		if err = os.MkdirAll(path, 0755); err != nil {
			return stat, err
		}
		if stat, err = os.Stat(path); err != nil {
			return stat, err
		}
	}
	return stat, nil
}

type GeneralConfig struct {
	Threads int    `toml:"threads"`
	SyncDir string `toml:"sync_dir"`
	Mirrors map[string]Mirror
}

type Mirror struct {
	Title   string `toml:"title"`
	URL     string `toml:"url"`
	Enabled bool   `toml:"enabled"`
}
