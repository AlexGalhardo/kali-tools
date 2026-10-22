package network_test

import (
	"fmt"
	"net/netip"
	"reflect"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func TestValidateNeighborFamily(t *testing.T) {
	for _, family := range []string{"all", "ipv4", "ipv6", "", "IPv4", "x"} {
		t.Run(family, func(t *testing.T) {
			valid := family == "all" || family == "ipv4" || family == "ipv6"
			if err := network.ValidateNeighborFamily(family); (err == nil) != valid {
				t.Fatal(family, err)
			}
		})
	}
}

func TestNormalizeNeighborsFiltersAndSorts(t *testing.T) {
	rows := []network.Neighbor{
		{Address: "2001:db8::2", Family: "ipv6", InterfaceIndex: 2},
		{Address: "192.0.2.20", Family: "ipv4", InterfaceIndex: 2, MAC: "02:00:00:00:00:20"},
		{Address: "192.0.2.3", Family: "ipv4", InterfaceIndex: 2},
		{Address: "fe80::1%ethernet", Family: "ipv6", InterfaceIndex: 1},
		{Address: "192.0.2.20", Family: "ipv4", InterfaceIndex: 2, MAC: "duplicate"},
		{Address: "192.0.2.20", Family: "ipv4", InterfaceIndex: 3},
	}
	for _, address := range []string{"224.0.0.22", "255.255.255.255", "::1", "127.0.0.1", "::", "0.0.0.0", "invalid", "ff02::1"} {
		rows = append(rows, network.Neighbor{Address: address, Family: "ipv4"})
	}
	rows = append(rows, network.Neighbor{Address: "192.0.2.255", MAC: "ff:ff:ff:ff:ff:ff", Family: "ipv4"})
	before := append([]network.Neighbor(nil), rows...)
	for _, test := range []struct {
		family string
		want   []string
	}{
		{"all", []string{"fe80::1%ethernet", "192.0.2.3", "192.0.2.20", "2001:db8::2", "192.0.2.20"}},
		{"ipv4", []string{"192.0.2.3", "192.0.2.20", "192.0.2.20"}},
		{"ipv6", []string{"fe80::1%ethernet", "2001:db8::2"}},
	} {
		t.Run(test.family, func(t *testing.T) {
			got := network.NormalizeNeighbors(rows, test.family)
			addresses := make([]string, len(got.Neighbors))
			for i, row := range got.Neighbors {
				addresses[i] = row.Address
				if row.MAC == "duplicate" {
					t.Fatal("kept later duplicate")
				}
			}
			if !reflect.DeepEqual(addresses, test.want) || got.Limited {
				t.Fatal(got)
			}
		})
	}
	if !reflect.DeepEqual(rows, before) {
		t.Fatal("mutated caller rows")
	}
	if got := network.NormalizeNeighbors(nil, "all"); got.Neighbors == nil || len(got.Neighbors) != 0 {
		t.Fatal("nil empty table")
	}
}

func TestNormalizeNeighborsBound(t *testing.T) {
	for _, count := range []int{4096, 4097} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			rows := make([]network.Neighbor, count)
			address := netip.MustParseAddr("10.0.0.1")
			for i := range rows {
				rows[i] = network.Neighbor{Address: address.String(), Family: "ipv4", InterfaceIndex: 1}
				address = address.Next()
			}
			got := network.NormalizeNeighbors(rows, "all")
			if len(got.Neighbors) != 4096 || got.Limited != (count > 4096) {
				t.Fatal(len(got.Neighbors), got.Limited)
			}
		})
	}
}
