package icons

import "os"

// Mode controls which icon set to use.
type Mode int

const (
	ModeAuto Mode = iota
	ModeNerd
	ModeASCII
)

// ParseMode converts a string to a Mode.
func ParseMode(s string) Mode {
	switch s {
	case "nerd":
		return ModeNerd
	case "ascii":
		return ModeASCII
	default:
		return ModeAuto
	}
}

// Icon represents a single icon with Nerd Font and ASCII variants.
type Icon struct {
	Nerd  string
	ASCII string
}

// String returns the appropriate glyph based on the active icon mode.
func (i Icon) String(mode Mode) string {
	if mode == ModeNerd || (mode == ModeAuto && DetectNerdFonts()) {
		return i.Nerd
	}
	return i.ASCII
}

// All icon definitions from PRD section 9.2.
var (
	AppMark      = Icon{Nerd: "\U000F0620", ASCII: "\u2261"}
	StashItem    = Icon{Nerd: "\uF423", ASCII: "\u25AA"}
	FileModified = Icon{Nerd: "\uF440", ASCII: "~"}
	FileAdded    = Icon{Nerd: "\uF457", ASCII: "+"}
	FileRemoved  = Icon{Nerd: "\uF458", ASCII: "-"}
	FileRenamed  = Icon{Nerd: "\uF45A", ASCII: "\u2192"}
	Conflict     = Icon{Nerd: "\u26A1", ASCII: "!"}
	Clean        = Icon{Nerd: "\u2713", ASCII: "\u221A"}
	StaleBadge   = Icon{Nerd: "\uF017", ASCII: "\u231B"}
	Export       = Icon{Nerd: "\uF093", ASCII: "\u2191"}
	Import       = Icon{Nerd: "\uF019", ASCII: "\u2193"}
	Search       = Icon{Nerd: "\uF002", ASCII: "/"}
	Undo         = Icon{Nerd: "\uF0E2", ASCII: "\u21BA"}
	Branch       = Icon{Nerd: "\uF418", ASCII: "\u238B"}
)

func AllIcons() []Icon {
	return []Icon{
		AppMark, StashItem, FileModified, FileAdded, FileRemoved,
		FileRenamed, Conflict, Clean, StaleBadge, Export, Import,
		Search, Undo, Branch,
	}
}

func AllIconNames() []string {
	return []string{
		"AppMark", "StashItem", "FileModified", "FileAdded", "FileRemoved",
		"FileRenamed", "Conflict", "Clean", "StaleBadge", "Export", "Import",
		"Search", "Undo", "Branch",
	}
}

var nerdFontsDetected int = -1

// DetectNerdFonts checks if Nerd Fonts are available via NERD_FONTS env var.
func DetectNerdFonts() bool {
	if nerdFontsDetected >= 0 {
		return nerdFontsDetected == 1
	}

	v := os.Getenv("NERD_FONTS")
	switch v {
	case "1", "true", "yes":
		nerdFontsDetected = 1
		return true
	default:
		nerdFontsDetected = 0
		return false
	}
}

// ResetDetection clears the cached detection result (for tests).
func ResetDetection() {
	nerdFontsDetected = -1
}
