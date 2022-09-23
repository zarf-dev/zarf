package message

import "github.com/pterm/pterm"

type Generic struct{}

func (g *Generic) Write(p []byte) (n int, err error) {
	text := string(p)
	pterm.Println(text)
	return len(p), nil
}
