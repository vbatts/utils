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
		fmt.Printf("## %s", time.Now().Format(time.UnixDate))
		return
	}

	var sunday int
	t := time.Now()
	if *flCurrWeek {
		sunday = -1 * int(t.Weekday())
	} else if *flPrevWeek {
		sunday = -1*int(t.Weekday()) - 7
	} else {
		sunday = -1*int(t.Weekday()) + 7
	}
	saturday := sunday + 6
	startDate := t.AddDate(0, 0, sunday).Format(FileDate)
	endDate := t.AddDate(0, 0, saturday).Format(FileDate)
	filename := fmt.Sprintf("Tasks-%s-%s.md", startDate, endDate)
	fmt.Println(path.Join(*flDir, filename))
}
