package main

import (
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"net/url"
	"os"
	"os/exec"
	"path"
)

func main() {
	flag.Parse()
	var config GeneralConfig
	_, err := toml.DecodeFile(*flConfigFile, &config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	_, err = EnsureDirExists(config.SyncDir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, mirror := range config.Mirrors {
		if !mirror.Enabled {
			continue
		}
		uri, err := url.Parse(mirror.URL)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		dest := path.Join(config.SyncDir, uri.Host, uri.Path)
		_, err = EnsureDirExists(dest)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		cmd := exec.Command("rsync", "-avPHS", uri.String(), dest+"/")
		cmd.Stderr = os.Stderr // we'll want to see errors, regardless
		if !*flQuiet {
			cmd.Stdout = os.Stdout
		}

    err = cmd.Run()
    if err != nil {
			fmt.Fprintln(os.Stderr, err)
    }
	}
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
	SyncDir string `toml:"sync_dir"`
	Mirrors map[string]Mirror
}

type Mirror struct {
	Title   string `toml:"title"`
	URL     string `toml:"url"`
	Enabled bool   `toml:"enabled"`
}

var (
	flConfigFile = flag.String("c", path.Join(os.Getenv("HOME"), ".slackware-sync.toml"), "config file for the sync")
	flQuiet      = flag.Bool("q", false, "less output")
)
