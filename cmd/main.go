package main

import (
	// "fmt"
	"os"
	"path/filepath"

	"github.com/DanArmor/GoTorrent/pkg/torrentmeta"
	// tea "github.com/charmbracelet/bubbletea"
)

func min(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func max(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

type Settings struct {
	ConfigPath   string
	DownloadPath string
}

var GlobalSettings Settings

func createAllParentDirs(p string) (*os.File, error) {
    if err := os.MkdirAll(filepath.Dir(p), 0770); err != nil {
        return nil, err
    }
    return os.Create(p)
}

func AddTorent(path string) {
	tf := torrentmeta.New(path, GlobalSettings.DownloadPath)
	for i := range tf.Files{
		createAllParentDirs(tf.Files[i].FullPath)
		f, err := os.Create(tf.Files[i].FullPath)
		if err != nil {
			panic(err)
		}
		if err := f.Truncate(int64(tf.Files[i].Length)); err != nil {
			panic(err)
		}
	}
	tf.Save(filepath.Join(GlobalSettings.ConfigPath, tf.Name+"meta.meta"))
}

func createDir(path string){
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.Mkdir(path, 0777)
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	dir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	GlobalSettings.ConfigPath = filepath.Join(dir, ".gotorrent")
	createDir(GlobalSettings.ConfigPath)
	dir, err = os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	GlobalSettings.DownloadPath = filepath.Join(dir, "Downloads")
	AddTorent("testFolder.torrent")
	// m := NewModel()
	// if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
	// 	fmt.Println("Error during running program:", err)
	// 	os.Exit(1)
	// }
}
