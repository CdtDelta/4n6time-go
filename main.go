package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	appMenu := menu.NewMenu()

	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Open Database", keys.CmdOrCtrl("o"), func(cd *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:open-database")
	})
	fileMenu.AddText("Import CSV", keys.CmdOrCtrl("i"), func(cd *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:import-csv")
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Close Database", keys.CmdOrCtrl("w"), func(cd *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:close-database")
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Export CSV", keys.CmdOrCtrl("e"), func(cd *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:export-csv")
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(cd *menu.CallbackData) {
		runtime.Quit(app.ctx)
	})

	editMenu := appMenu.AddSubmenu("Edit")
	editMenu.AddText("Cut", keys.CmdOrCtrl("x"), nil)
	editMenu.AddText("Copy", keys.CmdOrCtrl("c"), nil)
	editMenu.AddText("Paste", keys.CmdOrCtrl("v"), nil)
	editMenu.AddText("Select All", keys.CmdOrCtrl("a"), nil)

	viewMenu := appMenu.AddSubmenu("View")
	viewMenu.AddText("Theme...", keys.CmdOrCtrl("t"), func(cd *menu.CallbackData) {
		runtime.EventsEmit(app.ctx, "menu:theme")
	})

	err := wails.Run(&options.App{
		Title:  "4n6time v" + Version + " - Forensic Timeline Viewer",
		Width:  1400,
		Height: 900,
		Menu:   appMenu,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
