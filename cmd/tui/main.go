package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eugegm01-dev/gophkeepston/internal/tui"
)

func main() {
	serverAddr := "localhost:50051"
	if len(os.Args) > 1 {
		serverAddr = os.Args[1]
	}
	p := tea.NewProgram(tui.NewModel(serverAddr))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
