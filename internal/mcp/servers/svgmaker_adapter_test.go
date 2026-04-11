package servers

import (
	"context"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSVGMakerAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, nil)

	assert.NotNil(t, adapter)
	assert.False(t, adapter.initialized)
	assert.Equal(t, 800, adapter.config.DefaultWidth)
	assert.Equal(t, 600, adapter.config.DefaultHeight)
}

func TestDefaultSVGMakerAdapterConfig(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()

	assert.Equal(t, 800, config.DefaultWidth)
	assert.Equal(t, 600, config.DefaultHeight)
	assert.Equal(t, 4000, config.MaxWidth)
	assert.Equal(t, 4000, config.MaxHeight)
	assert.Equal(t, 1.0, config.DefaultStrokeWidth)
	assert.Equal(t, "Arial, sans-serif", config.DefaultFontFamily)
	assert.Equal(t, 12, config.DefaultFontSize)
}

func TestSVGMakerAdapter_Initialize(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)
	assert.True(t, adapter.initialized)
}

func TestSVGMakerAdapter_Health(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	// Health check should fail if not initialized
	err := adapter.Health(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Initialize and check again
	err = adapter.Initialize(context.Background())
	require.NoError(t, err)

	err = adapter.Health(context.Background())
	assert.NoError(t, err)
}

func TestSVGMakerAdapter_CreateSVG(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	doc := &SVGDocument{
		Width:      400,
		Height:     300,
		Background: "#fff",
		Elements: []SVGElement{
			{
				Type: "rect",
				Attributes: map[string]interface{}{
					"x":      10,
					"y":      10,
					"width":  100,
					"height": 50,
					"fill":   "blue",
				},
			},
		},
	}

	svg, err := adapter.CreateSVG(context.Background(), doc)
	require.NoError(t, err)

	assert.Contains(t, svg, "<svg")
	assert.Contains(t, svg, "</svg>")
	assert.Contains(t, svg, `width="400"`)
	assert.Contains(t, svg, `height="300"`)
	assert.Contains(t, svg, "<rect")
	assert.Contains(t, svg, `fill="blue"`)
}

func TestSVGMakerAdapter_CreateSVG_DefaultDimensions(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	doc := &SVGDocument{
		Elements: []SVGElement{
			{Type: "circle", Attributes: map[string]interface{}{"r": 50}},
		},
	}

	svg, err := adapter.CreateSVG(context.Background(), doc)
	require.NoError(t, err)

	assert.Contains(t, svg, `width="800"`)
	assert.Contains(t, svg, `height="600"`)
}

func TestSVGMakerAdapter_CreateSVG_MaxDimensions(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	doc := &SVGDocument{
		Width:  5000, // Exceeds max
		Height: 300,
	}

	_, err = adapter.CreateSVG(context.Background(), doc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "width exceeds maximum")
}

func TestSVGMakerAdapter_CreateSVG_AllElementTypes(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	doc := &SVGDocument{
		Width:  800,
		Height: 600,
		Elements: []SVGElement{
			{Type: "rect", Attributes: map[string]interface{}{"x": 10, "y": 10, "width": 100, "height": 50}},
			{Type: "circle", Attributes: map[string]interface{}{"cx": 200, "cy": 100, "r": 40}},
			{Type: "ellipse", Attributes: map[string]interface{}{"cx": 300, "cy": 100, "rx": 50, "ry": 30}},
			{Type: "line", Attributes: map[string]interface{}{"x1": 400, "y1": 50, "x2": 500, "y2": 150}},
			{Type: "polyline", Attributes: map[string]interface{}{"points": "50,200 100,250 150,200"}},
			{Type: "polygon", Attributes: map[string]interface{}{"points": "200,200 250,250 150,250"}},
			{Type: "path", Attributes: map[string]interface{}{"d": "M 300 200 L 400 300 L 350 250 Z"}},
			{Type: "text", Content: "Hello SVG", Attributes: map[string]interface{}{"x": 500, "y": 200}},
			{Type: "group", Attributes: map[string]interface{}{"id": "group1"}, Children: []SVGElement{
				{Type: "rect", Attributes: map[string]interface{}{"x": 600, "y": 200, "width": 50, "height": 50}},
			}},
		},
	}

	svg, err := adapter.CreateSVG(context.Background(), doc)
	require.NoError(t, err)

	assert.Contains(t, svg, "<rect")
	assert.Contains(t, svg, "<circle")
	assert.Contains(t, svg, "<ellipse")
	assert.Contains(t, svg, "<line")
	assert.Contains(t, svg, "<polyline")
	assert.Contains(t, svg, "<polygon")
	assert.Contains(t, svg, "<path")
	assert.Contains(t, svg, "<text")
	assert.Contains(t, svg, "Hello SVG")
	assert.Contains(t, svg, "<g")
	assert.Contains(t, svg, "</g>")
}

func TestSVGMakerAdapter_CreateSVG_WithDefs(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	doc := &SVGDocument{
		Width:  400,
		Height: 300,
		Defs: []SVGDef{
			{
				Type: "linearGradient",
				ID:   "grad1",
				Attributes: map[string]interface{}{
					"x1": "0%",
					"y1": "0%",
					"x2": "100%",
					"y2": "0%",
				},
				Stops: []GradientStop{
					{Offset: "0%", Color: "red"},
					{Offset: "100%", Color: "blue"},
				},
			},
		},
		Elements: []SVGElement{
			{Type: "rect", Attributes: map[string]interface{}{
				"width": 200, "height": 100, "fill": "url(#grad1)",
			}},
		},
	}

	svg, err := adapter.CreateSVG(context.Background(), doc)
	require.NoError(t, err)

	assert.Contains(t, svg, "<defs>")
	assert.Contains(t, svg, "<linearGradient")
	assert.Contains(t, svg, `id="grad1"`)
	assert.Contains(t, svg, "<stop")
	assert.Contains(t, svg, "</defs>")
}

func TestSVGMakerAdapter_CreateRectangle(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	svg, err := adapter.CreateRectangle(context.Background(), 10, 20, 100, 50, "red", "black", 2)
	require.NoError(t, err)

	assert.Contains(t, svg, "<rect")
	assert.Contains(t, svg, `x="10"`)
	assert.Contains(t, svg, `y="20"`)
	assert.Contains(t, svg, `width="100"`)
	assert.Contains(t, svg, `height="50"`)
	assert.Contains(t, svg, `fill="red"`)
	assert.Contains(t, svg, `stroke="black"`)
}

func TestSVGMakerAdapter_CreateCircle(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	svg, err := adapter.CreateCircle(context.Background(), 100, 100, 50, "blue", "green", 3)
	require.NoError(t, err)

	assert.Contains(t, svg, "<circle")
	assert.Contains(t, svg, `cx="100"`)
	assert.Contains(t, svg, `cy="100"`)
	assert.Contains(t, svg, `r="50"`)
	assert.Contains(t, svg, `fill="blue"`)
}

func TestSVGMakerAdapter_CreateLine(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	svg, err := adapter.CreateLine(context.Background(), 10, 20, 200, 150, "red", 2)
	require.NoError(t, err)

	assert.Contains(t, svg, "<line")
	assert.Contains(t, svg, `x1="10"`)
	assert.Contains(t, svg, `y1="20"`)
	assert.Contains(t, svg, `x2="200"`)
	assert.Contains(t, svg, `y2="150"`)
}

func TestSVGMakerAdapter_CreateText(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	svg, err := adapter.CreateText(context.Background(), 50, 100, "Hello World", "", 16, "black")
	require.NoError(t, err)

	assert.Contains(t, svg, "<text")
	assert.Contains(t, svg, "Hello World")
	assert.Contains(t, svg, "</text>")
	assert.Contains(t, svg, `font-size="16"`)
}

func TestSVGMakerAdapter_CreatePath(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	svg, err := adapter.CreatePath(context.Background(), "M 10 10 L 100 100 Z", "red", "black", 2)
	require.NoError(t, err)

	assert.Contains(t, svg, "<path")
	assert.Contains(t, svg, `d="M 10 10 L 100 100 Z"`)
}

func TestSVGMakerAdapter_CreateBarChart(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	data := &ChartData{
		Labels: []string{"A", "B", "C"},
		Values: []float64{10, 25, 15},
		Title:  "Test Chart",
	}

	svg, err := adapter.CreateBarChart(context.Background(), data, 400, 300)
	require.NoError(t, err)

	assert.Contains(t, svg, "<svg")
	assert.Contains(t, svg, "<rect")
	assert.Contains(t, svg, "Test Chart")
	assert.Contains(t, svg, "A")
	assert.Contains(t, svg, "B")
	assert.Contains(t, svg, "C")
}

func TestSVGMakerAdapter_CreateBarChart_EmptyData(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	data := &ChartData{}

	_, err = adapter.CreateBarChart(context.Background(), data, 400, 300)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestSVGMakerAdapter_CreatePieChart(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	data := &ChartData{
		Labels: []string{"Slice A", "Slice B", "Slice C"},
		Values: []float64{30, 50, 20},
		Title:  "Pie Chart Test",
	}

	svg, err := adapter.CreatePieChart(context.Background(), data, 400, 400)
	require.NoError(t, err)

	assert.Contains(t, svg, "<svg")
	assert.Contains(t, svg, "<path")
	assert.Contains(t, svg, "Pie Chart Test")
	// Check for percentage labels
	assert.Contains(t, svg, "30.0%")
	assert.Contains(t, svg, "50.0%")
	assert.Contains(t, svg, "20.0%")
}

func TestSVGMakerAdapter_ValidateSVG(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	// Valid SVG
	validSVG := `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><rect/></svg>`
	valid, errors := adapter.ValidateSVG(context.Background(), validSVG)
	assert.True(t, valid)
	assert.Empty(t, errors)

	// Invalid SVG - missing namespace
	invalidSVG := `<svg width="100" height="100"><rect/></svg>`
	valid, errors = adapter.ValidateSVG(context.Background(), invalidSVG)
	assert.False(t, valid)
	assert.Contains(t, errors, "missing xmlns namespace declaration")

	// Invalid SVG - missing closing tag
	invalidSVG2 := `<svg xmlns="http://www.w3.org/2000/svg">`
	valid, errors = adapter.ValidateSVG(context.Background(), invalidSVG2)
	assert.False(t, valid)
	assert.Contains(t, errors, "missing </svg> closing tag")
}

func TestSVGMakerAdapter_GetMCPTools(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	tools := adapter.GetMCPTools()
	assert.Len(t, tools, 9)

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "svg_create")
	assert.Contains(t, toolNames, "svg_rectangle")
	assert.Contains(t, toolNames, "svg_circle")
	assert.Contains(t, toolNames, "svg_line")
	assert.Contains(t, toolNames, "svg_text")
	assert.Contains(t, toolNames, "svg_path")
	assert.Contains(t, toolNames, "svg_bar_chart")
	assert.Contains(t, toolNames, "svg_pie_chart")
	assert.Contains(t, toolNames, "svg_validate")
}

func TestSVGMakerAdapter_ExecuteTool_Rectangle(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	result, err := adapter.ExecuteTool(context.Background(), "svg_rectangle", map[string]interface{}{
		"x":      10.0,
		"y":      20.0,
		"width":  100.0,
		"height": 50.0,
		"fill":   "red",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)

	svg, ok := resultMap["svg"].(string)
	require.True(t, ok)
	assert.Contains(t, svg, "<rect")
}

func TestSVGMakerAdapter_ExecuteTool_BarChart(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	result, err := adapter.ExecuteTool(context.Background(), "svg_bar_chart", map[string]interface{}{
		"labels": []interface{}{"A", "B", "C"},
		"values": []interface{}{10.0, 20.0, 15.0},
		"title":  "Test",
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)

	svg, ok := resultMap["svg"].(string)
	require.True(t, ok)
	assert.Contains(t, svg, "<svg")
	assert.Contains(t, svg, "Test")
}

func TestSVGMakerAdapter_ExecuteTool_Validate(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	result, err := adapter.ExecuteTool(context.Background(), "svg_validate", map[string]interface{}{
		"svg": `<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
	})
	require.NoError(t, err)

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.True(t, resultMap["valid"].(bool))
}

func TestSVGMakerAdapter_ExecuteTool_Unknown(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	_, err = adapter.ExecuteTool(context.Background(), "unknown_tool", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestSVGMakerAdapter_ExecuteTool_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	_, err := adapter.ExecuteTool(context.Background(), "svg_rectangle", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSVGMakerAdapter_Close(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)
	assert.True(t, adapter.initialized)

	err = adapter.Close()
	assert.NoError(t, err)
	assert.False(t, adapter.initialized)
}

func TestSVGMakerAdapter_GetCapabilities(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	caps := adapter.GetCapabilities()
	assert.Equal(t, "svgmaker", caps["name"])
	assert.Equal(t, 800, caps["default_width"])
	assert.Equal(t, 600, caps["default_height"])
	assert.Equal(t, 4000, caps["max_width"])
	assert.Equal(t, 4000, caps["max_height"])
	assert.Equal(t, 9, caps["tools"])
}

func TestSVGMakerAdapter_EscapeXML(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	// Test with special characters in text
	svg, err := adapter.CreateText(context.Background(), 10, 20, "Test <>&\"' chars", "", 12, "black")
	require.NoError(t, err)

	assert.Contains(t, svg, "&lt;")
	assert.Contains(t, svg, "&gt;")
	assert.Contains(t, svg, "&amp;")
	assert.Contains(t, svg, "&quot;")
	assert.Contains(t, svg, "&apos;")
	assert.NotContains(t, svg, "Test <>&\"' chars")
}

func TestSVGMakerAdapter_CreateSVG_NotInitialized(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	doc := &SVGDocument{}
	_, err := adapter.CreateSVG(context.Background(), doc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSVGMakerAdapter_ExecuteTool_AllTools(t *testing.T) {
	t.Parallel()
	config := DefaultSVGMakerAdapterConfig()
	adapter := NewSVGMakerAdapter(config, logrus.New())

	err := adapter.Initialize(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name   string
		params map[string]interface{}
	}{
		{
			name: "svg_create",
			params: map[string]interface{}{
				"width":  100.0,
				"height": 100.0,
				"elements": []interface{}{
					map[string]interface{}{"type": "rect", "attributes": map[string]interface{}{"width": 50.0}},
				},
			},
		},
		{
			name:   "svg_rectangle",
			params: map[string]interface{}{"width": 100.0, "height": 50.0},
		},
		{
			name:   "svg_circle",
			params: map[string]interface{}{"r": 50.0},
		},
		{
			name:   "svg_line",
			params: map[string]interface{}{"x1": 0.0, "y1": 0.0, "x2": 100.0, "y2": 100.0},
		},
		{
			name:   "svg_text",
			params: map[string]interface{}{"text": "Hello"},
		},
		{
			name:   "svg_path",
			params: map[string]interface{}{"d": "M 0 0 L 100 100"},
		},
		{
			name:   "svg_pie_chart",
			params: map[string]interface{}{"labels": []interface{}{"A", "B"}, "values": []interface{}{1.0, 2.0}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := adapter.ExecuteTool(context.Background(), tt.name, tt.params)
			require.NoError(t, err)
			require.NotNil(t, result)

			resultMap, ok := result.(map[string]interface{})
			require.True(t, ok)

			svg, ok := resultMap["svg"].(string)
			require.True(t, ok)
			assert.True(t, strings.Contains(svg, "<svg") || strings.Contains(svg, "valid"))
		})
	}
}
