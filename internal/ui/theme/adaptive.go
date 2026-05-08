package theme

// ColorProfile represents the terminal's color capability.
type ColorProfile int

const (
	ProfileTrueColor ColorProfile = iota
	ProfileANSI256
	ProfileANSI
	ProfileNoColor
)

func (p ColorProfile) String() string {
	switch p {
	case ProfileTrueColor:
		return "truecolor"
	case ProfileANSI256:
		return "ansi256"
	case ProfileANSI:
		return "ansi"
	case ProfileNoColor:
		return "nocolor"
	default:
		return "unknown"
	}
}

func (p ColorProfile) HasColor() bool {
	return p != ProfileNoColor
}

func (p ColorProfile) HasTrueColor() bool {
	return p == ProfileTrueColor
}

// ThemeForProfile returns the appropriate theme for the given color profile.
func ThemeForProfile(_ ColorProfile) Theme {
	return NewAgni()
}
