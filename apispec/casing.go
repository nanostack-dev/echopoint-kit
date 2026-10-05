package apispec

import "regexp"

type casing string

const (
	casingSnake     casing = "snake"
	casingCamel     casing = "camel"
	casingPascal    casing = "pascal"
	casingKebab     casing = "kebab"
	casingScreaming casing = "screaming"
	casingLower     casing = "lower"
	casingOther     casing = "other"
)

var (
	lowerName     = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	snakeName     = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)+$`)
	kebabName     = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)+$`)
	camelName     = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[A-Z][a-z0-9]*)+$`)
	pascalName    = regexp.MustCompile(`^[A-Z][a-z0-9]+(?:[A-Z][a-z0-9]*)*$`)
	screamingName = regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+$`)
)

func classifyCasing(name string) casing {
	switch {
	case lowerName.MatchString(name):
		return casingLower
	case snakeName.MatchString(name):
		return casingSnake
	case kebabName.MatchString(name):
		return casingKebab
	case camelName.MatchString(name):
		return casingCamel
	case pascalName.MatchString(name):
		return casingPascal
	case screamingName.MatchString(name):
		return casingScreaming
	default:
		return casingOther
	}
}

func (c casing) isNamed() bool {
	return c != casingLower && c != casingOther
}

func (c casing) label() string {
	switch c {
	case casingSnake:
		return "snake_case"
	case casingCamel:
		return "camelCase"
	case casingPascal:
		return "PascalCase"
	case casingKebab:
		return "kebab-case"
	case casingScreaming:
		return "SCREAMING_CASE"
	case casingLower, casingOther:
		return string(c)
	}
	return string(c)
}
