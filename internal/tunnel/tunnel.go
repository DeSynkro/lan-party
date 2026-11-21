package tunnel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

// maxControlMessage bounds one JSON object on the control stream. A hello is
// tiny; a full service list is a few kilobytes.
const maxControlMessage = 64 * 1024

// Conn is one authenticated QUIC connection. The control stream is already
// past its hello/welcome exchange when a Conn is returned.
type Conn struct {
	qc         *quic.Conn
	control    *quic.Stream
	reader     *bufio.Reader
	enc        *json.Encoder
	transport  *quic.Transport
	packetConn net.PacketConn
	writeMu    sync.Mutex
	datagram   sync.Mutex
	messages   chan ControlMessage
	ctx        context.Context
	cancel     context.CancelFunc
	close      sync.Once
	cleanup    sync.Once
	onClose    func()

	// MemberID, Services and RelayToken are the values agreed during the
	// handshake. They are written once before the Conn is handed to the caller.
	MemberID   string
	Services   []ServiceInfo
	RelayToken string
}

func newConn(qc *quic.Conn, control *quic.Stream, reader *bufio.Reader, enc *json.Encoder, onClose func()) *Conn {
	ctx, cancel := context.WithCancel(qc.Context())
	return &Conn{
		qc:       qc,
		control:  control,
		reader:   reader,
		enc:      enc,
		messages: make(chan ControlMessage, 16),
		ctx:      ctx,
		cancel:   cancel,
		onClose:  onClose,
	}
}

// start begins delivering control messages. Callers set MemberID, Services,
// transport and onClose before starting so the first close cannot race.
func (c *Conn) start() {
	go c.readLoop()
}

func (c *Conn) readLoop() {
	defer close(c.messages)
	defer c.cleanupOnce()
	for {
		msg, err := readControl(c.reader)
		if err != nil {
			return
		}
		select {
		case c.messages <- msg:
		case <-c.ctx.Done():
			return
		}
	}
}

func readControl(r *bufio.Reader) (ControlMessage, error) {
	var buf bytes.Buffer
	for {
		chunk, err := r.ReadSlice('\n')
		if buf.Len()+len(chunk) > maxControlMessage {
			return ControlMessage{}, errors.New("control message is too large")
		}
		buf.Write(chunk)
		switch {
		case err == bufio.ErrBufferFull:
			continue
		case err != nil:
			return ControlMessage{}, err
		default:
			var msg ControlMessage
			if err := json.Unmarshal(buf.Bytes(), &msg); err != nil {
				return ControlMessage{}, err
			}
			return msg, nil
		}
	}
}

// Send writes one control message. The control stream is shared, so writes
// are serialized.
func (c *Conn) Send(msg ControlMessage) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := writeControl(c.enc, c.control, msg); err != nil {
		return fmt.Errorf("send control message: %w", err)
	}
	return nil
}

func writeControl(enc *json.Encoder, stream *quic.Stream, msg ControlMessage) error {
	if err := stream.SetWriteDeadline(time.Now().Add(controlWriteTimeout)); err != nil {
		return err
	}
	defer func() { _ = stream.SetWriteDeadline(time.Time{}) }()
	return enc.Encode(msg)
}

// Messages delivers control messages from the peer until the connection ends.
func (c *Conn) Messages() <-chan ControlMessage { return c.messages }

// Context is cancelled when the connection closes.
func (c *Conn) Context() context.Context { return c.ctx }

// RemoteAddr is the address the peer connected from, as the host sees it.
func (c *Conn) RemoteAddr() net.Addr { return c.qc.RemoteAddr() }

// RTT is the transport's current round-trip estimate.
func (c *Conn) RTT() time.Duration {
	stats := c.qc.ConnectionStats()
	if stats.SmoothedRTT > 0 {
		return stats.SmoothedRTT
	}
	return stats.LatestRTT
}

