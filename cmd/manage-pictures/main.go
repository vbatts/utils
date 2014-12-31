package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
)

var (
	flPicturePath = flag.String("p", filepath.Join(os.Getenv("HOME"), "Pictures"), "Base path for pictures to be stored")
	flPathFormat  = flag.String("f", "2006/01", "formatting for picture path")
	flMove        = flag.Bool("m", false, "actually move the pictures to the picture store")
	flVerbose     = flag.Bool("V", false, "more information")
)

func main() {
	flag.Parse()

	var paths []string
	if flag.NArg() == 0 {
		paths = []string{"."}
	} else {
		paths = flag.Args()
	}

	exif.RegisterParsers(mknote.All...)
	ps := PictureStore{
		BasePath: *flPicturePath,
		Format:   *flPathFormat,
	}

	for _, path := range paths {
		err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			if *flMove {
				if err := ps.Move(path, *flVerbose); err != nil {
					log.Printf("WARN: %q failed with %s", path, err)
				}
			} else {
				if err := ps.Copy(path, *flVerbose); err != nil {
					log.Printf("WARN: %q failed with %s", path, err)
				}
			}
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	}
}

type PictureStore struct {
	BasePath string
	Format   string
}

func (ps PictureStore) Dest(path string) (string, error) {
	fh, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer fh.Close()

	x, err := exif.Decode(fh)
	if err != nil {
		return "", err
	}
	tm, _ := x.DateTime()
	return fmt.Sprintf("%s/%s/%s", ps.BasePath, tm.Format(ps.Format), filepath.Base(path)), nil
}

func (ps PictureStore) Copy(path string, verbose bool) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	destPath, err := ps.Dest(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	fh, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	destFh, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFh.Close()

	_, err = io.Copy(destFh, fh)
	if err != nil {
		return err
	}
	if verbose {
		fmt.Printf("cp %q %q\n", path, destPath)
	}
	err = os.Chmod(destPath, stat.Mode())
	if err != nil {
		return err
	}
	return nil
}

func (ps PictureStore) Move(path string, verbose bool) error {
	if err := ps.Copy(path, verbose); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	if verbose {
		fmt.Printf("rm %q\n", path)
	}
	return nil
}
