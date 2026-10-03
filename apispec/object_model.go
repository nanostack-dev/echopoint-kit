package apispec

type objectKind int

const (
	kindAny objectKind = iota
	kindRoot
	kindInfo
	kindContact
	kindLicense
	kindServer
	kindServerVariable
	kindComponents
	kindPathItem
	kindOperation
	kindParameter
	kindRequestBody
	kindMediaType
	kindEncoding
	kindResponse
	kindHeader
	kindExample
	kindLink
	kindTag
	kindExternalDocs
	kindSchema
	kindDiscriminator
	kindXML
	kindSecurityScheme
	kindOAuthFlows
	kindOAuthFlow
)

type containerKind int

const (
	containerSingle containerKind = iota
	containerMap
	containerExtensibleMap
	containerList
)

type valueShape struct {
	kind      objectKind
	container containerKind
}

type objectField struct {
	name  string
	shape valueShape
}

func object(kind objectKind) valueShape {
	return valueShape{kind: kind, container: containerSingle}
}

func mapOf(kind objectKind) valueShape {
	return valueShape{kind: kind, container: containerMap}
}

func extensibleMapOf(kind objectKind) valueShape {
	return valueShape{kind: kind, container: containerExtensibleMap}
}

func listOf(kind objectKind) valueShape {
	return valueShape{kind: kind, container: containerList}
}

func field(name string, shape valueShape) objectField {
	return objectField{name: name, shape: shape}
}

func scalar(name string) objectField {
	return objectField{name: name, shape: object(kindAny)}
}

// objectFields lists the fixed fields of each OpenAPI object in the order the
// canonical layout writes them. A key that is not listed (an extension, or a
// field this table does not know) is written after them, sorted by name.
//
//nolint:funlen // one table, read top to bottom
func objectFields(kind objectKind) []objectField {
	switch kind {
	case kindRoot:
		return []objectField{
			scalar("openapi"),
			field("info", object(kindInfo)),
			scalar("jsonSchemaDialect"),
			field("servers", listOf(kindServer)),
			field("paths", extensibleMapOf(kindPathItem)),
			field("webhooks", mapOf(kindPathItem)),
			field("components", object(kindComponents)),
			scalar("security"),
			field("tags", listOf(kindTag)),
			field("externalDocs", object(kindExternalDocs)),
		}
	case kindInfo:
		return []objectField{
			scalar("title"),
			scalar("summary"),
			scalar("description"),
			scalar("termsOfService"),
			field("contact", object(kindContact)),
			field("license", object(kindLicense)),
			scalar("version"),
		}
	case kindContact:
		return []objectField{scalar("name"), scalar("url"), scalar("email")}
	case kindLicense:
		return []objectField{scalar("name"), scalar("identifier"), scalar("url")}
	case kindServer:
		return []objectField{
			scalar("url"),
			scalar("description"),
			field("variables", mapOf(kindServerVariable)),
		}
	case kindServerVariable:
		return []objectField{scalar("enum"), scalar("default"), scalar("description")}
	case kindComponents:
		return []objectField{
			field("schemas", mapOf(kindSchema)),
			field("responses", mapOf(kindResponse)),
			field("parameters", mapOf(kindParameter)),
			field("examples", mapOf(kindExample)),
			field("requestBodies", mapOf(kindRequestBody)),
			field("headers", mapOf(kindHeader)),
			field("securitySchemes", mapOf(kindSecurityScheme)),
			field("links", mapOf(kindLink)),
			field("callbacks", extensibleMapOf(kindPathItem)),
			field("pathItems", mapOf(kindPathItem)),
		}
	case kindPathItem:
		operation := object(kindOperation)
		return []objectField{
			scalar("summary"),
			scalar("description"),
			field("servers", listOf(kindServer)),
			field("parameters", listOf(kindParameter)),
			field("get", operation),
			field("put", operation),
			field("post", operation),
			field("delete", operation),
			field("options", operation),
			field("head", operation),
			field("patch", operation),
			field("trace", operation),
		}
	case kindOperation:
		return []objectField{
			scalar("tags"),
			scalar("summary"),
			scalar("description"),
			field("externalDocs", object(kindExternalDocs)),
			scalar("operationId"),
			field("parameters", listOf(kindParameter)),
			field("requestBody", object(kindRequestBody)),
			field("responses", extensibleMapOf(kindResponse)),
			field("callbacks", extensibleMapOf(kindPathItem)),
			scalar("deprecated"),
			scalar("security"),
			field("servers", listOf(kindServer)),
		}
	case kindParameter:
		return []objectField{
			scalar("name"),
			scalar("in"),
			scalar("description"),
			scalar("required"),
			scalar("deprecated"),
			scalar("allowEmptyValue"),
			scalar("style"),
			scalar("explode"),
			scalar("allowReserved"),
			field("schema", object(kindSchema)),
			scalar("example"),
			field("examples", mapOf(kindExample)),
			field("content", mapOf(kindMediaType)),
		}
	case kindRequestBody:
		return []objectField{scalar("description"), field("content", mapOf(kindMediaType)), scalar("required")}
	case kindMediaType:
		return []objectField{
			field("schema", object(kindSchema)),
			scalar("example"),
			field("examples", mapOf(kindExample)),
			field("encoding", mapOf(kindEncoding)),
		}
	case kindEncoding:
		return []objectField{
			scalar("contentType"),
			field("headers", mapOf(kindHeader)),
			scalar("style"),
			scalar("explode"),
			scalar("allowReserved"),
		}
	case kindResponse:
		return []objectField{
			scalar("description"),
			field("headers", mapOf(kindHeader)),
			field("content", mapOf(kindMediaType)),
			field("links", mapOf(kindLink)),
		}
	case kindHeader:
		return []objectField{
			scalar("description"),
			scalar("required"),
			scalar("deprecated"),
			scalar("allowEmptyValue"),
			scalar("style"),
			scalar("explode"),
			scalar("allowReserved"),
			field("schema", object(kindSchema)),
			scalar("example"),
			field("examples", mapOf(kindExample)),
			field("content", mapOf(kindMediaType)),
		}
	case kindExample:
		return []objectField{scalar("summary"), scalar("description"), scalar("value"), scalar("externalValue")}
	case kindLink:
		return []objectField{
			scalar("operationRef"),
			scalar("operationId"),
			scalar("parameters"),
			scalar("requestBody"),
			scalar("description"),
			field("server", object(kindServer)),
		}
	case kindTag:
		return []objectField{scalar("name"), scalar("description"), field("externalDocs", object(kindExternalDocs))}
	case kindExternalDocs:
		return []objectField{scalar("description"), scalar("url")}
	case kindSchema:
		return schemaFields()
	case kindDiscriminator:
		return []objectField{scalar("propertyName"), scalar("mapping")}
	case kindXML:
		return []objectField{
			scalar("name"),
			scalar("namespace"),
			scalar("prefix"),
			scalar("attribute"),
			scalar("wrapped"),
		}
	case kindSecurityScheme:
		return []objectField{
			scalar("type"),
			scalar("description"),
			scalar("name"),
			scalar("in"),
			scalar("scheme"),
			scalar("bearerFormat"),
			field("flows", object(kindOAuthFlows)),
			scalar("openIdConnectUrl"),
		}
	case kindOAuthFlows:
		flow := object(kindOAuthFlow)
		return []objectField{
			field("implicit", flow),
			field("password", flow),
			field("clientCredentials", flow),
			field("authorizationCode", flow),
		}
	case kindOAuthFlow:
		return []objectField{scalar("authorizationUrl"), scalar("tokenUrl"), scalar("refreshUrl"), scalar("scopes")}
	case kindAny:
		return nil
	}
	return nil
}

