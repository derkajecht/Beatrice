package tui

import (
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
)

type ChatConfig struct {
	Title    string              `toml:"title"`
	Sections []ChatSectionConfig `toml:"sections"`
}

type ChatSectionConfig struct {
	Type     string         `toml:"type"`
	Title    string         `toml:"title"`
	Width    float64        `toml:"width"`
	Row      int            `toml:"row"`
	ChatView ChatViewConfig `toml:"chat_view"`
	Header   HeaderConfig   `toml:"header"`
	Sidebar  SidebarConfig  `toml:"sidebar"`
}

type Section interface {
	Name() string
	Init() tea.Cmd
	Update(msg tea.Msg) (Section, tea.Cmd)
	View(width, height int, focused bool) string
}

type SectionFactory func(ChatSectionConfig) Section

// registry of all available sections to be built
var registry = map[string]SectionFactory{
	"chat": func(config ChatSectionConfig) Section {
		return newChatModel(config.ChatView)
	},
	"sidebar": func(config ChatSectionConfig) Section {
		return newSidebarModel(config.Sidebar)
	},
	"header": func(config ChatSectionConfig) Section {
		return newHeaderModel(config.Header)
	},
}

func BuildSections(cfg ChatConfig) []Section {
	var sections []Section
	for _, sc := range cfg.Sections {
		// simple error log on unknown section type
		if sc.Type == "" {
			slog.Warn("unknown dashboard section type")
			continue
		}
		if factory, ok := registry[sc.Type]; ok {
			sections = append(sections, factory(sc))
		}
	}
	return sections
}
