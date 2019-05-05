package libgorrent

import (
	"bytes"
	"crypto/rand"
	"encoding/gob"
	"errors"
	"log"
	"os"
)

// Session TODO
type Session struct {
	AllTorrents []*Torrent

	// Privates
	port   int16
	peerID []byte
}

func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

// NewSession TODO
func NewSession() (*Session, error) {

	PeerID := "-GOR000-" + randStringBytesMaskImprSrcUnsafe(20-len("-GOR000-"))

	return &Session{
		peerID: []byte(PeerID),
		port:   1337, // Deberia venir de alguna config
	}, nil
}

// NewSessionFromFile TODO
func NewSessionFromFile(fname string) (*Session, error) {
	f, err := os.OpenFile(fname, os.O_RDONLY, 0755)
	if err != nil {
		log.Printf("Could not open session.\n")
		return nil, err
	}

	dec := gob.NewDecoder(f)

	sess, err := NewSession()
	if err != nil {
		log.Printf("Could not create new session.\n")
		return nil, err
	}

	err = dec.Decode(&sess)
	if err != nil {
		log.Printf("Could not decode session.\n")
		return nil, err
	}

	if err := f.Close(); err != nil {
		log.Printf("Could not persist session.\n")
		return nil, err
	}

	if err = sess.ResumeFromFile(); err != nil {
		log.Printf("Could not resume session.\n")
		return nil, err
	}

	return sess, nil
}

// Debug TODO
func (s *Session) Debug() {
	log.Printf("________________________________________________________________________\n")
	log.Printf("Session: %X\n", s.peerID)
	log.Printf("Torrents: %d\n", len(s.AllTorrents))
	for _, x := range s.AllTorrents {
		x.Debug()
	}
	log.Printf("Port: %d\n", s.port)
	log.Printf("------------------------------------------------------------------------\n")
}

// AddTorrent TODO
func (s *Session) AddTorrent(tor *TorrentFile) (*Torrent, error) {
	// Me fijo si el InfoHash no fue agregado anteriormente

	for _, torrent := range s.AllTorrents {
		if bytes.Compare(torrent.File.InfoHash, tor.InfoHash) == 0 {
			return nil, errors.New("Torrent already added")
		}
	}

	var aNewTorrent = &Torrent{
		File:       tor,
		Location:   "",
		Downloaded: 0,
		Uploaded:   0,
		Left:       tor.GetLength(),
		Status:     Stopped,
		Trackers:   make([]*Tracker, 0),
	}

	aNewTorrent.SetSession(s)
	aNewTorrent.Init()

	s.AllTorrents = append(s.AllTorrents, aNewTorrent)
	return aNewTorrent, nil
}

// Save TODO
func (s *Session) Save() error {
	var data bytes.Buffer
	enc := gob.NewEncoder(&data)
	err := enc.Encode(s)
	if err != nil {
		log.Printf("Could not encode session.\n")
		return err
	}

	f, err := os.OpenFile("session.gob", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.Printf("Could not persist session.\n")
		return err
	}
	_, err = f.Write(data.Bytes())
	if err != nil {
		log.Printf("Could not write session.\n")
		return err
	}
	if err := f.Close(); err != nil {
		log.Printf("Could not save session.\n")
		return err
	}

	return nil
}

// ResumeFromFile TODO
func (s *Session) ResumeFromFile() error {
	for _, to := range s.AllTorrents {
		to.SetSession(s)
		if err := to.ResumeFromFile(); err != nil {
			return err
		}
	}

	return nil
}
