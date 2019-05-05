package libgorrent

import (
	"log"
	"sort"
	"sync"
)

// StatusEnum TODO
type StatusEnum int

const (
	// Stopped Must be sent to the tracker if the client is shutting down gracefully
	Stopped StatusEnum = iota

	// Started The first request to the tracker must include the event key with this value.
	Started

	// Completed Must be sent to the tracker when the download completes.
	// However, must not be sent if the download was already 100% complete when the client started.
	Completed
)

// BitmapFlags TODO
type BitmapFlags int8

// TODO
const (
	FlagNone      BitmapFlags = 0
	FlagRequested             = 1
	FlagCompleted             = 2
)

// PieceMap TODO
type PieceMap struct {
	Flag         BitmapFlags
	Availability int8
}

// Torrent TODO
type Torrent struct {
	File       *TorrentFile
	Location   string
	Downloaded int64
	Uploaded   int64
	Left       int64
	Status     StatusEnum
	Trackers   []*Tracker
	Peers      []*Peer
	Bitmap     []PieceMap
	BitmapChan chan int64

	//Privates
	session       *Session
	mutexPeers    sync.RWMutex
	peersAvailIn  chan<- *Peer
	peersAvailOut <-chan *Peer
	// peersConnected chan interface{}
}

// ByStatus implements sort.Interface for []*Peer based on the PeerStatus field.
type ByStatus []*Peer

func (a ByStatus) Len() int      { return len(a) }
func (a ByStatus) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByStatus) Less(i, j int) bool {
	if a[i].PeerStatus == a[j].PeerStatus {
		return string(a[i].String()) < string(a[j].String())
	}
	return a[i].PeerStatus > a[j].PeerStatus
}

// Init TODO
func (t *Torrent) Init() error {
	t.peersAvailIn, t.peersAvailOut = makeInfinite()
	// t.peersConnected = make(chan interface{}, 10000)

	for _, tracker := range t.File.AnnounceList {
		tr := &Tracker{
			URL: tracker,
		}

		// Ignoro los errores
		tr.SetTorrent(t)
		tr.Init()

		t.Trackers = append(t.Trackers, tr)
	}

	t.Bitmap = make([]PieceMap, len(t.File.Info.Pieces))
	t.BitmapChan = make(chan int64)
	t.Status = Stopped

	return nil
}

// SetSession TODO
func (t *Torrent) SetSession(session *Session) {
	t.session = session
}

// ResumeFromFile TODO
func (t *Torrent) ResumeFromFile() error {
	for _, tracker := range t.Trackers {
		tracker.SetTorrent(t)
		if err := tracker.ResumeFromFile(); err != nil {
			return err
		}
	}
	for _, peer := range t.Peers {
		peer.SetTorrent(t)
		if err := peer.ResumeFromFile(); err != nil {
			return err
		}
	}
	if t.Status == Started {
		t.Start()
	}
	return nil
}

// Start TODO
func (t *Torrent) Start() {
	t.Status = Started

	// Me conecto a los trackers
	for _, tr := range t.Trackers {
		go tr.Start()
	}
	// Inicializo el dispatcher de pares

	var wg sync.WaitGroup
	const numDigesters = 10
	wg.Add(numDigesters)
	for i := 0; i < numDigesters; i++ {
		go func() {
			t.startPeers(t.peersAvailOut)
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(t.peersAvailIn)
	}()

}

// Debug TODO
func (t *Torrent) Debug() {
	log.Printf("  |  Name: %s\n", t.File.Info.Name)
	log.Printf("    |  Files:\n")
	for _, file := range t.File.GetFiles() {
		log.Printf("      |  Fname: %s\n", file.Path)
	}
	log.Printf("    | Peers: %d\n", len(t.Peers))
	sort.Sort(ByStatus(t.Peers))
	for _, peer := range t.Peers {
		log.Printf("      |  Peer: %21s %d %s\n", peer, peer.PeerStatus, peer.ErrorReason)
	}
	log.Printf("    |  Perc: %f%%\n", float64(t.Downloaded)/float64(t.Left+t.Downloaded))
}

func (t *Torrent) startPeers(peers <-chan *Peer) {
	for p := range peers {
		if p != nil {
			p.using = true
			p.Connect()
		}
	}
}

func (t *Torrent) addPeer(p *Peer) {
	for _, x := range t.Peers {
		if p.IP.Equal(x.IP) && p.Port == x.Port {
			return
		}
	}
	t.mutexPeers.Lock()
	defer t.mutexPeers.Unlock()
	p.SetTorrent(t)
	t.Peers = append(t.Peers, p)
	t.peersAvailIn <- p
}

func (t *Torrent) getAPeer() *Peer {
	t.mutexPeers.RLock()
	defer t.mutexPeers.RUnlock()
	for _, x := range t.Peers {
		if x.PeerStatus != PeerError && !x.using {
			return x
		}
	}
	return nil
}

func (t *Torrent) connectedPeers() int {
	t.mutexPeers.RLock()
	defer t.mutexPeers.RUnlock()
	ammt := 0
	for _, x := range t.Peers {
		if x.using {
			ammt = ammt + 1
		}
	}
	return ammt
}
