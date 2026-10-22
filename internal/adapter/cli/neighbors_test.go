package cli_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
)

func TestCLINeighborsJSONAndCSV(t *testing.T) {
	for _, mode := range []string{"json", "csv", "bad family", "bad format", "csv json conflict"} {
		t.Run(mode, func(t *testing.T) {
			store := fake.NewStore()
			a := app.New(app.Deps{Jobs: store, Settings: store, Host: fake.Host{NeighborTable: network.NeighborTable{Neighbors: []network.Neighbor{
				{Address: "192.0.2.1", MAC: "02:00:00:00:00:01", Family: "ipv4", InterfaceIndex: 7, InterfaceName: "fixture", State: "reachable", Router: true},
			}, Provider: "synthetic cache"}}})
			t.Cleanup(func() { _ = a.Close() })
			args := []string{"neighbors", "--family", "ipv4", "--format", mode}
			want := 0
			if mode == "bad family" {
				args = []string{"neighbors", "--family", "bogus", "--json"}
				want = 2
			}
			if mode == "bad format" {
				want = 2
			}
			if mode == "csv json conflict" {
				args = []string{"neighbors", "--format", "csv", "--json"}
				want = 2
			}
			var out, stderr bytes.Buffer
			if code := cli.Run(t.Context(), a, args, &out, &stderr); code != want {
				t.Fatal(code, out.String(), stderr.String())
			}
			if want != 0 {
				return
			}
			if mode == "csv" {
				rows, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
				if err != nil || len(rows) != 2 || strings.Join(rows[0], ",") != "schema_version,interface_index,interface_name,family,address,mac,state,router" || rows[1][0] != "1" || rows[1][4] != "192.0.2.1" {
					t.Fatal(rows, err)
				}
			} else {
				var response struct {
					SchemaVersion int                    `json:"schema_version"`
					Data          contract.NeighborTable `json:"data"`
				}
				if err := json.Unmarshal(out.Bytes(), &response); err != nil || response.SchemaVersion != 1 || len(response.Data.Neighbors) != 1 {
					t.Fatal(out.String(), err)
				}
			}
		})
	}
}

func TestNeighborsCSVReportsWriterFailure(t *testing.T) {
	for _, count := range []int{1, 200} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			store := fake.NewStore()
			rows := make([]network.Neighbor, count)
			for i := range rows {
				rows[i] = network.Neighbor{Address: "192.0.2.1", Family: "ipv4", InterfaceIndex: uint32(i + 1)}
			}
			a := app.New(app.Deps{Jobs: store, Settings: store, Host: fake.Host{NeighborTable: network.NeighborTable{Neighbors: rows}}})
			t.Cleanup(func() { _ = a.Close() })
			var stderr bytes.Buffer
			if code := cli.Run(t.Context(), a, []string{"neighbors", "--format", "csv"}, closedOutput{}, &stderr); code != cli.ExitFailed {
				t.Fatal(code, stderr.String())
			}
		})
	}
}
