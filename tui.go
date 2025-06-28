package main

import (
	"fmt"
	"maps"
	"math"
	"os"
	"slices"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nbd-wtf/go-nostr"
)

type RelayStats struct {
	url    string
	seen   int
	offset int
	date   nostr.Timestamp
}

type UIInsertMsg struct{}

type UIRelayPaginationMsg struct {
	url string
}

type UIRelayEventMsg struct {
	url        string
	kind       int
	created_at int64
}

type UI struct {
	kinds     map[int]int
	relays    map[string]RelayStats
	total     int
	responses int
	spinner   spinner.Model
	quitting  bool
}

func InitializeTUI(bufferSize int) (*tea.Program, chan tea.Msg) {
	ui := tea.NewProgram(
		UI{
			kinds:   make(map[int]int),
			relays:  make(map[string]RelayStats),
			spinner: spinner.New(),
		})

	uiCh := make(chan tea.Msg, bufferSize)

	go func() {
		if _, err := ui.Run(); err != nil {
			fmt.Println("could not start program:", err)
			os.Exit(1)
		}
	}()

	go func() {
		for ui_msg := range uiCh {
			ui.Send(ui_msg)
		}
	}()

	return ui, uiCh
}

func (ui *UI) getOrCreateRelay(url string) RelayStats {
	r, ok := ui.relays[url]
	if !ok {
		r = RelayStats{
			url:    url,
			seen:   0,
			offset: 1,
			date:   math.MaxInt32,
		}
		ui.relays[url] = r
		return r
	}
	return r
}

func (ui *UI) incrementKind(kind int) {
	k, ok := ui.kinds[kind]
	if !ok {
		k = 0
	}
	k++
	ui.kinds[kind] = k
}

func (ui UI) Init() tea.Cmd {
	return tea.Batch(ui.spinner.Tick)
}

func (ui UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {

	case tea.KeyMsg:
		ui.quitting = true
		return ui, tea.Quit

	case UIInsertMsg:
		ui.total++
		return ui, nil

	case UIRelayPaginationMsg:
		stats := ui.getOrCreateRelay(m.url)
		stats.offset++
		ui.relays[m.url] = stats
		return ui, nil

	case UIRelayEventMsg:
		url := m.url
		stats := ui.getOrCreateRelay(url)
		stats.seen++
		timestamp := nostr.Timestamp(m.created_at)
		if stats.date > timestamp {
			stats.date = timestamp
		}
		ui.relays[url] = stats
		ui.incrementKind(m.kind)
		return ui, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		ui.spinner, cmd = ui.spinner.Update(msg)
		return ui, cmd

	default:
		return ui, nil
	}
}

func (ui UI) View() string {
	s := "\n"

	kinds := slices.Sorted(maps.Keys(ui.kinds))
	relays := slices.Sorted(maps.Keys(ui.relays))

	s += fmt.Sprintf("  %-4s\n", "kinds")
	for _, k := range kinds {
		s += fmt.Sprintf("Kind %-5d %d\n", k, ui.kinds[k])
	}

	s += fmt.Sprintf("\n\n  %-40s %-10s %s\n", "relays", "events", "current date")
	for _, k := range relays {
		r := ui.relays[k]
		s += fmt.Sprintf("%-40s %-4d date %s\n",
			r.url,
			r.seen,
			r.date.Time().Format("January 2, 2006 15:04:05"))
	}

	s += fmt.Sprintf("\n%s total inserted: %d\n\n press any key to exit\n", ui.spinner.View(), ui.total)

	if ui.quitting {
		s += "\n"
	}
	return s
}
