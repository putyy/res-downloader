package system

import (
	"reflect"
	"testing"
)

type testProxyStore map[string]any

func (s testProxyStore) read(key string) (any, error) { return s[key], nil }
func (s testProxyStore) write(key string, _, target any) (any, error) {
	s[key] = target
	return target, nil
}

func TestProxyRestoration(t *testing.T) {
	for _, test := range []struct {
		name             string
		before, external any
	}{
		{"existing proxy", "127.0.0.1:7890", nil},
		{"no proxy", nil, nil},
		{"another client took over", "127.0.0.1:7890", "127.0.0.1:7891"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := testProxyStore{"system": test.before}
			state := newProxyState()
			setup := &Setup{proxyState: state}
			if setup.WithPassword("").proxyState != state {
				t.Fatal("password copy lost the shared snapshot")
			}
			if err := state.set(store, []proxyChange{{key: "system", own: "127.0.0.1:8899"}}); err != nil {
				t.Fatal(err)
			}
			if err := state.set(store, []proxyChange{{key: "system", own: "other"}}); err != nil {
				t.Fatal(err)
			}
			want := test.before
			if test.external != nil {
				store["system"], want = test.external, test.external
			}
			if err := state.unset(store); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(store["system"], want) {
				t.Fatalf("proxy = %v, want %v", store["system"], want)
			}
			if err := state.unset(store); err != nil {
				t.Fatal(err)
			}
		})
	}
}
