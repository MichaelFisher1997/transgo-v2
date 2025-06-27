package testutils

import (
	"bytes"
	"context"

	"github.com/a-h/templ"
)

// RenderComponent converts a templ component to a string for testing
func RenderComponent(c templ.Component) (string, error) {
	buf := new(bytes.Buffer)
	ctx := templ.InitializeContext(context.Background())
	err := c.Render(ctx, buf)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// MustRender renders a component or panics (for test setup)
func MustRender(c templ.Component) string {
	s, err := RenderComponent(c)
	if err != nil {
		panic(err)
	}
	return s
}
