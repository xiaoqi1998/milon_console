package gen

import (
	"fmt"
	"sync"

	"github.com/milon-labs/milon-go-sdk/provider"
)

// Binder rebinds a generated IDL app object with a freshly loaded provider.
// It is registered by the code generated with tools/idlgen.
type Binder func(pd *provider.Provider) error

var (
	bindersMu sync.RWMutex
	binders   = make(map[string]Binder)
)

// RegisterApp registers the binder of a generated IDL app.
// It is called from the init() of the generated file so the app object
// (e.g. token.ClaimFaucet) can be rebound on every milon.NewClient call.
func RegisterApp(appName string, binder Binder) {
	bindersMu.Lock()
	defer bindersMu.Unlock()
	binders[appName] = binder
}

// BindAll rebinds every registered generated app with the currently loaded
// providers. milon.NewClient calls it after IDL loading, so each NewClient
// re-binds the generated instruction objects to the latest IDL definitions.
func BindAll(providers map[string]*provider.Provider) error {
	bindersMu.RLock()
	defer bindersMu.RUnlock()
	for appName, binder := range binders {
		pd, ok := providers[appName]
		if !ok {
			return fmt.Errorf("gen: provider for IDL app %q not loaded", appName)
		}
		if err := binder(pd); err != nil {
			return fmt.Errorf("gen: failed to bind IDL app %q: %w", appName, err)
		}
	}
	return nil

}
