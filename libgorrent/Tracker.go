package libgorrent

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	bencode "github.com/jackpal/bencode-go"
)

// Protocol TODO
type Protocol int

// TODO
const (
	HTTP Protocol = iota
	UDP
)

// TrackerStatus TODO
type TrackerStatus int

// TODO
const (
	NotConnected TrackerStatus = iota
	Connected
	Error
)

// Tracker TODO
type Tracker struct {
	Protocol  Protocol
	URL       string
	LastError string
	Status    TrackerStatus

	// Privates
	torrent   *Torrent
	trackerID []byte
	interval  int64
}

// HTTPTrackerResponse TODO
type HTTPTrackerResponse struct {
	Interval int64  `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

// SetTorrent Funcion que setea el torrent en el tracker. Esta funcion existe para no crear una recursividad en gob
func (tr *Tracker) SetTorrent(t *Torrent) {
	tr.torrent = t
}

// Init TODO
func (tr *Tracker) Init() error {
	tr.Status = NotConnected
	tr.interval = 10

	if strings.HasPrefix(tr.URL, "http") {
		tr.Protocol = HTTP
	} else if strings.HasPrefix(tr.URL, "udp") {
		tr.Protocol = UDP
	} else {
		tr.LastError = "Protocol not supported"
		tr.Status = Error
		return errors.New("Protocol not supported")
	}

	return nil
}

// ResumeFromFile TODO
func (tr *Tracker) ResumeFromFile() error {
	return nil
}

// Start GoRoutine TODO
func (tr *Tracker) Start() {
	for {
		if tr.Status == NotConnected {
			switch tr.Protocol {
			case HTTP:
				if err := tr.connectHTTP(); err != nil {
					log.Println(err.Error())
					tr.LastError = err.Error()
					tr.Status = Error
				}

			case UDP:
			default:
				log.Println("Protocol not supported " + tr.URL)
				tr.LastError = "Protocol not supported " + tr.URL
				tr.Status = Error
				return
			}
		}

		if tr.interval <= 0 {
			tr.interval = 10
		}

		time.Sleep(time.Duration(tr.interval) * time.Second)
	}
}

func (tr *Tracker) connectHTTP() error {
	req, err := http.NewRequest("GET", tr.URL, nil)
	if err != nil {
		return errors.New("Could not create request to " + tr.URL + ": " + err.Error())
	}

	q := req.URL.Query()
	q.Add("info_hash", string(tr.torrent.File.InfoHash))
	q.Add("peer_id", string(tr.torrent.session.peerID))
	q.Add("port", strconv.FormatInt(int64(tr.torrent.session.port), 10))
	q.Add("uploaded", strconv.FormatInt(tr.torrent.Uploaded, 10))
	q.Add("downloaded", strconv.FormatInt(tr.torrent.Downloaded, 10))
	q.Add("left", strconv.FormatInt(tr.torrent.Left, 10))
	q.Add("compact", "1")
	if tr.trackerID != nil {
		q.Add("trackerid", string(tr.trackerID))
	}
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("Errored when sending request to the server: " + err.Error())
	}

	defer resp.Body.Close()

	// Parseo la respuesta
	res := &HTTPTrackerResponse{}
	err = bencode.Unmarshal(resp.Body, res)
	if err != nil {
		return errors.New("Cannot decode tracker response. " + err.Error())
	}

	for i := 0; i < len(res.Peers)/6; i++ {
		alldata := res.Peers[i*6 : (i+1)*6]

		p := &Peer{
			IP:         net.IPv4(alldata[0], alldata[1], alldata[2], alldata[3]),
			Port:       binary.BigEndian.Uint16([]byte(alldata[4:])),
			Choked:     true,
			Interested: false,
		}

		if p.Port <= 0 {
			continue
		}

		p.Init()
		tr.torrent.addPeer(p)
	}

	tr.interval = res.Interval
	tr.Status = Connected
	return nil
}
