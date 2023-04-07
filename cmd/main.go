package main

import (

	//	"github.com/DanArmor/GoTorrent/pkg/torrent"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/DanArmor/GoTorrent/pkg/torrent"
	// "fyne.io/fyne/v2/data/binding"
)

var data = []string{"a", "b", "c"}

func main() {

	mainApp := app.NewWithID("MainApp")
	mainWindow := mainApp.NewWindow("GoTorrent")
	
	mainWindow.SetContent(
		container.NewBorder(
			container.NewGridWithColumns(
				2,
				container.NewHBox(
					widget.NewButton("Open URL", func(){}),
					widget.NewButton("Open", func(){
						fileDialog := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error){
							if err != nil {
								panic(err)
							}
							if r != nil {
								defer r.Close()
								tf, err := torrent.Parse(r.URI().String())
								if err != nil {
									panic(err)
								}
								err = tf.DownloadToFile(mainApp.Preferences().String("downloadpath"))
								if err != nil {
									panic(err)
								}
							}
						}, mainWindow)
						fileDialog.Show()
					}),
					widget.NewButton("Remove", func(){}),
					widget.NewButton("Resume", func(){}),
					widget.NewButton("Pause", func(){}),
					widget.NewButton("Preferences", func(){}),
				),
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
			nil,
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

	mainApp.Preferences().SetString("lang", mainApp.Preferences().StringWithFallback("lang", "en-US"))
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	mainApp.Preferences().SetString("downloadpath", mainApp.Preferences().StringWithFallback("downloadpath", filepath.Join(userHomeDir, "Downloads")))

	mainWindow.ShowAndRun()

}
