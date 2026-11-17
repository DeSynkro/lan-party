// Package preset ships the port table for games and services people actually
// host. Anyone can add their own service instead.
package preset

// Preset is one shipped service before it has a LAN target.
type Preset struct {
	ID    string
	Name  string
	Proto string
	Ports []int
	Hint  string
}

var builtin = []Preset{
	{ID: "minecraft-java", Name: "Minecraft: Java Edition", Proto: "tcp", Ports: []int{25565}, Hint: "Direct Connect in Minecraft"},
	{ID: "minecraft-bedrock", Name: "Minecraft: Bedrock", Proto: "udp", Ports: []int{19132}, Hint: "Add server in Minecraft"},
	{ID: "terraria", Name: "Terraria", Proto: "tcp", Ports: []int{7777}, Hint: "Join via IP in Terraria"},
	{ID: "valheim", Name: "Valheim", Proto: "udp", Ports: []int{2456, 2457, 2458}, Hint: "Join Game, then add address"},
	{ID: "factorio", Name: "Factorio", Proto: "udp", Ports: []int{34197}, Hint: "Connect to address"},
	{ID: "palworld", Name: "Palworld", Proto: "udp", Ports: []int{8211}, Hint: "Join with address"},
	{ID: "stardew", Name: "Stardew Valley", Proto: "tcp", Ports: []int{24642}, Hint: "Join LAN, then enter address"},
	{ID: "zomboid", Name: "Project Zomboid", Proto: "udp", Ports: []int{16261, 16262}, Hint: "Add server by address"},
	{ID: "sevendays", Name: "7 Days to Die", Proto: "both", Ports: []int{26900, 26901, 26902, 26903}, Hint: "Add server by address"},
	{ID: "rust", Name: "Rust", Proto: "udp", Ports: []int{28015}, Hint: "Client connect, then address"},
	{ID: "source", Name: "Source engine (CS2, GMod, TF2)", Proto: "both", Ports: []int{27015}, Hint: "Use the console: connect address"},
	{ID: "ark", Name: "ARK: Survival", Proto: "udp", Ports: []int{7777, 7778}, Hint: "Join with address"},
	{ID: "dst", Name: "Don't Starve Together", Proto: "udp", Ports: []int{10999}, Hint: "Join with address"},
	{ID: "jellyfin", Name: "Jellyfin", Proto: "tcp", Ports: []int{8096}, Hint: "Open the address in a browser"},
	{ID: "plex", Name: "Plex", Proto: "tcp", Ports: []int{32400}, Hint: "Open the address in a browser"},
}

// All returns a copy, so callers cannot edit the shipped list.
func All() []Preset {
	out := make([]Preset, len(builtin))
	for i, p := range builtin {
		out[i] = Preset{
			ID:    p.ID,
			Name:  p.Name,
			Proto: p.Proto,
			Ports: append([]int(nil), p.Ports...),
			Hint:  p.Hint,
		}
	}
	return out
}

// ByID looks up a shipped preset.
func ByID(id string) (Preset, bool) {
	for _, p := range builtin {
		if p.ID == id {
			return Preset{
				ID:    p.ID,
				Name:  p.Name,
				Proto: p.Proto,
				Ports: append([]int(nil), p.Ports...),
				Hint:  p.Hint,
			}, true
		}
	}
	return Preset{}, false
}
