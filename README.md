# freezing-octo-hipster

Random utilities of vbatts' being cleaned up, and published

# Commands

## dups

building a document of file checksum info, for a directory tree. Optionally
deduplicate the tree using hardlinks.

### Install

	go get github.com/vbatts/freezing-octo-hipster/cmd/dups

### Usage

	$ dups -h
	Usage of dups:
	  -H=false: hardlink the duplicate files
	  -l="": load existing map from file
	  -q=false: less output
	  -s="hash-map.json": file to save map of file hashes to

By default it scans the paths provided, and creates a JSON document of the file paths and their checksum:

	$ dups .
	"/home/vbatts/src/vb/freezing-octo-hipster/.git/logs/refs/heads/master" is the same content as "/home/vbatts/src/vb/freezing-octo-hipster/.git/logs/HEAD"
	"/home/vbatts/src/vb/freezing-octo-hipster/.git/refs/remotes/origin/master" is the same content as "/home/vbatts/src/vb/freezing-octo-hipster/.git/refs/heads/master"
	"/home/vbatts/src/vb/freezing-octo-hipster/cmd/find-todos/main.go~" is the same content as "/home/vbatts/src/vb/freezing-octo-hipster/cmd/find-todos/main.go"
	"/home/vbatts/src/vb/freezing-octo-hipster/cmd/slackware-sync/README.md~" is the same content as "/home/vbatts/src/vb/freezing-octo-hipster/cmd/slackware-sync/README.md"
	"/home/vbatts/src/vb/freezing-octo-hipster/cmd/slackware-sync/main.go" is the same content as "/home/vbatts/src/vb/freezing-octo-hipster/cmd/slackware-sync/main.go~"
	Savings of 0.005681mb
	wrote "hash-map.json"

With the `-H` flag, as duplicate files are found (files with matching checksum)
are encountered, hardlink it to the duplicate file.


## next-note

Simple date formating for notes

### Install

	go get github.com/vbatts/freezing-octo-hipster/cmd/next-note

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

	go get github.com/vbatts/freezing-octo-hipster/cmd/find-todos

