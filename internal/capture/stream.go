package capture

import (
	"bufio"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"

	"github.com/mickamy/trc/internal/model"
)

// h2cPreface is the HTTP/2 client connection preface.
const h2cPreface = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"

const h2cPrefaceLen = len(h2cPreface)

// httpMethods lists HTTP method prefixes for direction detection.
var httpMethods = []string{
	"GET ", "POST ", "PUT ", "DELETE ", "PATCH ",
	"HEAD ", "OPTIONS ", "CONNECT ", "TRACE ",
}

// h1Pending holds a parsed HTTP/1.1 request waiting to be paired with its response.
type h1Pending struct {
	method  string
	path    string
	traceID string
	start   time.Time
}

// connState holds shared state between two directions of a TCP connection.
type connState struct {
	clientIP string
	serverIP string

	// HTTP/1.1: request queue for pairing with responses.
	reqCh chan h1Pending

	// HTTP/2: shared parser accessed by both directions concurrently.
	h2      *http2Parser
	h2Once  sync.Once
	h2Ready chan struct{} // closed when h2 parser is created
}

func newConnState(clientIP, serverIP string) *connState {
	return &connState{
		clientIP: clientIP,
		serverIP: serverIP,
		reqCh:    make(chan h1Pending, 64), //nolint:mnd // reasonable pipeline depth
		h2Ready:  make(chan struct{}),
	}
}

// connKey identifies a TCP connection direction by its 4-tuple.
type connKey struct {
	net, transport gopacket.Flow
}

// reverse returns the connKey for the opposite direction.
func (k connKey) reverse() connKey {
	return connKey{net: k.net.Reverse(), transport: k.transport.Reverse()}
}

// streamFactory implements tcpassembly.StreamFactory.
// It creates a new stream parser for each TCP direction and pairs the two
// directions of a connection for request/response correlation.
type streamFactory struct {
	emit func(model.Record)
	mu   sync.Mutex
	// peers maps connection keys to shared connection state.
	peers map[connKey]*connState
}

func newStreamFactory(emit func(model.Record)) *streamFactory {
	return &streamFactory{
		emit:  emit,
		peers: make(map[connKey]*connState),
	}
}

// New implements tcpassembly.StreamFactory.
func (f *streamFactory) New(
	net, transport gopacket.Flow,
) tcpassembly.Stream {
	key := connKey{net: net, transport: transport}
	srcIP := net.Src().String()
	dstIP := net.Dst().String()

	rs := tcpreader.NewReaderStream()
	go f.handleStream(key, srcIP, dstIP, &rs)
	return &rs
}

// getOrCreateConn returns the shared connection state for a connection,
// looking up both the key and its reverse.
func (f *streamFactory) getOrCreateConn(
	key connKey, clientIP, serverIP string,
) *connState {
	f.mu.Lock()
	defer f.mu.Unlock()

	if cs, ok := f.peers[key]; ok {
		return cs
	}
	rev := key.reverse()
	if cs, ok := f.peers[rev]; ok {
		f.peers[key] = cs
		return cs
	}

	cs := newConnState(clientIP, serverIP)
	f.peers[key] = cs
	return cs
}

// handleStream reads the stream and delegates to the appropriate
// protocol parser based on the initial bytes.
func (f *streamFactory) handleStream(
	key connKey, srcIP, dstIP string, r io.Reader,
) {
	br := bufio.NewReaderSize(r, 4096) //nolint:mnd // read buffer size

	peek, err := br.Peek(h2cPrefaceLen)
	if err != nil {
		// Less data available. Try a smaller peek.
		if n := br.Buffered(); n > 0 {
			peek, _ = br.Peek(n)
		} else if _, peekErr := br.Peek(1); peekErr == nil {
			peek, _ = br.Peek(br.Buffered())
		}
		if len(peek) == 0 {
			_, _ = io.Copy(io.Discard, br)
			return
		}
	}

	s := string(peek)

	// HTTP/2 client direction: starts with the h2c connection preface.
	if len(peek) >= h2cPrefaceLen && s[:h2cPrefaceLen] == h2cPreface {
		if _, discardErr := br.Discard(h2cPrefaceLen); discardErr != nil {
			return
		}
		cs := f.getOrCreateConn(key, srcIP, dstIP)
		cs.h2Once.Do(func() {
			cs.clientIP = srcIP
			cs.serverIP = dstIP
			cs.h2 = newHTTP2Parser(srcIP, dstIP, f.emit)
			close(cs.h2Ready)
		})
		cs.h2.runDirection(br, dirClient)
		return
	}

	// HTTP/1.1 response direction: starts with "HTTP/".
	if strings.HasPrefix(s, "HTTP/") {
		// For response streams, srcIP is the server and dstIP is the client.
		cs := f.getOrCreateConn(key, dstIP, srcIP)
		runHTTP1Response(br, cs, cs.clientIP, cs.serverIP, f.emit)
		return
	}

	// HTTP/1.1 request direction: starts with an HTTP method.
	if looksLikeHTTPMethod(s) {
		cs := f.getOrCreateConn(key, srcIP, dstIP)
		runHTTP1Request(br, cs)
		return
	}

	// Possibly HTTP/2 server direction — binary frames without h2c preface.
	// Check if the peer has already been identified as HTTP/2.
	cs := f.getOrCreateConn(key, dstIP, srcIP)
	select {
	case <-cs.h2Ready:
		cs.h2.runDirection(br, dirServer)
		return
	default:
	}

	// Wait briefly for the client direction to identify the protocol.
	select {
	case <-cs.h2Ready:
		cs.h2.runDirection(br, dirServer)
	case <-time.After(500 * time.Millisecond): //nolint:mnd // peer detection grace period
		_, _ = io.Copy(io.Discard, br)
	}
}

func looksLikeHTTPMethod(s string) bool {
	for _, m := range httpMethods {
		if strings.HasPrefix(s, m) {
			return true
		}
	}
	return false
}
