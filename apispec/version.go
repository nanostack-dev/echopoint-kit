package apispec

import (
	"fmt"
	"regexp"
	"strconv"
)

// Bump is how far a new Live version moves from the previous one.
type Bump string

const (
	// BumpNone means the document is identical to the Live version.
	BumpNone Bump = "none"
	// BumpPatch covers descriptions, examples, extensions, or other edits clients
	// do not depend on.
	BumpPatch Bump = "patch"
	// BumpMinor covers new operations, schemas, optional fields, or enum values, or
	// looser limits.
	BumpMinor Bump = "minor"
	// BumpMajor means at least one change breaks clients or may break them.
	BumpMajor Bump = "major"
	// BumpInitial marks the first Live version of a spec.
	BumpInitial Bump = "initial"
)

// InitialVersion is the first Live version of a spec whose info.version is
// not a semantic version.
const InitialVersion = "1.0.0"

var semanticVersion = regexp.MustCompile(
	`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
		`(?:-(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*)?` +
		`(?:\+[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*)?$`,
)

func bumpRank(bump Bump) int {
	switch bump {
	case BumpPatch:
		return 1
	case BumpMinor:
		return 2 //nolint:mnd // ranks are an order, not quantities
	case BumpMajor:
		return 3 //nolint:mnd // ranks are an order, not quantities
	case BumpNone, BumpInitial:
		return 0
	}
	return 0
}

func bumpOf(severity Severity) Bump {
	switch severity {
	case SeverityBreaking, SeverityRisky:
		return BumpMajor
	case SeverityAdditive:
		return BumpMinor
	case SeverityEdit:
		return BumpPatch
	}
	return BumpPatch
}

// bumpFor is the highest bump any of the changes calls for.
func bumpFor(changes []Change) Bump {
	highest := BumpNone
	for _, change := range changes {
		if bump := bumpOf(change.Severity); bumpRank(bump) > bumpRank(highest) {
			highest = bump
		}
	}
	return highest
}

// FirstVersion is the version of a spec's first Live version: the document's
// info.version when it is a semantic version, else InitialVersion.
func FirstVersion(document *Document) string {
	if semanticVersion.MatchString(document.Version()) {
		return document.Version()
	}
	return InitialVersion
}

// NextVersion applies a bump to a semantic version. Pre-release and build
// metadata are dropped: 1.4.2-beta with a minor bump is 1.5.0.
func NextVersion(previous string, bump Bump) (string, error) {
	match := semanticVersion.FindStringSubmatch(previous)
	if match == nil {
		return "", fmt.Errorf("%q is not a semantic version", previous)
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])

	switch bump {
	case BumpMajor:
		return fmt.Sprintf("%d.0.0", major+1), nil
	case BumpMinor:
		return fmt.Sprintf("%d.%d.0", major, minor+1), nil
	case BumpPatch:
		return fmt.Sprintf("%d.%d.%d", major, minor, patch+1), nil
	case BumpNone, BumpInitial:
	}
	return "", fmt.Errorf("a %s bump does not produce a next version", bump)
}
