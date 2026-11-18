package session

import (
	"fmt"
	"testing"
)

func TestHostPeerLimits(t *testing.T) {
	h := &Host{peers: map[*hostPeer]struct{}{}}

	if h.trackPeer(&hostPeer{}) {
		t.Fatal("a peer without a member id was accepted")
	}

	for i := 0; i < maxPeersPerMember; i++ {
		if !h.trackPeer(&hostPeer{memberID: "member-1"}) {
			t.Fatalf("peer %d for one member was rejected", i)
		}
	}
	if h.trackPeer(&hostPeer{memberID: "member-1"}) {
		t.Fatal("too many connections for one member were accepted")
	}

	for i := len(h.peers); i < maxHostPeers; i++ {
		if !h.trackPeer(&hostPeer{memberID: fmt.Sprintf("member-%d", i)}) {
			t.Fatalf("peer %d was rejected before the total cap", i)
		}
	}
	if h.trackPeer(&hostPeer{memberID: "one-more"}) {
		t.Fatal("the total peer cap was not enforced")
	}
}
