package wailsui_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/wailsui"
)

func TestBindingSignaturesUseContractTypes(t *testing.T) {
	seen := make(map[reflect.Type]bool)
	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		if seen[typ] {
			return
		}
		seen[typ] = true
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(typ.Elem())
		case reflect.Struct:
			if typ == reflect.TypeFor[time.Time]() {
				return
			}
			if typ.PkgPath() != reflect.TypeFor[contract.Job]().PkgPath() {
				t.Errorf("non-contract type %v", typ)
			}
			for f := range typ.Fields() {
				walk(f.Type)
			}
		}
	}
	typ := reflect.TypeFor[*wailsui.Bindings]()
	for method := range typ.Methods() {
		if method.Name == "Startup" || method.Name == "Shutdown" {
			continue
		}
		for i := 1; i < method.Type.NumIn(); i++ {
			walk(method.Type.In(i))
		}
		for i := 0; i < method.Type.NumOut(); i++ {
			walk(method.Type.Out(i))
		}
	}
}
func TestBindingErrorsAreContractErrors(t *testing.T) {
	b := wailsui.NewWithRuntime(newApplication(t), quietRuntime())
	b.Startup(t.Context())
	defer b.Shutdown(t.Context())
	_, err := b.Discover("0.0.0.0/0", []int{80})
	var mapped contract.Error
	if !errors.As(err, &mapped) || mapped.Code != "invalid_input" || !strings.HasPrefix(err.Error(), "invalid_input: ") {
		t.Fatal(err)
	}
}

func TestSaveMissingCaptureMapsApplicationError(t *testing.T) {
	rt := quietRuntime()
	rt.SaveFile = func(_ context.Context, _ wruntime.SaveDialogOptions) (string, error) {
		return filepath.Join(t.TempDir(), "capture.pcap"), nil
	}
	b := wailsui.NewWithRuntime(newApplication(t), rt)
	b.Startup(t.Context())
	defer b.Shutdown(t.Context())
	saved, err := b.SaveCapture("pcap")
	var mapped contract.Error
	if saved || !errors.As(err, &mapped) || mapped.Code != "invalid_input" {
		t.Fatalf("saved=%v error=%v", saved, err)
	}
}
