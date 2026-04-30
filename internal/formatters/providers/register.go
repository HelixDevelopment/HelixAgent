package providers

import (
	"os"

	"dev.helix.agent/internal/formatters"
	"dev.helix.agent/internal/formatters/providers/native"
	"dev.helix.agent/internal/formatters/providers/service"
	"github.com/sirupsen/logrus"
)

// RegisterAllFormatters registers all available formatters with the registry
// using lazy initialization. Formatter instances are created on first access,
// not at registration time.
func RegisterAllFormatters(registry *formatters.FormatterRegistry, logger *logrus.Logger) error {
	// Track registration stats
	registered := 0
	failed := 0

	// Helper function to register lazily and track
	registerLazy := func(
		factory formatters.LazyFormatterFunc,
		metadata *formatters.FormatterMetadata,
	) {
		if err := registry.RegisterLazy(factory, metadata); err != nil {
			logger.Warnf("Failed to register %s: %v", metadata.Name, err)
			failed++
		} else {
			registered++
		}
	}

	// Python formatters — ruff registered FIRST because it's the
	// canonical preference per internal/formatters/config.go (and 30x
	// faster than black). Was registered after black, which caused the
	// system to pick black first; if black wasn't installed (common on
	// fresh systems) /v1/format silently failed for Python. CONST-035
	// §c regression-fix companion to commit d82d631c (gofmt fix).
	registerLazy(
		func() (formatters.Formatter, error) {
			return native.NewRuffFormatter(logger), nil
		},
		&formatters.FormatterMetadata{
			Name: "ruff", Type: formatters.FormatterTypeNative, Version: "0.9.6",
			Languages: []string{"python"}, Performance: "very_fast",
			SupportsStdin: true, SupportsInPlace: true, SupportsCheck: true,
			SupportsConfig: true,
		},
	)

	registerLazy(
		func() (formatters.Formatter, error) {
			return native.NewBlackFormatter(logger), nil
		},
		&formatters.FormatterMetadata{
			Name: "black", Type: formatters.FormatterTypeNative, Version: "26.1a1",
			Languages: []string{"python"}, Performance: "medium",
			SupportsStdin: true, SupportsInPlace: true, SupportsCheck: true,
			SupportsConfig: true,
		},
	)

	// JavaScript/TypeScript formatters
	registerLazy(
		func() (formatters.Formatter, error) {
			return native.NewPrettierFormatter(logger), nil
		},
		&formatters.FormatterMetadata{
			Name: "prettier", Type: formatters.FormatterTypeUnified, Version: "3.4.2",
			Languages:     []string{"javascript", "typescript", "json", "html", "css", "scss", "markdown", "yaml", "graphql"},
			Performance:   "medium",
			SupportsStdin: true, SupportsInPlace: true, SupportsCheck: true,
			SupportsConfig: true,
		},
	)

	registerLazy(
		func() (formatters.Formatter, error) {
			return native.NewBiomeFormatter(logger), nil
		},
		&formatters.FormatterMetadata{
			Name: "biome", Type: formatters.FormatterTypeNative, Version: "1.9.4",
			Languages:     []string{"javascript", "typescript", "json", "jsx", "tsx"},
			Performance:   "very_fast",
			SupportsStdin: true, SupportsInPlace: true, SupportsCheck: true,
			SupportsConfig: true,
		},
	)

	// Systems languages
	registerLazy(
		func() (formatters.Formatter, error) {
			return native.NewRustfmtFormatter(logger), nil
		},
		&formatters.FormatterMetadata{
			Name: "rustfmt", Type: formatters.FormatterTypeNative, Version: "1.8.1",
			Languages: []string{"rust"}, Performance: "fast",
			SupportsStdin: true, SupportsInPlace: true, SupportsCheck: true,
			SupportsConfig: true,
		},
	)

	registerLazy(
		func() (formatters.Formatter, error) {
			return native.NewGofmtFormatter(logger), nil
		},
		&formatters.FormatterMetadata{
			Name: "gofmt", Type: formatters.FormatterTypeBuiltin, Version: "go1.24.11",
			Languages: []string{"go"}, Performance: "fast",
			SupportsStdin: true, SupportsInPlace: true, SupportsCheck: false,
			SupportsConfig: false,
		},
	)

	registerLazy(
		func() (formatters.Formatter, error) {
			return native.NewClangFormatFormatter(logger), nil
		},
		&formatters.FormatterMetadata{
			Name: "clang-format", Type: formatters.FormatterTypeNative, Version: "19.1.8",
			Languages:     []string{"c", "cpp", "java", "javascript", "objectivec", "protobuf"},
			Performance:   "fast",
			SupportsStdin: true, SupportsInPlace: true, SupportsCheck: false,
			SupportsConfig: true,
		},
	)

	// Scripting languages
	registerLazy(
		func() (formatters.Formatter, error) {
			return native.NewShfmtFormatter(logger), nil
		},
		&formatters.FormatterMetadata{
			Name: "shfmt", Type: formatters.FormatterTypeNative, Version: "3.10.0",
			Languages: []string{"bash", "sh", "shell"}, Performance: "fast",
			SupportsStdin: true, SupportsInPlace: true, SupportsCheck: false,
			SupportsConfig: true,
		},
	)

	registerLazy(
		func() (formatters.Formatter, error) {
			return native.NewStyluaFormatter(logger), nil
		},
		&formatters.FormatterMetadata{
			Name: "stylua", Type: formatters.FormatterTypeNative, Version: "2.0.2",
			Languages: []string{"lua"}, Performance: "fast",
			SupportsStdin: true, SupportsInPlace: true, SupportsCheck: true,
			SupportsConfig: true,
		},
	)

	// Data formats
	registerLazy(
		func() (formatters.Formatter, error) {
			return native.NewYamlfmtFormatter(logger), nil
		},
		&formatters.FormatterMetadata{
			Name: "yamlfmt", Type: formatters.FormatterTypeNative, Version: "0.14.0",
			Languages: []string{"yaml", "yml"}, Performance: "fast",
			SupportsStdin: true, SupportsInPlace: true, SupportsCheck: false,
			SupportsConfig: true,
		},
	)

	registerLazy(
		func() (formatters.Formatter, error) {
			return native.NewTaploFormatter(logger), nil
		},
		&formatters.FormatterMetadata{
			Name: "taplo", Type: formatters.FormatterTypeNative, Version: "0.9.3",
			Languages: []string{"toml"}, Performance: "fast",
			SupportsStdin: true, SupportsInPlace: true, SupportsCheck: false,
			SupportsConfig: true,
		},
	)

	// Service formatters (optional, requires Docker services running)
	serviceBaseURL := os.Getenv("FORMATTER_SERVICE_BASE_URL")
	if serviceBaseURL == "" {
		serviceBaseURL = "http://localhost"
	}

	enableServiceFormatters := os.Getenv("FORMATTER_ENABLE_SERVICES")
	if enableServiceFormatters == "true" || enableServiceFormatters == "1" {
		logger.Info("Registering service formatters (lazy)...")

		// Python service formatters
		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewAutopep8Formatter(serviceBaseURL, logger), nil
			},
			service.NewAutopep8Formatter(serviceBaseURL, logger).GetMetadata(),
		)

		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewYapfFormatter(serviceBaseURL, logger), nil
			},
			service.NewYapfFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		// SQL formatter
		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewSQLFluffFormatter(serviceBaseURL, logger), nil
			},
			service.NewSQLFluffFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		// Ruby formatters
		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewRubocopFormatter(serviceBaseURL, logger), nil
			},
			service.NewRubocopFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewStandardRBFormatter(serviceBaseURL, logger), nil
			},
			service.NewStandardRBFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		// PHP formatters
		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewPHPCSFixerFormatter(serviceBaseURL, logger), nil
			},
			service.NewPHPCSFixerFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewLaravelPintFormatter(serviceBaseURL, logger), nil
			},
			service.NewLaravelPintFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		// Other languages
		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewPerltidyFormatter(serviceBaseURL, logger), nil
			},
			service.NewPerltidyFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewCljfmtFormatter(serviceBaseURL, logger), nil
			},
			service.NewCljfmtFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewSpotlessFormatter(serviceBaseURL, logger), nil
			},
			service.NewSpotlessFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewGroovyLintFormatter(serviceBaseURL, logger), nil
			},
			service.NewGroovyLintFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewStylerFormatter(serviceBaseURL, logger), nil
			},
			service.NewStylerFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewAirFormatter(serviceBaseURL, logger), nil
			},
			service.NewAirFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		registerLazy(
			func() (formatters.Formatter, error) {
				return service.NewPSScriptAnalyzerFormatter(serviceBaseURL, logger), nil
			},
			service.NewPSScriptAnalyzerFormatter(serviceBaseURL, logger).GetMetadata(),
		)

		logger.Infof("Service formatters registration complete")
	} else {
		logger.Info("Service formatters disabled (set FORMATTER_ENABLE_SERVICES=true to enable)")
	}

	logger.Infof("Formatter registration complete: %d registered (lazy), %d failed",
		registered, failed)

	return nil
}
