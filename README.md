# utils

Random utilities of vbatts' being cleaned up, and published

# Commands

## see-pr

Add the github PR refspec to a local git repo.

### Install

	go get github.com/vbatts/utils/cmd/see-pr

### Usage

	$ see-pr -h
	Usage of see-pr:
	  -config string
	        path to the git config (default ".git/config")
	  -path string
	        local path of the git repo (default ".")
	  -remote string
	        upstream remote name (default "origin")

Then to let it update your `.git/config`:

	$ see-pr
	INFO[0000] reading from ".git/config"                   
	INFO[0000] origin/origin URL: git@github.com:vbatts/utils.git 
	INFO[0000] appending fetch = +refs/pull/*/head:refs/remotes/pr/* 
	INFO[0000] SUCCESS! `git fetch` and then you can `git checkout pr/$NUM` of your PRs 

## dups

building a document of file checksum info, for a directory tree. Optionally
deduplicate the tree using hardlinks.

### Install

	go get github.com/vbatts/utils/cmd/dups

### Usage

	$ dups -h
	Usage of dups:
	  -H=false: hardlink the duplicate files
	  -l="": load existing map from file
	  -q=false: less output
	  -s="hash-map.json": file to save map of file hashes to

By default it scans the paths provided, and creates a JSON document of the file paths and their checksum:

	$ dups .
	"/home/vbatts/src/vb/utils/.git/logs/refs/heads/master" is the same content as "/home/vbatts/src/vb/utils/.git/logs/HEAD"
	"/home/vbatts/src/vb/utils/.git/refs/remotes/origin/master" is the same content as "/home/vbatts/src/vb/utils/.git/refs/heads/master"
	"/home/vbatts/src/vb/utils/cmd/find-todos/main.go~" is the same content as "/home/vbatts/src/vb/utils/cmd/find-todos/main.go"
	"/home/vbatts/src/vb/utils/cmd/slackware-sync/README.md~" is the same content as "/home/vbatts/src/vb/utils/cmd/slackware-sync/README.md"
	"/home/vbatts/src/vb/utils/cmd/slackware-sync/main.go" is the same content as "/home/vbatts/src/vb/utils/cmd/slackware-sync/main.go~"
	Savings of 0.005681mb
	wrote "hash-map.json"

With the `-H` flag, as duplicate files are found (files with matching checksum)
are encountered, hardlink it to the duplicate file.


## next-note

Simple date formating for notes

### Install

	go get github.com/vbatts/utils/cmd/next-note

### Usage

	next-note -h
	Usage of next-note:
	 -c=false: print current week's filename
	 -d=false: print current date
	 -dir="": Base directory for tasks
	 -p=false: print previous week's filename

### vim

Copy ./cmd/next-note/next-note.vim to ~/.vim/plugin/

This provides commands, like:

* `:Nd` - append the date to the current buffer
* `:Nc` - to open a tabe for current week's notes file
* `:Np` - to open a tabe for previous week's notes file
* `:Nn` - to open a tabe for next week's notes file

## find-todos

Look through your notes directory for TODO items.

### Install

	go get github.com/vbatts/utils/cmd/find-todos

