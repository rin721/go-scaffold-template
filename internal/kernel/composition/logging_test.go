package composition

import (
	"testing"

	"github.com/rin721/go-scaffold2/internal/kernel"
	builtincli "github.com/rin721/go-scaffold2/internal/kernel/builtin/cli"
	builtinconfig "github.com/rin721/go-scaffold2/internal/kernel/builtin/config"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
)

func newTestAssembly(t *testing.T, source config.Source, cli *pkgcli.Config) *kernel.Assembly {
	t.Helper()
	options := kernel.AssemblyOptions{Config: builtinconfig.Options{Sources: []config.Source{source}}}
	if cli != nil {
		options.CLI = &builtincli.Options{App: *cli}
	}
	assembly, err := kernel.NewAssembly(options)
	if err != nil {
		t.Fatalf("kernel.NewAssembly() error = %v", err)
	}
	return assembly
}
