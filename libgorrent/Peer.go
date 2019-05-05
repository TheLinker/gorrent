package libgorrent

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	restruct "gopkg.in/restruct.v1"
)

// Handshake TODO
type Handshake struct {
	Pstrlen  byte
	Pstr     string
	Reserved [8]byte
	InfoHash [20]byte
	PeerID   [20]byte
}

// Peer TODO
type Peer struct {
	IP          net.IP
	Port        uint16
	Choked      bool
	Interested  bool
	PeerStatus  PeerStatus
	ErrorReason string
	PeerID      [20]byte

	// Privates
	torrent *Torrent
	using   bool
}

// PeerStatus TODO
type PeerStatus int

// TODO
const (
	PeerDisconnected PeerStatus = iota
	PeerConnected
	PeerError
)

// SetTorrent Funcion que setea el torrent en el tracker. Esta funcion existe para no crear una recursividad en gob
func (p *Peer) SetTorrent(t *Torrent) {
	p.torrent = t
}

// Init TODO
func (p *Peer) Init() error {
	p.using = false
	return nil
}

// ResumeFromFile TODO
func (p *Peer) ResumeFromFile() error {
	return nil
}

// String TODO
func (p *Peer) String() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

// ConnectAddr TODO
func (p *Peer) ConnectAddr() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

// Open TODO
func (p *Peer) Open() (conn net.Conn, err error) {
	log.Printf("%21s Dial\n", p.ConnectAddr())
	conn, err = net.DialTimeout("tcp", p.ConnectAddr(), 5*time.Second)
	if err != nil {
		err = errors.New("Dialing " + p.String() + " failed: " + err.Error())
	}
	return
}

// nolint
// Connect TODO
func (p *Peer) Connect() {
	p.using = true
	defer func() {
		p.using = false
	}()

	// Open connection to peer
	conn, err := p.Open()
	if err != nil {
		p.PeerStatus = PeerError
		p.ErrorReason = err.Error()
		return
	}
	defer conn.Close()

	r, w := bufio.NewReader(conn), bufio.NewWriter(conn)
	// r, w := conn.(io.Reader), conn.(io.Writer)

	err = p.doHandshake(r, w)
	if err != nil {
		log.Println("Errors during HandShake: ", p, err.Error())
		p.PeerStatus = PeerError
		p.ErrorReason = err.Error()
		return
	}
	p.PeerStatus = PeerConnected

	for {
		if p.PeerStatus == PeerError {
			return
		}
		// Leo 4 bytes BigEndian
		var l int32

		err = conn.SetDeadline(time.Now().Add(2 * time.Second))
		if err != nil {
			continue
		}

		err := binary.Read(r, binary.BigEndian, &l)

		to, ok := err.(interface{ Timeout() bool })
		if ok && to.Timeout() {
			// No me importan los timeouts
			continue
		}

		if !p.checkConnStatus(err) {
			return
		}

		if l > 1<<15 {
			p.PeerStatus = PeerError
			return
		}

		if l < 0 {
			p.PeerStatus = PeerError
			return
		}

		if l == 0 {
			// Keep Alive
			// Niice
			continue
		}

		var pkg_id byte
		err = binary.Read(r, binary.BigEndian, &pkg_id)
		if !p.checkConnStatus(err) {
			return
		}

		switch pkg_id {
		case 0:
			log.Printf("%21s- Choke\n", p)
		case 1:
			log.Printf("%21s- UnChoke\n", p)
		case 2:
			log.Printf("%21s- Interested\n", p)
		case 3:
			log.Printf("%21s- Not Interested\n", p)
		case 4:
			log.Printf("%21s- Have\n", p)
		case 5:
			log.Printf("%21s- Bitfield\n", p)
		case 6:
			log.Printf("%21s- Request\n", p)
		}

		data := make([]byte, r.Buffered())
		_, err = io.ReadFull(r, data)
		if !p.checkConnStatus(err) {
			return
		}
		log.Printf("%21s- Recv. % X", p.String(), data)
	}

}

func (p *Peer) doHandshake(r *bufio.Reader, w *bufio.Writer) error {
	c := Handshake{
		Pstrlen: 19,
		Pstr:    "BitTorrent protocol",
	}

	copy(c.InfoHash[:], p.torrent.File.InfoHash[:])
	copy(c.PeerID[:], p.torrent.session.peerID[:])

	d, err := restruct.Pack(binary.BigEndian, &c)
	if err != nil {
		return err
	}

	_, err = w.Write(d)
	if err != nil {
		return err
	}

	err = w.Flush()
	if err != nil {
		return err
	}

	ret := make([]byte, 49+19)
	n, err := io.ReadFull(r, ret)
	if err != nil {
		to, ok := err.(interface{ Timeout() bool })
		if ok {
			log.Printf("%+v\n", to.Timeout())
		}
		log.Printf("%d %d %s\n", 4, n, err.Error())
		return err
	}

	err = restruct.Unpack(ret, binary.BigEndian, &c)
	if err != nil {
		return err
	}

	copy(p.PeerID[:], c.PeerID[:20])

	return nil
}

func (p *Peer) checkConnStatus(err error) bool {
	if err != nil {
		if err == io.EOF {
			p.PeerStatus = PeerDisconnected
		} else {
			p.PeerStatus = PeerError
		}

		return false
	}

	return true
}
