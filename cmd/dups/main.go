package main

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var (
	flLoadMap  = flag.String("l", "", "load existing map from file")
	flSaveMap  = flag.String("o", "hash-map.json", "file to save map of file hashes to")
	flHardlink = flag.Bool("H", false, "hardlink the duplicate files")
	flSymlink  = flag.Bool("s", false, "symlink the duplicate files")
	flQuiet    = flag.Bool("q", false, "less output")
	nprocs     = 1
)

func init() {
	nprocs = runtime.NumCPU()
	runtime.GOMAXPROCS(nprocs)
}

func main() {
	flag.Parse()
	for _, arg := range flag.Args() {
		savings := int64(0)
		found := map[string]string{}
		if len(*flLoadMap) > 0 {
			fh, err := os.Open(*flLoadMap)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			buf, err := ioutil.ReadAll(fh)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			if err = json.Unmarshal(buf, &found); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		workers := make(chan int, nprocs)
		mu := sync.Mutex{}
		err := filepath.Walk(arg, func(path string, info os.FileInfo, err error) error {
			/*
				if err != nil {
					return err
				}
			*/
			if !info.Mode().IsRegular() {
				return nil
			}
			workers <- 1
			go func() {
				defer func() { <-workers }()

				fh, err := os.Open(path)
				if err != nil {
					fmt.Fprintln(os.Stderr, err, path)
					return
				}
				defer fh.Close()

				h := sha1.New()
				if _, err = io.Copy(h, fh); err != nil {
					fmt.Fprintln(os.Stderr, err, path)
					return
				}
				sum := fmt.Sprintf("%x", h.Sum(nil))

				// get the absolute filename
				p, err := filepath.Abs(path)
				if err != nil {
					fmt.Fprintln(os.Stderr, err, path)
					return
				}
				path = p

				mu.Lock()
				defer mu.Unlock()
				if fpath, ok := found[sum]; ok && fpath != path {
					if !(*flQuiet) {
						fmt.Printf("%q is the same content as %q\n", path, fpath)
					}
					if *flHardlink {
						if err = SafeLink(fpath, path, true); err != nil {
							fmt.Fprintln(os.Stderr, err, path)
							return
						}
						fmt.Printf("hard linked %q to %q\n", path, fpath)
					}
					if *flSymlink {
						if err = SafeLink(fpath, path, false); err != nil {
							fmt.Fprintln(os.Stderr, err, path)
							return
						}
						fmt.Printf("soft linked %q to %q\n", path, fpath)
					}
					savings += info.Size()
				} else {
					found[sum] = path
				}
			}()
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for len(workers) > 0 {
			time.Sleep(5 * time.Microsecond)
		}
		fmt.Printf("Savings of %fmb\n", float64(savings)/1024.0/1024.0)
		fh, err := os.Create(*flSaveMap)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		buf, err := json.Marshal(found)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_, err = fh.Write(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fh.Close()
		fmt.Fprintf(os.Stderr, "wrote %q\n", fh.Name())
	}
}

// SafeLink overrides newname if it already exists. If there is an error in creating the link, the transaction is rolled back
func SafeLink(oldname, newname string, hard bool) error {
	var backupName string
	// check if newname exists
	if fi, err := os.Stat(newname); err == nil && fi != nil {
		// make a random name
		buf := make([]byte, 5)
		if _, err = rand.Read(buf); err != nil {
			return err
		}
		backupName = fmt.Sprintf("%s.%x", newname, buf)
		// move newname to the random name backupName
		if err = os.Rename(newname, backupName); err != nil {
			return err
		}
	}
	if hard {
		// hardlink oldname to newname
		if err := os.Link(oldname, newname); err != nil {
			// if that failed, and there is a backupName
			if len(backupName) > 0 {
				// then move back the backup
				if err = os.Rename(backupName, newname); err != nil {
					return err
				}
			}
			return err
		}
	} else {
		// symlink
		relpath, err := filepath.Rel(filepath.Dir(newname), oldname)
		if err != nil {
			return err
		}
		if err := os.Symlink(relpath, newname); err != nil {
			// if that failed, and there is a backupName
			if len(backupName) > 0 {
				// then move back the backup
				if err = os.Rename(backupName, newname); err != nil {
					return err
				}
			}
			return err
		}
	}
	// remove the backupName
	if len(backupName) > 0 {
		os.Remove(backupName)
	}
	return nil
}
