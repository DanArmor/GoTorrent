package main

import (
	"log"
	"os"

	"github.com/DanArmor/GoTorrent/pkg/torrent"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("Not enough args")
	}
	inPath := os.Args[1]
	outPath := os.Args[2]

	tf, err := torrent.Parse(inPath)
	if err != nil {
		log.Fatal(err)
	}

	err = tf.DownloadToFile(outPath)
	if err != nil {
		log.Fatal(err)
	}
}
