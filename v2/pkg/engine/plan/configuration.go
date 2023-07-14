package plan

type Configuration struct {
	DefaultFlushIntervalMillis int64
	DataSources                []DataSourceConfiguration
	Fields                     FieldConfigurations
	Types                      TypeConfigurations
	// DisableResolveFieldPositions should be set to true for testing purposes
	// This setting removes position information from all fields
	// In production, this should be set to false so that error messages are easier to understand
	DisableResolveFieldPositions bool
}

type TypeConfigurations []TypeConfiguration

func (t *TypeConfigurations) RenameTypeNameOnMatchStr(typeName string) string {
	for i := range *t {
		if (*t)[i].TypeName == typeName {
			return (*t)[i].RenameTo
		}
	}
	return typeName
}

func (t *TypeConfigurations) RenameTypeNameOnMatchBytes(typeName []byte) []byte {
	str := string(typeName)
	for i := range *t {
		if (*t)[i].TypeName == str {
			return []byte((*t)[i].RenameTo)
		}
	}
	return typeName
}

type TypeConfiguration struct {
	TypeName string
	// RenameTo modifies the TypeName
	// so that a downstream Operation can contain a different TypeName than the upstream Schema
	// e.g. if the downstream Operation contains { ... on Human_api { height } }
	// the upstream Operation can be rewritten to { ... on Human { height }}
	// by setting RenameTo to Human
	// This way, Types can be suffixed / renamed in downstream Schemas while keeping the contract with the upstream ok
	RenameTo string
}

type FieldConfigurations []FieldConfiguration

func (f FieldConfigurations) ForTypeField(typeName, fieldName string) *FieldConfiguration {
	for i := range f {
		if f[i].TypeName == typeName && f[i].FieldName == fieldName {
			return &f[i]
		}
	}
	return nil
}

func (f FieldConfigurations) IsKey(typeName, fieldName string) bool {
	for i := range f {
		if f[i].TypeName != typeName {
			continue
		}

		for j := range f[i].RequiresFields {
			if f[i].RequiresFields[j] == fieldName {
				return true
			}
		}
	}
	return false
}

func (f FieldConfigurations) Keys(typeName, fieldName string) (out []string) {
	keys := map[string]struct{}{}

	for i := range f {
		if f[i].TypeName != typeName {
			continue
		}

		for j := range f[i].RequiresFields {
			if f[i].RequiresFields[j] != fieldName {
				keys[f[i].RequiresFields[j]] = struct{}{}
			}
		}
	}

	for k := range keys {
		out = append(out, k)
	}

	return
}

type FieldConfiguration struct {
	TypeName  string
	FieldName string
	// DisableDefaultMapping - instructs planner whether to use path mapping coming from Path field
	DisableDefaultMapping bool
	// Path - represents a json path to lookup for a field value in response json
	Path           []string
	Arguments      ArgumentsConfigurations
	RequiresFields []string
	// UnescapeResponseJson set to true will allow fields (String,List,Object)
	// to be resolved from an escaped JSON string
	// e.g. {"response":"{\"foo\":\"bar\"}"} will be returned as {"foo":"bar"} when path is "response"
	// This way, it is possible to resolve a JSON string as part of the response without extra String encoding of the JSON
	UnescapeResponseJson bool
}

type ArgumentsConfigurations []ArgumentConfiguration

func (a ArgumentsConfigurations) ForName(argName string) *ArgumentConfiguration {
	for i := range a {
		if a[i].Name == argName {
			return &a[i]
		}
	}
	return nil
}

// SourceType is used to determine the source of an argument
type SourceType string

const (
	ObjectFieldSource   SourceType = "object_field"
	FieldArgumentSource SourceType = "field_argument"
)

// ArgumentRenderConfig is used to determine how an argument should be rendered
type ArgumentRenderConfig string

const (
	RenderArgumentDefault        ArgumentRenderConfig = ""
	RenderArgumentAsArrayCSV     ArgumentRenderConfig = "render_argument_as_array_csv"
	RenderArgumentAsGraphQLValue ArgumentRenderConfig = "render_argument_as_graphql_value"
	RenderArgumentAsJSONValue    ArgumentRenderConfig = "render_argument_as_json_value"
)

type ArgumentConfiguration struct {
	Name         string
	SourceType   SourceType
	SourcePath   []string
	RenderConfig ArgumentRenderConfig
	RenameTypeTo string
}
