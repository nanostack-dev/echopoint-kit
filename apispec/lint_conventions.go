package apispec

import (
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	majority                    = 0.8
	propertyDescriptionMajority = 0.7
	minTagOperations            = 3
)

type casingConvention struct {
	casing casing
	share  float64
	total  int
}

type shareConvention struct {
	share float64
	total int
}

type operationProfile struct {
	count           int
	statusCodes     map[string]int
	extensions      map[string]int
	withSecurity    int
	withDescription int
}

func newOperationProfile() *operationProfile {
	return &operationProfile{statusCodes: make(map[string]int), extensions: make(map[string]int)}
}

func (p *operationProfile) add(operation *yaml.Node) {
	p.count++
	for _, response := range mappingOf(lookup(operation, "responses")) {
		p.statusCodes[response.key.Value]++
	}
	for _, entry := range mappingOf(operation) {
		if strings.HasPrefix(entry.key.Value, extensionPrefix) {
			p.extensions[entry.key.Value]++
		}
	}
	if lookup(operation, "security") != nil {
		p.withSecurity++
	}
	if isNonBlankString(lookup(operation, "description")) {
		p.withDescription++
	}
}

func (p *operationProfile) share(count int) float64 {
	return float64(count) / float64(p.count)
}

func isErrorStatus(status string) bool {
	return status == "default" || strings.HasPrefix(status, "4") || strings.HasPrefix(status, "5")
}

func dominantCasing(names []string) *casingConvention {
	counts := make(map[casing]int)
	total := 0
	for _, name := range names {
		if kind := classifyCasing(name); kind.isNamed() {
			total++
			counts[kind]++
		}
	}
	var best casing
	bestCount := 0
	for kind, count := range counts {
		if count > bestCount {
			best, bestCount = kind, count
		}
	}
	if total == 0 || float64(bestCount)/float64(total) < majority {
		return nil
	}
	return &casingConvention{casing: best, share: float64(bestCount) / float64(total), total: total}
}

type baseline struct {
	tagProfiles       map[string]*operationProfile
	overall           *operationProfile
	propertyCasing    *casingConvention
	operationIDCasing *casingConvention
	propertyShare     shareConvention
	errorShape        shareConvention
}

func newBaseline(document *Document) *baseline {
	result := &baseline{
		tagProfiles: make(map[string]*operationProfile),
		overall:     newOperationProfile(),
	}
	operations := collectOperations(document.root)
	operationIDs := make([]string, 0, len(operations))
	for _, operation := range operations {
		profile := result.tagProfiles[operation.tag]
		if profile == nil {
			profile = newOperationProfile()
			result.tagProfiles[operation.tag] = profile
		}
		profile.add(operation.node)
		result.overall.add(operation.node)
		if id, isString := stringValue(lookup(operation.node, "operationId")); isString {
			operationIDs = append(operationIDs, id)
		}
	}
	result.operationIDCasing = dominantCasing(operationIDs)
	result.learnProperties(componentSchemaProperties(document.root))
	result.learnErrorShape(operations)
	return result
}

func (b *baseline) learnProperties(properties []propertyRecord) {
	names := make([]string, 0, len(properties))
	candidates, described := 0, 0
	for _, property := range properties {
		names = append(names, property.name)
		if property.referenceOnly {
			continue
		}
		candidates++
		if property.described {
			described++
		}
	}
	b.propertyCasing = dominantCasing(names)
	if candidates > 0 {
		b.propertyShare = shareConvention{share: float64(described) / float64(candidates), total: candidates}
	}
}

func (b *baseline) learnErrorShape(operations []operationRecord) {
	conforming, total := 0, 0
	for _, operation := range operations {
		for _, response := range mappingOf(lookup(operation.node, "responses")) {
			if !isErrorStatus(response.key.Value) {
				continue
			}
			total++
			if isSharedErrorShape(response.value) {
				conforming++
			}
		}
	}
	if total > 0 {
		b.errorShape = shareConvention{share: float64(conforming) / float64(total), total: total}
	}
}

func (b *baseline) profileFor(tag string) (*operationProfile, string) {
	if profile := b.tagProfiles[tag]; profile != nil && profile.count >= minTagOperations {
		return profile, tag
	}
	return b.overall, ""
}

func isSharedErrorShape(response *yaml.Node) bool {
	if _, isReference := stringValue(lookup(response, refKey)); isReference {
		return true
	}
	for _, media := range mappingOf(lookup(response, "content")) {
		if _, isReference := stringValue(lookup(lookup(media.value, "schema"), refKey)); isReference {
			return true
		}
	}
	return false
}

func sortedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
