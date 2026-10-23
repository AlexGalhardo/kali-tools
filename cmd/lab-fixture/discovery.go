package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/hashicorp/mdns"
	"golang.org/x/net/ipv4"

	"github.com/AlexGalhardo/kali-tools/internal/localdiscovery/snmpfake"
)

const labCommunity = "lab-community"

func startDiscovery(ip net.IP) {
	agent, err := snmpfake.Listen("0.0.0.0:1161", labCommunity, 1)
	if err != nil {
		log.Fatal(err)
	}
	_ = agent
	go serveSSDP(ip)
	go serveMDNS(ip)
}

func serveSSDP(ip net.IP) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 1900})
	if err != nil {
		log.Fatal(err)
	}
	packet := ipv4.NewPacketConn(conn)
	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		log.Fatal(err)
	}
	group := &net.UDPAddr{IP: net.ParseIP("239.255.255.250")}
	if err := packet.JoinGroup(iface, group); err != nil {
		log.Fatal(err)
	}
	location := "http://" + ip.String() + ":8080/description.xml"
	reply := []byte("HTTP/1.1 200 OK\r\nCACHE-CONTROL: max-age=1800\r\nST: upnp:rootdevice\r\nUSN: uuid:kali-tools-lab::upnp:rootdevice\r\nLOCATION: " + location + "\r\n\r\n")
	buf := make([]byte, 8192)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if !bytes.Contains(buf[:n], []byte("M-SEARCH")) {
			continue
		}
		_, _ = conn.WriteToUDP(reply, addr)
	}
}

func serveMDNS(ip net.IP) {
	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		log.Fatal(err)
	}
	service, err := mdns.NewMDNSService("fixture", "_kali-lab._tcp", "local.", "fixture.local.", 8080, []net.IP{ip}, nil)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := mdns.NewServer(&mdns.Config{Zone: service, Iface: iface, Logger: log.New(io.Discard, "", 0)}); err != nil {
		log.Fatal(err)
	}
	select {}
}

func probeSNMP() bool {
	client := &gosnmp.GoSNMP{
		Target: "127.0.0.1", Port: 1161, Version: gosnmp.Version2c, Community: labCommunity,
		Timeout: 300 * time.Millisecond, Retries: 0, Logger: gosnmp.NewLogger(log.New(io.Discard, "", 0)),
	}
	if err := client.Connect(); err != nil {
		return false
	}
	defer func() { _ = client.Conn.Close() }()
	pkt, err := client.Get([]string{".1.3.6.1.2.1.1.5.0"})
	if err != nil || pkt == nil || len(pkt.Variables) != 1 {
		return false
	}
	switch value := pkt.Variables[0].Value.(type) {
	case string:
		return value == snmpfake.SysName
	case []byte:
		return string(value) == snmpfake.SysName
	default:
		return false
	}
}
