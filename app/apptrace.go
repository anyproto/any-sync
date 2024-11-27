//go:build appdebug
// +build appdebug

package app

func (app *App) SetOnComponentListener(listener func(comp Component)) {
	app.componentListener = listener
}
