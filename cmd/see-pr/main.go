package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/config"
	"github.com/sirupsen/logrus"
)

var (
	DefaultConfigPath   = ".git/config"
	DefaultRepoPath     = "."
	DefaultUpstreamName = "origin"

	PullRequestRefSpec = "+refs/pull/*/head:refs/remotes/pr/*"
)

func init() {
	flag.StringVar(&DefaultConfigPath, "config", DefaultConfigPath, "path to the git config")
	flag.StringVar(&DefaultRepoPath, "path", DefaultRepoPath, "local path of the git repo")
	flag.StringVar(&DefaultUpstreamName, "remote", DefaultUpstreamName, "upstream remote name")
}

func main() {
	flag.Parse()

	path := filepath.Join(DefaultRepoPath, DefaultConfigPath)
	logrus.Infof("reading from %q", path)
	origBuf, err := ioutil.ReadFile(path)
	if err != nil {
		logrus.Fatal(err)
	}

	cfg := config.NewConfig()
	err = cfg.Unmarshal(origBuf)
	if err != nil {
		logrus.Fatal(err)
	}

	changesMade := false
	foundRemote := false
outer:
	for origin, remote := range cfg.Remotes {
		if origin != DefaultUpstreamName {
			logrus.Infof("skipping %s", origin)
			continue
		}
		foundRemote = true

		foundGithub := false
		for _, url := range remote.URLs {
			logrus.Infof("%s/%s URL: %s", origin, remote.Name, url)
			if strings.Contains(url, "github.com") {
				foundGithub = true
			}
		}
		if !foundGithub {
			logrus.Warn("no github remote URL found. PR RefSpec doesn't make sense here")
			break outer
		}
		for _, fetch := range remote.Fetch {
			if fetch.String() == PullRequestRefSpec {
				logrus.Infof("PR Fetch RefSpec is already here!")
				break outer
			}
			logrus.Debugf("%s/%s = %s", origin, remote.Name, fetch)
		}
		logrus.Infof("appending fetch = %s", PullRequestRefSpec)
		remote.Fetch = append(remote.Fetch, config.RefSpec(PullRequestRefSpec))
		changesMade = true
	}
	if !foundRemote {
		logrus.Fatalf("failed to find %q remote", DefaultUpstreamName)
	}

	// write back the Marshalled config
	if changesMade {
		newBuf, err := cfg.Marshal()
		if err != nil {
			logrus.Fatalf("failed to marshal back: %s", err)
		}
		//fmt.Println(string(newBuf))
		if err := ioutil.WriteFile(path, newBuf, os.FileMode(0644)); err != nil {
			logrus.Fatalf("failed to rewrite %q: %s", path, err)
		}
		logrus.Infof("SUCCESS! `git fetch` and then you can `git checkout pr/$NUM` of your PRs")
	}
}
