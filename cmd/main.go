package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func min(a, b int) int{
	if(a < b){
		return a
	} else{
		return b
	}
}

func max(a, b int) int{
	if(a > b){
		return a
	} else{
		return b
	}
}

func main() {
	tabs := []string{"Torrents", "Settings"}
	tabContent := []string{"FirstTabContent", "SecondTabContent"}
	m := model{Tabs: tabs, TabContent: tabContent}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error during running program:", err)
		os.Exit(1)
	}
}
