// Command lab-fixture serves deterministic, container-local diagnostic targets.
package main

import (
	"context"
	"golang.org/x/net/dns/dnsmessage"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--health" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:8080/health", nil)
		if e != nil {
			os.Exit(1)
		}
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			os.Exit(1)
		}
		resp.Body.Close()
		if resp.StatusCode != 204 {
			os.Exit(1)
		}
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	go func() {
		listener, e := net.Listen("tcp", ":9090")
		if e != nil {
			log.Fatal(e)
		}
		defer listener.Close()
		for {
			c, e := listener.Accept()
			if e != nil {
				return
			}
			c.Close()
		}
	}()
	go serveDNS()
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, IdleTimeout: 2 * time.Second}
	log.Fatal(server.ListenAndServe())
}
func serveDNS() {
	socket, e := net.ListenPacket("udp", ":5353")
	if e != nil {
		log.Fatal(e)
	}
	defer socket.Close()
	buffer := make([]byte, 4096)
	for {
		n, addr, e := socket.ReadFrom(buffer)
		if e != nil {
			return
		}
		var query dnsmessage.Message
		if query.Unpack(buffer[:n]) != nil || len(query.Questions) != 1 {
			continue
		}
		q := query.Questions[0]
		response := dnsmessage.Message{Header: dnsmessage.Header{ID: query.ID, Response: true, RecursionDesired: query.RecursionDesired, RecursionAvailable: true}, Questions: query.Questions}
		if q.Name.String() == "fixture.test." {
			if q.Type == dnsmessage.TypeA {
				response.Answers = []dnsmessage.Resource{{Header: dnsmessage.ResourceHeader{Name: q.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 1}, Body: &dnsmessage.AResource{A: [4]byte{192, 0, 2, 42}}}}
			}
		} else {
			response.RCode = dnsmessage.RCodeNameError
		}
		raw, e := response.Pack()
		if e == nil {
			if _, e = socket.WriteTo(raw, addr); e != nil {
				log.Print("DNS fixture write failed")
			}
		}
	}
}
