package main

import (

	//	"github.com/DanArmor/GoTorrent/pkg/torrent"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	// "fyne.io/fyne/v2/data/binding"
)

var data = []string{"a", "b", "c"}

func main() {

	mainApp := app.NewWithID("MainApp")
	mainWindow := mainApp.NewWindow("GoTorrent")
	findTorrent := widget.NewEntry()
	findTorrent.PlaceHolder = "Find torrent by name. . ."
	
	mainWindow.SetContent(
		container.NewBorder(
			container.NewGridWithColumns(
				2,
				container.NewHBox(
					widget.NewButton("Open URL", func(){}),
					widget.NewButton("Open", func(){}),
					widget.NewButton("Remove", func(){}),
					widget.NewButton("Resume", func(){}),
					widget.NewButton("Pause", func(){}),
					widget.NewButton("Preferences", func(){}),
				),
				findTorrent,
			),
			container.NewVBox(
				layout.NewSpacer(),
				container.NewHBox(
					widget.NewButton("General", func() {
						mainApp.Quit()
					}),
					widget.NewButton("Trackers", func() {
						mainApp.Quit()
					}),
					widget.NewButton("Peers", func() {
						mainApp.Quit()
					}),
					widget.NewButton("Content", func() {
						mainApp.Quit()
					}),
					layout.NewSpacer(),
					widget.NewButton("Speed", func() {
						mainApp.Quit()
					}),
				),
			),
			container.NewVBox(
				widget.NewLabel("Status"),
				widget.NewSeparator(),
				widget.NewButton("All", func() {
					mainApp.Quit()
				}),
				widget.NewButton("Downloading", func() {
					mainApp.Quit()
				}),
				widget.NewButton("Seeding", func() {
					mainApp.Quit()
				}),
				widget.NewButton("Completed", func() {
					mainApp.Quit()
				}),
				widget.NewButton("Resumed", func() {
					mainApp.Quit()
				}),
				widget.NewButton("Paused", func() {
					mainApp.Quit()
				}),
			),
			nil,
			widget.NewTable(
				func()(int, int){return 3, 3},
				func() fyne.CanvasObject {
					return widget.NewLabel("template")
				},
				func(i widget.TableCellID, o fyne.CanvasObject) {
					o.(*widget.Label).SetText(data[i.Row])
				},
			),
	))
	mainWindow.ShowAndRun()
	mainApp.Preferences().SetString("lang", mainApp.Preferences().StringWithFallback("lang", "en-US"))

}
