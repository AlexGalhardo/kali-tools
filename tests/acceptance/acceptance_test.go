package acceptance

import "testing"

func TestAcceptance(t *testing.T) {
	drivers := []struct {
		name    string
		factory driverFactory
	}{{"cli", newCLI}, {"direct", newDirect}, {"wails", newWails}}
	specs := []struct {
		name string
		run  func(*testing.T, scenarioDriver)
	}{{"diagnostics", DiagnosticsSpec}, {"discovery", DiscoverySpec}, {"history", HistorySpec}}
	for _, spec := range specs {
		t.Run(spec.name, func(t *testing.T) {
			for _, driver := range drivers {
				t.Run(driver.name, func(t *testing.T) {
					spec.run(t, func(t *testing.T) Driver { return driver.factory(t, newApplication(t)) })
				})
			}
		})
	}
}
