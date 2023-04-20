package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DanArmor/GoTorrent/pkg/bitfield"
	"github.com/DanArmor/GoTorrent/pkg/client"
	"github.com/DanArmor/GoTorrent/pkg/handshake"
	"github.com/DanArmor/GoTorrent/pkg/message"
	"github.com/DanArmor/GoTorrent/pkg/p2p"
	"github.com/DanArmor/GoTorrent/pkg/torrent"
	"github.com/DanArmor/GoTorrent/pkg/torrentmeta"
	"github.com/DanArmor/GoTorrent/pkg/utils"
	tea "github.com/charmbracelet/bubbletea"
)

var SeedPeerID [utils.PeerIDLen]byte

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
	Torrents     []torrentmeta.TorrentFile
	Ctx          []context.Context
	Wg           sync.WaitGroup
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

	allFilesExist := true
	for i := range tf.Files {
		if fi, err := os.Stat(tf.Files[i].FullPath); errors.Is(err, os.ErrNotExist) || fi.Size() != int64(tf.Files[i].Length) {
			allFilesExist = false
			break
		}
	}

	if allFilesExist {
		p2p.WriteToLog("All files exist")
		if tf.CheckFilesIntegrity() {
			for i := range tf.PieceHashes {
				tf.Bitfield.SetPiece(i)
			}
			tf.IsDone = true
			tf.Downloaded = len(tf.PieceHashes)
		}
	} else {
		for i := range tf.Files {
			f, err := createAllParentDirs(tf.Files[i].FullPath)
			if err != nil {
				panic(err)
			}
			if err := f.Truncate(int64(tf.Files[i].Length)); err != nil {
				panic(err)
			}
			f.Close()
		}
	}
	tf.Save(GlobalSettings.makeMetaName(tf.Name))
	s.Torrents = append(s.Torrents, tf)
	s.Ctx = append(s.Ctx, nil)
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
		s.Ctx = append(s.Ctx, nil)
	}
}

func (s *Settings) RemoveTorrent(index int) {
	metaName := s.makeMetaName(s.Torrents[index].Name)
	s.stopTorrent(index)
	s.Torrents = append(s.Torrents[:index], s.Torrents[index+1:]...)
	s.Ctx = append(s.Ctx[:index], s.Ctx[index+1:]...)
	os.Remove(metaName)
}

