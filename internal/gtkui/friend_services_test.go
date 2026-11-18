package gtkui

import (
	"testing"

	"github.com/desynkro/lan-party/internal/session"
)

func TestBuildFriendServiceViewsGroupsPortsAndProtocols(t *testing.T) {
	views := buildFriendServiceViews([]session.Bound{
		{ServiceID: "db", Service: "Database", Proto: "tcp", PublishedPort: 3306, Port: 3306, Address: "127.0.0.1:3306", Hint: "Connect"},
		{ServiceID: "db", Service: "Database", Proto: "udp", PublishedPort: 3306, Port: 3306, Address: "127.0.0.1:3306", Hint: "Connect"},
		{ServiceID: "db", Service: "Database", Proto: "tcp", PublishedPort: 3307, Port: 3308, Address: "127.0.0.1:3308"},
		{ServiceID: "val", Service: "Valheim", Proto: "udp", PublishedPort: 2456, Port: 2456, Address: "127.0.0.1:2456"},
	})

	if len(views) != 2 {
		t.Fatalf("views = %d, want 2", len(views))
	}
	db := views[0]
	if db.name != "Database" || protocolSetName(db.protocols) != "TCP + UDP" {
		t.Fatalf("database view = %+v", db)
	}
	if len(db.addresses) != 2 {
		t.Fatalf("database addresses = %+v", db.addresses)
	}
	if db.addresses[0].address != "127.0.0.1:3306" || protocolSetName(db.addresses[0].protocols) != "TCP + UDP" {
		t.Fatalf("first address = %+v", db.addresses[0])
	}
	if len(db.notes) != 1 || db.notes[0] != "Port 3307 was busy; this game uses 3308." {
		t.Fatalf("database notes = %v", db.notes)
	}
	if db.hint != "Connect" {
		t.Fatalf("database hint = %q", db.hint)
	}

	valheim := views[1]
	if valheim.name != "Valheim" || len(valheim.addresses) != 1 || protocolSetName(valheim.protocols) != "UDP" {
		t.Fatalf("valheim view = %+v", valheim)
	}
}

func TestBuildFriendServiceViewsDeduplicatesErrors(t *testing.T) {
	views := buildFriendServiceViews([]session.Bound{
		{ServiceID: "svc", Service: "Service", Proto: "tcp", PublishedPort: 25565, Port: 25565, Error: "address already in use"},
		{ServiceID: "svc", Service: "Service", Proto: "udp", PublishedPort: 25565, Port: 25565, Error: "address already in use"},
	})
	if len(views) != 1 || len(views[0].errors) != 1 {
		t.Fatalf("views = %+v", views)
	}
}
