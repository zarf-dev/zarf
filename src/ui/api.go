package ui

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/defenseunicorns/zarf/src/ui/k8s"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// Startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetZarfState() types.ZarfState {
	return k8s.ViewState()
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	fmt.Println("test")
	return fmt.Sprintf("Hello %s, It's show time, I think.", name)
}
