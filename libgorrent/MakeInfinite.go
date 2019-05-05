package libgorrent

func makeInfinite() (chan<- *Peer, <-chan *Peer) {
	in := make(chan *Peer)
	out := make(chan *Peer)
	go func() {
		var inQueue []*Peer
		outCh := func() chan *Peer {
			if len(inQueue) == 0 {
				return nil
			}
			return out
		}
		curVal := func() *Peer {
			if len(inQueue) == 0 {
				return nil
			}
			return inQueue[0]
		}
		for len(inQueue) > 0 || in != nil {
			select {
			case v, ok := <-in:
				if !ok {
					in = nil
				} else {
					inQueue = append(inQueue, v)
				}
			case outCh() <- curVal():
				inQueue = inQueue[1:]
			}
		}
		close(out)
	}()
	return in, out
}
