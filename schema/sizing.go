package ui

const (
	LengthAuto uint8 = iota
	LengthPixels
	LengthPercent
)

// Length stores logical pixels or hundredths of a percent; zero means auto.
// A style presence bit distinguishes an explicit auto override from omission.
type Length struct {
	Unit  uint8  `json:"unit,omitempty"`
	Value uint16 `json:"value,omitempty"`
}

const (
	PercentScale       = 10000
	MaxDimensionPixels = 2048
)

func validLength(length Length) bool {
	switch length.Unit {
	case LengthAuto:
		return length.Value == 0
	case LengthPixels:
		return length.Value <= MaxDimensionPixels
	case LengthPercent:
		return length.Value <= PercentScale
	default:
		return false
	}
}

func validSizing(style Style, kind string, version uint32) bool {
	const sizing = Style2Width | Style2Height
	if style.Set2&sizing != 0 && (version < SchemaSizing || kind == "slot") {
		return false
	}

	if !validLength(style.Width) || !validLength(style.Height) {
		return false
	}

	if style.Set2&Style2Width == 0 && style.Width != (Length{}) {
		return false
	}

	if style.Set2&Style2Height == 0 && style.Height != (Length{}) {
		return false
	}

	if style.Set2&sizing != 0 {
		if style.Set&(StyleMinWidth|StyleMaxWidth) == StyleMinWidth|StyleMaxWidth && style.MinWidth > style.MaxWidth {
			return false
		}

		if style.Set&(StyleMinHeight|StyleMaxHeight) == StyleMinHeight|StyleMaxHeight && style.MinHeight > style.MaxHeight {
			return false
		}
	}

	return true
}