func createDir(path string) {
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

func ReadBlock(pieceLength int, index int, begin int, length int, files []p2p.File) []byte {
	totalOffset := pieceLength*index + begin
	fileIndex := 0
	for {
		totalOffset = totalOffset - files[fileIndex].Length
		if totalOffset <= 0 {
			break
		} else {
			fileIndex++
		}
	}
	n := 0
	b := make([]byte, length)
	files[fileIndex].Handler.Seek(int64(files[fileIndex].Length + totalOffset), io.SeekStart)
	for {
		r, err := files[fileIndex].Handler.Read(b)
		if err != nil {
			if err != io.EOF {
				panic(err)
			} else {
				fileIndex++
			}
		}
		n += r
		if n == length {
			break
		}
	}
	return b
}

func (s *Settings) SeedTorrent(conn net.Conn) {
	res, err := handshake.Read(conn)
	if err != nil {
		return
	}
	serve := false
	var infoHash [utils.InfoHashLen]byte
	var bf bitfield.Bitfield
	var files []torrent.File
	var ctx context.Context
	var pieceLength int
	for i := range s.Torrents {
		if bytes.Equal(res.InfoHash[:], s.Torrents[i].InfoHash[:]) && s.Torrents[i].IsDone && s.Torrents[i].InProgress {
			serve = true
			infoHash = s.Torrents[i].InfoHash
			bf = s.Torrents[i].Bitfield
			files = s.Torrents[i].Files
			ctx = s.Ctx[i]
			pieceLength = s.Torrents[i].PieceLength
		}
	}
	if serve {
		req := handshake.New(infoHash, SeedPeerID)
		_, err := conn.Write(req.Serialize())
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		cl := &client.Client{
			Conn:     conn,
			Choked:   true,
			InfoHash: infoHash,
			PeerID:   SeedPeerID,
		}
		cl.Conn.SetDeadline(time.Now().Add(1 * time.Second))
		cl.SendBitfield(bf)

		var p2pFiles []p2p.File
		for i := range files {
			f, err := os.Open(files[i].FullPath)
			if err != nil {
				panic(err)
			}
			p2pFiles = append(p2pFiles, p2p.File{File: files[i], Handler: f})
			defer func(index int) {
				p2pFiles[index].Handler.Close()
			}(i)
		}
		for {
			select {
			case <-ctx.Done():
				return
			default:
				cl.Conn.SetDeadline(time.Now().Add(1 * time.Second))
				m, err := cl.Read()
				if err != nil {
					panic(err)
				}
				reqindex, reqbegin, reqlength, err := message.ParseRequest(m)
				if err != nil {
					panic(err)
				}
				if m.ID != message.MsgRequest {
					continue
				}
				b := ReadBlock(pieceLength, reqindex, reqbegin, reqlength, p2pFiles)
				cl.SendPiece(reqindex, reqbegin, b)
			}
		}
	} else {
		p2p.WriteToLog("Reject serve - wrong handshake")
	}
}

func (s *Settings) Seeding(ctx context.Context) {
	p2p.WriteToLog("Started seeding")
	s.Wg.Add(1)
	defer s.Wg.Done()
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", torrentmeta.Port))
	if err != nil {
		p2p.WriteToLog("Error: " + fmt.Sprint(err))
	}
	defer ln.Close()
	var wg sync.WaitGroup
	go func() {
		for {
			p2p.WriteToLog("Accepting")
			conn, err := ln.Accept()
			if err != nil {
				p2p.WriteToLog("Error: " + fmt.Sprint(err))
			}
			p2p.WriteToLog("New connection")
			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				p2p.WriteToLog("Start seed")
				s.SeedTorrent(c)
			}(conn)
		}
	}()
	<-ctx.Done()
	ln.Close()
	wg.Wait()
}

func (s *Settings) startTorrent(index int) {
	s.Wg.Add(1)
	s.Torrents[index].InProgress = true
	s.Torrents[index].Count = make(chan int)
	s.Torrents[index].Done = make(chan struct{})
	s.Torrents[index].Out = make(chan struct{})
	if s.Torrents[index].IsDone {
		trackerUrl, err := s.Torrents[index].BuildTrackerURL(SeedPeerID, torrentmeta.Port)
		if err != nil {
			panic(err)
		}
		c := &http.Client{Timeout: 15 * time.Second}
		resp, err := c.Get(trackerUrl)
		if err != nil {
			p2p.WriteToLog("Can't announce")
		}
		p2p.WriteToLog("Announced with " + trackerUrl)
		defer resp.Body.Close()
		go func() {
			defer s.Wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			s.Ctx[index] = ctx
			s.Torrents[index].InProgress = true
			<-s.Torrents[index].Done
			cancel()
		}()
	} else {
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
}

func (s *Settings) stopTorrent(index int) {
	if s.Torrents[index].InProgress {
		if s.Torrents[index].IsDone {
			s.Torrents[index].Done <- struct{}{}
		} else {
			s.Torrents[index].Done <- struct{}{}
			<-s.Torrents[index].Out
		}
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
	_, err := rand.Read(SeedPeerID[:])
	if err != nil {
		panic(err)
	}
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
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		GlobalSettings.Seeding(ctx)
	}()
	m := NewModel()
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		cancel()
		fmt.Println("Error during running program:", err)
		fmt.Println("Wait for goroutines")
		GlobalSettings.stopAllTorrents()
		GlobalSettings.Wg.Wait()
		os.Exit(1)
	}
	cancel()
	GlobalSettings.stopAllTorrents()
	GlobalSettings.Wg.Wait()
}
