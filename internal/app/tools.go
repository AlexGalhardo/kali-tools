package app

import "encoding/xml"

type Tool struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

type Nmap struct {
	XMLName  xml.Name     `xml:"nmaprun" json:"-"`
	Scanner  string       `xml:"scanner,attr" json:"scanner"`
	Version  string       `xml:"version,attr" json:"version"`
	Finished NmapFinished `xml:"runstats>finished" json:"finished"`
	Hosts    []NmapHost   `xml:"host" json:"hosts"`
}
type NmapFinished struct {
	Exit string `xml:"exit,attr" json:"exit"`
}
type NmapState struct {
	State string `xml:"state,attr" json:"state"`
}
type NmapAddress struct {
	Address string `xml:"addr,attr" json:"address"`
	Type    string `xml:"addrtype,attr" json:"type"`
}
type NmapService struct {
	Name    string `xml:"name,attr" json:"name"`
	Product string `xml:"product,attr" json:"product"`
	Version string `xml:"version,attr" json:"version"`
	Method  string `xml:"method,attr" json:"method"`
}
type NmapPort struct {
	Service  NmapService `xml:"service" json:"service"`
	Port     int         `xml:"portid,attr" json:"port"`
	Protocol string      `xml:"protocol,attr" json:"protocol"`
	State    NmapState   `xml:"state" json:"state"`
}
type NmapHost struct {
	Status    NmapState     `xml:"status" json:"status"`
	Addresses []NmapAddress `xml:"address" json:"addresses"`
	Ports     []NmapPort    `xml:"ports>port" json:"ports"`
}