// schemaFields orders a Schema Object for reading: identity, then what the
// value is, then its structure, then its limits, then examples.
func schemaFields() []objectField {
	return []objectField{
		scalar("$id"),
		scalar("$schema"),
		scalar("$anchor"),
		scalar("$dynamicRef"),
		scalar("$dynamicAnchor"),
		scalar("$comment"),
		scalar("title"),
		scalar("summary"),
		scalar("description"),
		scalar("type"),
		scalar("format"),
		scalar("contentMediaType"),
		scalar("contentEncoding"),
		field("contentSchema", object(kindSchema)),
		scalar("enum"),
		scalar("const"),
		scalar("default"),
		scalar("nullable"),
		scalar("readOnly"),
		scalar("writeOnly"),
		scalar("deprecated"),
		field("discriminator", object(kindDiscriminator)),
		field("allOf", listOf(kindSchema)),
		field("oneOf", listOf(kindSchema)),
		field("anyOf", listOf(kindSchema)),
		field("not", object(kindSchema)),
		field("if", object(kindSchema)),
		field("then", object(kindSchema)),
		field("else", object(kindSchema)),
		scalar("required"),
		field("properties", mapOf(kindSchema)),
		field("patternProperties", mapOf(kindSchema)),
		field("additionalProperties", object(kindSchema)),
		field("unevaluatedProperties", object(kindSchema)),
		field("propertyNames", object(kindSchema)),
		scalar("minProperties"),
		scalar("maxProperties"),
		scalar("dependentRequired"),
		field("dependentSchemas", mapOf(kindSchema)),
		field("prefixItems", listOf(kindSchema)),
		field("items", object(kindSchema)),
		field("contains", object(kindSchema)),
		scalar("minContains"),
		scalar("maxContains"),
		field("unevaluatedItems", object(kindSchema)),
		scalar("minItems"),
		scalar("maxItems"),
		scalar("uniqueItems"),
		scalar("minimum"),
		scalar("exclusiveMinimum"),
		scalar("maximum"),
		scalar("exclusiveMaximum"),
		scalar("multipleOf"),
		scalar("minLength"),
		scalar("maxLength"),
		scalar("pattern"),
		field("$defs", mapOf(kindSchema)),
		field("definitions", mapOf(kindSchema)),
		field("xml", object(kindXML)),
		field("externalDocs", object(kindExternalDocs)),
		scalar("example"),
		scalar("examples"),
	}
}
