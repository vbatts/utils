# freezing-octo-hipster

Random utilities of vbatts' being cleaned up, and published

# Commands

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

