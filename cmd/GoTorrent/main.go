package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/DanArmor/GoTorrent/pkg/torrentmeta"
	tea "github.com/charmbracelet/bubbletea"
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
	Torrents []torrentmeta.TorrentFile
	Wg sync.WaitGroup
}

var GlobalSettings Settings

func createAllParentDirs(p string) (*os.File, error) {
    if err := os.MkdirAll(filepath.Dir(p), 0770); err != nil {
        return nil, err
    }
    return os.Create(p)
}

func (s *Settings) makeMetaName(name string) string {
	return filepath.Join(s.ConfigPath, name+"_meta.meta")
}

func (s *Settings) AddTorent(path string) {
	tf := torrentmeta.New(path, GlobalSettings.DownloadPath)
	for i := range tf.Files{
		f, err := createAllParentDirs(tf.Files[i].FullPath)
		if err != nil {
			panic(err)
		}
		if err := f.Truncate(int64(tf.Files[i].Length)); err != nil {
			panic(err)
		}
		f.Close()
	}
	tf.Save(GlobalSettings.makeMetaName(tf.Name))
	s.Torrents = append(s.Torrents, tf)
}

func (s *Settings) LoadTorrents() {
	entries, err := os.ReadDir(s.ConfigPath)
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		var tf torrentmeta.TorrentFile
		tf.Load(filepath.Join(s.ConfigPath, e.Name()))
		s.Torrents = append(s.Torrents, tf)
	}
}

func (s *Settings) RemoveTorrent(index int) {
	metaName := s.makeMetaName(s.Torrents[index].Name)
	s.stopTorrent(index)
	s.Torrents = append(s.Torrents[:index], s.Torrents[index+1:]...)
	os.Remove(metaName)
}

func createDir(path string){
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.Mkdir(path, 0777)
		if err != nil {
			panic(err)
		}
	}
}

func (s *Settings) stopAllTorrents() {
	for i := range s.Torrents {
		s.stopTorrent(i)
	}
}

func (s *Settings) startTorrent(index int) {
	if s.Torrents[index].Downloaded == len(s.Torrents[index].PieceHashes) {
		return
	}
	s.Wg.Add(1)
	s.Torrents[index].InProgress = true
	s.Torrents[index].Count = make(chan int)
	s.Torrents[index].Done = make(chan struct{})
	s.Torrents[index].Out = make(chan struct{})
	go func() {
		defer s.Wg.Done()
		s.Torrents[index].DownloadToFile()
		if len(s.Torrents[index].PieceHashes) == s.Torrents[index].Downloaded {
			s.Torrents[index].IsDone = true
			s.Torrents[index].InProgress = false
			s.Torrents[index].Save(s.makeMetaName(s.Torrents[index].Name))
		}
	}()
}

func (s *Settings) stopTorrent(index int) {
	if s.Torrents[index].InProgress {
		s.Torrents[index].Done <- struct{}{}
		<- s.Torrents[index].Out
		s.Torrents[index].InProgress = false
		s.Torrents[index].Save(s.makeMetaName(s.Torrents[index].Name))
	}
}

func formatBytes(size int) string {
    units := []string{"B", "KB", "MB", "GB", "TB"}
    var i int

    for size >= 1024 && i < len(units)-1 {
        size /= 1024
        i++
    }

    return fmt.Sprintf("%d %s", size, units[i])
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

	GlobalSettings.LoadTorrents()

	m := NewModel()
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error during running program:", err)
		fmt.Println("Wait for goroutines")
		GlobalSettings.stopAllTorrents()
		GlobalSettings.Wg.Wait()
		os.Exit(1)
	}
	GlobalSettings.stopAllTorrents()
	GlobalSettings.Wg.Wait()
}