// OpenData opens a stream for one published port and writes its header.
func (c *Conn) OpenData(ctx context.Context, header DataHeader) (*DataStream, error) {
	if err := header.validate(); err != nil {
		return nil, err
	}
	stream, err := c.qc.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("open data stream: %w", err)
	}
	if err := writeDataHeader(stream, header); err != nil {
		stream.CancelWrite(0)
		return nil, err
	}
	return &DataStream{Stream: stream, Header: header}, nil
}

// AcceptData waits for the next data stream and reads its header.
func (c *Conn) AcceptData(ctx context.Context) (*DataStream, error) {
	stream, err := c.qc.AcceptStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("accept data stream: %w", err)
	}
	header, err := readDataHeader(stream)
	if err != nil {
		stream.CancelRead(0)
		stream.CancelWrite(0)
		return nil, err
	}
	return &DataStream{Stream: stream, Header: header}, nil
}

// SendDatagram sends one unreliable payload. The forward layer adds its flow
// id before calling this.
func (c *Conn) SendDatagram(p []byte) error {
	if !c.qc.ConnectionState().SupportsDatagrams.Remote {
		return errors.New("the peer does not accept datagrams")
	}
	c.datagram.Lock()
	defer c.datagram.Unlock()
	if err := c.qc.SendDatagram(p); err != nil {
		return fmt.Errorf("send datagram: %w", err)
	}
	return nil
}

// ReceiveDatagram waits for one datagram or for the context to end.
func (c *Conn) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	p, err := c.qc.ReceiveDatagram(ctx)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Close ends the connection and releases its stream.
func (c *Conn) Close() error {
	var err error
	c.close.Do(func() {
		c.cancel()
		err = c.qc.CloseWithError(0, "closed")
		if c.transport != nil {
			_ = c.transport.Close()
		}
		if c.packetConn != nil {
			_ = c.packetConn.Close()
		}
		c.cleanupOnce()
	})
	return err
}

func (c *Conn) cleanupOnce() {
	c.cleanup.Do(func() {
		if c.onClose != nil {
			c.onClose()
		}
	})
}

// DataStream is the friend's or host's end of one forwarded connection.
type DataStream struct {
	*quic.Stream
	Header DataHeader
}

// CloseWrite half-closes the stream so the other side can finish its reply.
func (s *DataStream) CloseWrite() error { return s.Stream.Close() }

func (h DataHeader) validate() error {
	if len(h.ServiceID) == 0 || len(h.ServiceID) > 255 {
		return errors.New("the data header has a bad service id")
	}
	if h.Port < 1 || h.Port > 65535 {
		return errors.New("the data header has a bad port")
	}
	return nil
}

func writeDataHeader(w io.Writer, h DataHeader) error {
	if err := h.validate(); err != nil {
		return err
	}
	buf := make([]byte, 1+len(h.ServiceID)+2)
	buf[0] = byte(len(h.ServiceID))
	copy(buf[1:], h.ServiceID)
	binary.BigEndian.PutUint16(buf[1+len(h.ServiceID):], uint16(h.Port))
	return writeAll(w, buf)
}

func readDataHeader(r io.Reader) (DataHeader, error) {
	var size [1]byte
	if _, err := io.ReadFull(r, size[:]); err != nil {
		return DataHeader{}, fmt.Errorf("read data header: %w", err)
	}
	if size[0] == 0 {
		return DataHeader{}, errors.New("the data header has an empty service id")
	}
	id := make([]byte, int(size[0]))
	if _, err := io.ReadFull(r, id); err != nil {
		return DataHeader{}, fmt.Errorf("read data header: %w", err)
	}
	var port [2]byte
	if _, err := io.ReadFull(r, port[:]); err != nil {
		return DataHeader{}, fmt.Errorf("read data header: %w", err)
	}
	return DataHeader{ServiceID: string(id), Port: int(binary.BigEndian.Uint16(port[:]))}, nil
}

func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
