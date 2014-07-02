package main

import (
	"flag"
	"fmt"
	"path"
	"time"
)

var (
	flDir      = flag.String("dir", "", "Base directory for tasks")
	flDate     = flag.Bool("d", false, "print current date")
	flPrevWeek = flag.Bool("p", false, "print previous week's filename")
	flCurrWeek = flag.Bool("c", false, "print current week's filename")
)

var FileDate = "20060102"

func main() {
	flag.Parse()

	if *flDate {
    // this intentionally has no '\n'
		fmt.Printf("== %s", time.Now().Format(time.UnixDate))
		return
	}

	var monday int
	t := time.Now()
	if *flCurrWeek {
		monday = -1*int(t.Weekday()) + 1
	} else if *flPrevWeek {
		monday = -1*int(t.Weekday()) + 1 - 7
	} else {
		monday = -1*int(t.Weekday()) + 1 + 7
	}
	friday := monday + 4
	startDate := t.AddDate(0, 0, monday).Format(FileDate)
	endDate := t.AddDate(0, 0, friday).Format(FileDate)
	filename := fmt.Sprintf("Tasks-%s-%s.txt", startDate, endDate)
	fmt.Println(path.Join(*flDir, filename))
}
