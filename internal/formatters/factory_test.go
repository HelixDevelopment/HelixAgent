//go:build !integration

package formatters

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestFactory() *FormatterFactory {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	return NewFormatterFactory(DefaultConfig(), logger)
}

func TestNewFormatterFactory(t *testing.T) {
	t.Parallel()
	f := newTestFactory()
	assert.NotNil(t, f)
}

func TestFormatterFactory_Create_Native(t *testing.T) {
	t.Parallel()
	f := newTestFactory()

	_, err := f.Create(&FormatterMetadata{
		Name: "black",
		Type: FormatterTypeNative,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "native formatter creation not supported")
}

func TestFormatterFactory_Create_Service(t *testing.T) {
	t.Parallel()
	f := newTestFactory()

	_, err := f.Create(&FormatterMetadata{
		Name: "sqlfluff",
		Type: FormatterTypeService,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service formatter creation not supported")
}

func TestFormatterFactory_Create_Builtin(t *testing.T) {
	t.Parallel()
	f := newTestFactory()

	_, err := f.Create(&FormatterMetadata{
		Name: "gofmt",
		Type: FormatterTypeBuiltin,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "builtin formatter creation not supported")
}

func TestFormatterFactory_Create_Unified(t *testing.T) {
	t.Parallel()
	f := newTestFactory()

	_, err := f.Create(&FormatterMetadata{
		Name: "prettier",
		Type: FormatterTypeUnified,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unified formatter creation not supported")
}

func TestFormatterFactory_Create_UnknownType(t *testing.T) {
	t.Parallel()
	f := newTestFactory()

	_, err := f.Create(&FormatterMetadata{
		Name: "test",
		Type: FormatterType("custom"),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown formatter type: custom")
}

func TestFormatterFactory_CreateAll(t *testing.T) {
	t.Parallel()
	f := newTestFactory()

	metadataList := []*FormatterMetadata{
		{Name: "black", Type: FormatterTypeNative},
		{Name: "gofmt", Type: FormatterTypeBuiltin},
		{Name: "prettier", Type: FormatterTypeUnified},
	}

	formatters, errors := f.CreateAll(metadataList)
	require.NotNil(t, errors)
	// All should fail since dynamic creation is not supported
	assert.Len(t, formatters, 0)
	assert.Len(t, errors, 3)
}

func TestFormatterFactory_CreateAll_Empty(t *testing.T) {
	t.Parallel()
	f := newTestFactory()

	formatters, errors := f.CreateAll([]*FormatterMetadata{})
	assert.Empty(t, formatters)
	assert.Empty(t, errors)
}
