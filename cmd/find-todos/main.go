package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	var NoteDir string
	var err error

	if len(os.Getenv("NOTEDIR")) > 0 {
		NoteDir, err = filepath.Abs(os.Getenv("NOTEDIR"))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		NoteDir = filepath.Join(os.Getenv("HOME"), "Notes")
	}

	flNoteDir := flag.String("d", NoteDir, "directory of notes")
	flNoteFilePat := flag.String("p", "Tasks*.md", "file pattern")
	flNoteTodoPat := flag.String("s", "TODO", "search for pattern in files")
	flag.Parse()

	matches, err := filepath.Glob(filepath.Join(*flNoteDir, *flNoteFilePat))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, match := range matches {
		fh, err := os.Open(match)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		rdr := bufio.NewReader(fh)
		count := int64(0)
		for {
			line, err := rdr.ReadString('\n')
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			count++
			if strings.Contains(line, *flNoteTodoPat) {
				fmt.Printf("%s:%d\t%s\n", filepath.Base(match), count, strings.TrimRight(line, " \n"))
				trimmed := strings.TrimSpace(line)
				if strings.HasSuffix(trimmed, ":") || strings.HasSuffix(trimmed, *flNoteTodoPat) {
					i := strings.IndexRune(line, rune(trimmed[0]))
					for {
						buf, err := rdr.Peek(i + 1)
						if err == io.EOF {
							break
						}
						if err != nil {
							fmt.Println(err)
							os.Exit(1)
						}
						if strings.Count(string(buf), " ") > 0 {
							nextLine, err := rdr.ReadString('\n')
							if err == io.EOF {
								break
							}
							if err != nil {
								fmt.Println(err)
								os.Exit(1)
							}
							fmt.Printf("%s:%d\t%s\n", filepath.Base(match), count, strings.TrimRight(nextLine, " \n"))
						} else {
							break
						}
					}
				}
			}
		}
		fh.Close()
	}
}
