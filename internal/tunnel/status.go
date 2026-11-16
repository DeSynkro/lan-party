package tunnel

import (
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// PeerStatus is what WireGuard knows about one peer right now.
type PeerStatus struct {
	PublicKey     wgtypes.Key
	LastHandshake time.Time
	RxBytes       uint64
	TxBytes       uint64
}

// Peers reports the current state of every configured peer. A zero handshake
// time means the peer has never completed one.
func (d *Device) Peers() ([]PeerStatus, error) {
	out, err := d.wg.IpcGet()
	if err != nil {
		return nil, err
	}
	return parseStatus(out), nil
}

func parseStatus(uapi string) []PeerStatus {
	var peers []PeerStatus

	for _, line := range strings.Split(uapi, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		switch key {
		case "public_key":
			// The UAPI speaks hex, not the base64 that wgtypes uses.
			raw, err := hex.DecodeString(value)
			if err != nil || len(raw) != len(wgtypes.Key{}) {
				continue
			}
			var parsed wgtypes.Key
			copy(parsed[:], raw)
			peers = append(peers, PeerStatus{PublicKey: parsed})
		case "last_handshake_time_sec":
			if len(peers) == 0 {
				continue
			}
			seconds, err := strconv.ParseInt(value, 10, 64)
			if err != nil || seconds == 0 {
				continue
			}
			peers[len(peers)-1].LastHandshake = time.Unix(seconds, 0)
		case "rx_bytes":
			if len(peers) == 0 {
				continue
			}
			if n, err := strconv.ParseUint(value, 10, 64); err == nil {
				peers[len(peers)-1].RxBytes = n
			}
		case "tx_bytes":
			if len(peers) == 0 {
				continue
			}
			if n, err := strconv.ParseUint(value, 10, 64); err == nil {
				peers[len(peers)-1].TxBytes = n
			}
		}
	}

	return peers
}
