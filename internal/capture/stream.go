package capture

import (
	"bufio"
	"io"
	"sync"

	"github.com/google/gopacket"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"

	"github.com/mickamy/trc/internal/model"
)

// h2cPreface is the HTTP/2 client connection preface.
const h2cPreface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"

const h2cPrefaceLen = len(h2cPreface)

// streamFactory implements tcpassembly.StreamFactory.
// It creates a new stream parser for each TCP connection and detects the
// application-layer protocol (HTTP/1.1 or HTTP/2) from the initial bytes.
type streamFactory struct {
	emit func(model.Record)
	// mu protects peerMap for concurrent stream creation.
	mu sync.Mutex
	// peerMap tracks the reverse direction of each connection so that
	// request and response parsers share context.
	peerMap map[connKey]*peerState
}

// connKey identifies a TCP connection by its 4-tuple.
type connKey struct {
	net, transport gopacket.Flow
}

// reverse returns the connKey for the opposite direction.
func (k connKey) reverse() connKey {
	return connKey{net: k.net.Reverse(), transport: k.transport.Reverse()}
}

// peerState holds shared state between the two directions of a connection.
type peerState struct {
	srcIP string
	dstIP string
}

func newStreamFactory(emit func(model.Record)) *streamFactory {
	return &streamFactory{
		emit:    emit,
		peerMap: make(map[connKey]*peerState),
	}
}

// New implements tcpassembly.StreamFactory.
func (f *streamFactory) New(
	net, transport gopacket.Flow,
) tcpassembly.Stream {
	key := connKey{net: net, transport: transport}
	rs := tcpreader.NewReaderStream()

	srcIP := net.Src().String()
	dstIP := net.Dst().String()

	f.mu.Lock()
	// Check if the reverse direction already registered a peer.
	revKey := key.reverse()
	if _, ok := f.peerMap[revKey]; !ok {
		f.peerMap[key] = &peerState{srcIP: srcIP, dstIP: dstIP}
	}
	f.mu.Unlock()

	go f.handleStream(srcIP, dstIP, &rs)

	return &rs
}

// handleStream reads the stream and delegates to the appropriate protocol parser.
func (f *streamFactory) handleStream(srcIP, dstIP string, r io.Reader) {
	br := bufio.NewReaderSize(r, 4096)

	// Peek at the first bytes to detect the protocol.
	peek, err := br.Peek(h2cPrefaceLen)
	if err != nil {
		// Not enough data — try HTTP/1.1 as fallback.
		p := &http1Parser{srcIP: srcIP, dstIP: dstIP, emit: f.emit}
		p.run(br)
		return
	}

	if string(peek) == h2cPreface {
		// Consume the preface before handing off to the framer.
		if _, err := br.Discard(h2cPrefaceLen); err != nil {
			return
		}
		p := newHTTP2Parser(srcIP, dstIP, f.emit)
		p.run(br)
	} else {
		p := &http1Parser{srcIP: srcIP, dstIP: dstIP, emit: f.emit}
		p.run(br)
	}
}
