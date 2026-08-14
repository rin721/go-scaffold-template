// Package i18n 定义由 Kernel 治理的 I18n App 组件。
package i18n

import (
	"context"
	"fmt"
	"os"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	pkgi18n "github.com/rin721/go-scaffold-template/pkg/i18n"
)

const (
	ID         app.ID = "i18n"
	ConfigPath        = "i18n"
)

// Config 是 I18n App 的 typed 配置契约。
type Config struct {
	DefaultLanguage string                  `mapstructure:"defaultLanguage"`
	MessageFiles    []string                `mapstructure:"messageFiles"`
	MissingBehavior pkgi18n.MissingBehavior `mapstructure:"missingBehavior"`
}

type translator struct {
	delegate app.Lease[pkgi18n.Translator]
}

// Definition 返回无安装副作用的 I18n 组件声明。
func Definition() (app.Definition[pkgi18n.Translator], error) {
	source, err := app.Configured(ConfigPath, decode, defaults{})
	if err != nil {
		return app.Definition[pkgi18n.Translator]{}, err
	}
	return app.ManagedConfigured(
		ID,
		source,
		app.FixedDependencies(struct{}{}),
		build,
		app.Leased(newTranslator),
		app.KernelInstanceSwap,
	)
}

func newTranslator(delegate app.Lease[pkgi18n.Translator]) (pkgi18n.Translator, error) {
	if delegate == nil {
		return nil, fmt.Errorf("i18n lease is nil")
	}
	return &translator{delegate: delegate}, nil
}

func (t *translator) Translate(language string, message pkgi18n.Message) (string, error) {
	var text string
	err := t.delegate.Use(context.Background(), func(current pkgi18n.Translator) error {
		if current == nil {
			return fmt.Errorf("i18n translator instance is nil")
		}
		var err error
		text, err = current.Translate(language, message)
		return err
	})
	return text, err
}

func (t *translator) MustTranslate(language string, message pkgi18n.Message) string {
	text, err := t.Translate(language, message)
	if err != nil {
		panic(err)
	}
	return text
}

func build(ctx context.Context, cfg Config, _ struct{}) (pkgi18n.Translator, error) {
	if ctx == nil {
		return nil, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	packageConfig := cfg.packageConfig()
	current, err := pkgi18n.New(&packageConfig)
	if err != nil {
		return nil, fmt.Errorf("create i18n translator: %w", err)
	}
	return current, nil
}

func decode(snapshot config.Snapshot) (Config, error) {
	cfg := defaultConfig()
	if err := snapshot.DecodeSection(ConfigPath, &cfg); err != nil {
		return Config{}, err
	}
	packageConfig := cfg.packageConfig()
	if err := pkgi18n.ValidateConfig(&packageConfig); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

type defaults struct{}

func (defaults) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, app.ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, config.Continue, err
	}
	values := pkgi18n.DefaultConfig()
	return config.Object{
		config.FieldOf("defaultLanguage", config.String(values.DefaultLanguage)),
		config.FieldOf("messageFiles", stringList(values.MessageFiles)),
		config.FieldOf("missingBehavior", config.String(string(values.MissingBehavior))),
	}, config.Continue, nil
}

func defaultConfig() Config {
	values := pkgi18n.DefaultConfig()
	return Config{
		DefaultLanguage: values.DefaultLanguage,
		MessageFiles:    append([]string(nil), values.MessageFiles...),
		MissingBehavior: values.MissingBehavior,
	}
}

func (c Config) packageConfig() pkgi18n.Config {
	return pkgi18n.Config{
		DefaultLanguage: c.DefaultLanguage,
		MessageFiles:    append([]string(nil), c.MessageFiles...),
		MessageFS:       os.DirFS("."),
		MissingBehavior: c.MissingBehavior,
	}
}

func stringList(values []string) config.Value {
	elements := make([]config.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, config.String(value))
	}
	return config.List(elements...)
}

var _ pkgi18n.Translator = (*translator)(nil)
var _ config.DefaultContract = defaults{}
