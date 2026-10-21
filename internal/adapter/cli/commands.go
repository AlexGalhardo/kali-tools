package cli

type command struct {
	path     string
	synopsis string
	needsApp bool
	flags    []string
}

var commands = []command{
	{path: "version", synopsis: "Show version and build platform"},
	{path: "doctor", synopsis: "Show capabilities and optional dependencies", needsApp: true},
	{path: "interfaces", synopsis: "List local network interfaces", needsApp: true},
	{path: "inventory", synopsis: "Show routes, DNS and traffic counters", needsApp: true},
	{path: "diagnose", synopsis: "diagnose [dns|tcp|http|tls|ping|trace] [flags]", needsApp: true, flags: []string{"target", "kind", "port", "record"}},
	{path: "discover", synopsis: "Discover a bounded IPv4 scope", needsApp: true, flags: []string{"target", "ports", "format"}},
	{path: "inspect", synopsis: "Inspect services with separately installed Nmap", needsApp: true, flags: []string{"target", "ports", "format"}},
	{path: "wifi", synopsis: "Show current Wi-Fi metadata", needsApp: true},
	{path: "capture", synopsis: "Capture bounded local IPv4 traffic", needsApp: true, flags: []string{"target", "duration", "limit", "output", "format"}},
	{path: "import", synopsis: "Import a PCAP or PCAPNG file", needsApp: true, flags: []string{"file", "output", "format"}},
}
