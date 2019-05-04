package main

import (
	"log"
	"os"
	"time"

	"github.com/TheLinker/gorrent/libgorrent"
)

func main() {
	err := os.Remove("session.gob")
	if err != nil {
		log.Println(err.Error())
	}

	sess, err := libgorrent.NewSessionFromFile("session.gob")
	if err != nil {
		log.Println(err.Error())
		sess, _ = libgorrent.NewSession()
	}
	// sess.Debug()

	// log.Println("")

	for argc, argv := range os.Args {
		if argc == 0 {
			continue
		}

		torrentfile, err := libgorrent.LoadFromFile(argv)
		if err != nil {
			log.Println(err.Error())
			return
		}

		// torrentfile.Debug()

		torrent, err := sess.AddTorrent(torrentfile)
		if err != nil {
			log.Println(err.Error())
		} else {
			go torrent.Start()
		}

		log.Println("")
	}

	err = sess.Save()
	if err != nil {
		log.Println(err.Error())
		return
	}

	for {
		sess.Debug()
		time.Sleep(5 * time.Second)
	}

}
