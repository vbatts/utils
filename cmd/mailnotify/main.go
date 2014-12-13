package main

import (
	"flag"
	"fmt"
	"log"
	"net/mail"
	"os"
	"path/filepath"
	"time"

	"github.com/luksen/maildir"
	notify "github.com/mqu/go-notify"
	fsnotify "gopkg.in/fsnotify.v1"
)

var (
	AppName = os.Args[0]
	Linger  = 2
)

func init() {
	notify.Init(AppName)
}

func main() {
	flag.Parse()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				// 2014/12/12 21:18:08 event: "/home/vbatts/Maildir/INBOX/new/1418437088_0.3432.valse.usersys.redhat.com,U=478874,FMD5=7e33429f656f1e6e9d79b29c3f82c57e:2,": CREATE
				log.Println("event:", event)
				if event.Op == fsnotify.Create {
					func() {
						//Notify("new msg", filepath.Dir(filepath.Dir(event.Name)))
						fh, err := os.Open(event.Name)
						if err != nil {
							log.Println(err)
							return
						}
						msg, err := mail.ReadMessage(fh)
						if err != nil {
							log.Println(err)
							return
						}
						//fmt.Printf("%#v\n", msg.Header)
						Notify(filepath.Base(filepath.Dir(filepath.Dir(event.Name))), fmt.Sprintf("%s; From %s", msg.Header["Subject"][0], msg.Header["From"][0]))
					}()
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	go func() {
		if err := Notify(AppName, "running ..."); err != nil {
			log.Println(err)
		}
	}()

	for _, arg := range flag.Args() {
		dirs, err := GetMailDirs(arg)
		if err != nil {
			log.Fatal(err)
		}
		for _, dirPath := range dirs {
			if err = watcher.Add(filepath.Join(dirPath, "new")); err != nil {
				log.Fatal(err)
			}

			dir := maildir.Dir(dirPath)
			keys, err := dir.Keys()
			if err != nil {
				log.Fatal(err)
			}
			u, err := dir.UnreadKeys()
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(dir, len(keys), len(u))
		}
	}

	if flag.NArg() == 0 {
		close(done)
	}
	<-done
}

func Notify(title, message string) error {
	hello := notify.NotificationNew(title, message, "mail-message-new")
	notify.NotificationSetTimeout(hello, 0)

	if err := notify.NotificationShow(hello); err != nil && len(err.Message()) > 0 {
		return fmt.Errorf("%s", err.Message())
	}
	time.Sleep(time.Duration(Linger) * time.Second)
	if err := notify.NotificationClose(hello); err != nil && len(err.Message()) > 0 {
		return fmt.Errorf("%s", err.Message())
	}
	return nil
}

func GetMailDirs(root string) ([]string, error) {
	dirs := []string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		base := filepath.Base(path)
		if !info.IsDir() || base == "cur" || base == "new" || base == "tmp" {
			return filepath.SkipDir
		}
		if b, err := IsMaildir(path); err == nil && b {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return dirs, nil
	//return []string{root}, nil
}

func IsMaildir(path string) (bool, error) {
	curInfo, err := os.Stat(filepath.Join(path, "cur"))
	if err != nil {
		return false, err
	}
	tmpInfo, err := os.Stat(filepath.Join(path, "tmp"))
	if err != nil {
		return false, err
	}
	newInfo, err := os.Stat(filepath.Join(path, "new"))
	if err != nil {
		return false, err
	}
	return (curInfo != nil && tmpInfo != nil && newInfo != nil), nil
}
