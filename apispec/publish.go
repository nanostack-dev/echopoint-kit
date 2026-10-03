package apispec

// Live is a spec's current Live version.
type Live struct {
	Document *Document
	Version  string
}

// Publication is a document ready to become the next Live version.
type Publication struct {
	Version    string
	Comparison Comparison
	// YAML is the document in the canonical layout, with info.version set to
	// Version.
	YAML []byte
	// LayoutVersion is the canonical layout YAML follows.
	LayoutVersion int
}

// Publish turns next into the Live version that follows live, or into a
// spec's first Live version when live is nil. It refuses an invalid document,
// a document identical to Live (ErrNothingToPublish), and a comparison that
// fails (ErrComparisonFailed).
func Publish(live *Live, next *Document) (Publication, error) {
	if err := next.Validate(); err != nil {
		return Publication{}, err
	}

	if live == nil {
		return publication(next, FirstVersion(next), Comparison{Bump: BumpInitial})
	}

	comparison, err := Compare(live.Document, next)
	if err != nil {
		return Publication{}, err
	}
	if comparison.Bump == BumpNone {
		return Publication{}, ErrNothingToPublish
	}
	version, err := NextVersion(live.Version, comparison.Bump)
	if err != nil {
		return Publication{}, err
	}
	return publication(next, version, comparison)
}

func publication(document *Document, version string, comparison Comparison) (Publication, error) {
	yaml, err := document.WithVersion(version).Canonical()
	if err != nil {
		return Publication{}, err
	}
	return Publication{Version: version, Comparison: comparison, YAML: yaml, LayoutVersion: LayoutVersion}, nil
}
