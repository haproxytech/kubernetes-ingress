package validators

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	"gopkg.in/yaml.v3"
)

// AnnType defines the type of annotation used in the validation rules.
type AnnType string

const (
	DurationType AnnType = "duration" // DurationType represents a duration type
	IntType      AnnType = "int"      // IntType represents an integer type
	UintType     AnnType = "uint"     // UintType represents an unsigned integer type
	BoolType     AnnType = "bool"     // BoolType represents a boolean type
	StringType   AnnType = "string"   // StringType represents a string type
	FloatType    AnnType = "float"    // FloatType represents a float type
	JSONType     AnnType = "json"     // JSONType represents a JSON type
)

// Rule defines the structure for a single validation rule in the YAML.
type Rule struct {
	// +kubebuilder:validation:Enum=duration;int;uint;bool;string;float;json;
	Type AnnType `yaml:"type" json:"type"` // Expected data type (e.g., "duration", "int", "bool")
	Rule string  `yaml:"rule" json:"rule"`
	// Optional priority for sorting rules, bigger value has higher priority
	OrderPriority int    `yaml:"order_priority,omitempty" json:"order_priority,omitempty"`
	Template      string `yaml:"template,omitempty" json:"template,omitempty"` // Optional pattern
	// +kubebuilder:validation:Enum=frontend;backend;all
	// +kubebuilder:default=backend
	Section string `yaml:"section" json:"section,omitempty"`
	// resources where this rule can apply (service, frontend)
	// +kubebuilder:validation:MaxItems=42
	Resources []string `yaml:"resources,omitempty" json:"resources,omitempty"`
	// namespaces where this rule can apply (namespace names)
	Namespaces []string `yaml:"namespaces,omitempty" json:"namespaces,omitempty"`
	// ingresses where this rule can apply (ingress names)
	Ingresses []string `yaml:"ingresses,omitempty" json:"ingresses,omitempty"`
}

// Config defines the structure for the entire YAML configuration file.
type Config struct {
	ValidationRules map[string]Rule `yaml:"validation_rules" json:"validation_rules"` //nolint:tagalign
}

type Validator struct {
	env               *cel.Env
	compiledRules     map[string]cel.Program
	config            Config
	prefix            string
	prefixes          []string
	templateVariables map[string]any
}

var (
	validator Validator
	once      sync.Once
	mu        sync.RWMutex
)

var annotationsPrefixes = []string{
	"haproxy.org/",
	"ingress.kubernetes.io/",
	"haproxy.com/",
}

// Get initializes the Validator instance and returns a pointer to it.
func Get() (*Validator, error) {
	var err error
	once.Do(func() {
		validator = Validator{
			compiledRules:     make(map[string]cel.Program),
			templateVariables: make(map[string]any),
		}
		validator.templateVariables["POD_NAME"] = os.Getenv("POD_NAME")
		validator.templateVariables["POD_NAMESPACE"] = os.Getenv("POD_NAMESPACE")
		validator.templateVariables["POD_IP"] = os.Getenv("POD_IP")
		// validator.Init(example)
	})

	return &validator, err
}

// Init initializes the Validator with the provided YAML configuration data.
func (v *Validator) Init(data []byte) error {
	// load configuration
	// Unmarshal the YAML data into the Config struct.
	if err := yaml.Unmarshal(data, &validator.config); err != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return v.Set(validator.prefix, validator.config)
}

// Prefix returns the prefix used for the validation rules.
func (v *Validator) Prefix() string {
	return v.prefix
}

// SetPrefix sets the prefix used for the validation rules.
func (v *Validator) SetPrefix(prefix string) {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	v.prefix = prefix
}

// Prefixes returns the list of prefixes used for the validation rules.
func (v *Validator) Prefixes() []string {
	return v.prefixes
}

// Set initializes the Validator with the provided configuration data.
func (v *Validator) Set(prefix string, config Config) error {
	if prefix == "" {
		return errors.New("prefix cannot be empty")
	}
	mu.Lock()
	defer mu.Unlock()
	for name, rule := range config.ValidationRules {
		// Validate rule type
		switch rule.Type {
		case JSONType:
			rule.Template = strings.TrimSuffix(rule.Template, "\n")
			config.ValidationRules[name] = rule // Update the rule in the config
		default:
		}
	}
	validator.config = config
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	validator.prefix = prefix
	validator.prefixes = append(annotationsPrefixes, prefix) //nolint:gocritic
	// Define the CEL environment options.
	// We declare a variable 'value' of type 'any' to represent the input value
	// being validated.
	// We include 'cel.DurationType' to enable parsing and operations on durations.
	// 'cel.StdLib()' provides common functions.
	envOptions := []cel.EnvOption{
		cel.Variable("value", cel.AnyType),
		cel.Types(cel.DurationType),
		cel.StdLib(),
	}

	// Create a new CEL environment.
	env, err := cel.NewEnv(envOptions...)
	if err != nil {
		return fmt.Errorf("failed to create CEL environment: %w", err)
	}
	v.env = env

	returnErrors := []string{}

	// Iterate through each rule defined in the configuration.
	for name, rule := range v.config.ValidationRules {
		if rule.Section == "" {
			rule.Section = "backend"
			v.config.ValidationRules[name] = rule
		}
		// Compile the CEL expression string into an AST (Abstract Syntax Tree).
		ast, issues := env.Compile(rule.Rule)
		if issues != nil && issues.Err() != nil {
			// return fmt.Errorf("failed to compile CEL rule '%s': %w", name, issues.Err())
			returnErrors = append(returnErrors, fmt.Sprintf("failed to compile CEL rule '%s': %v", name, issues.Err()))
			continue
		}

		// Create a CEL program from the AST. This program is ready for evaluation.
		prog, err := env.Program(ast)
		if err != nil {
			// return fmt.Errorf("failed to create CEL program for '%s': %w", name, err)
			returnErrors = append(returnErrors, fmt.Sprintf("failed to create CEL program for '%s': %v", name, err))
			continue
		}
		// Store the compiled program in our global map for quick access.
		v.compiledRules[name] = prog
		// log.Printf("Successfully compiled CEL rule: %s", name)
	}
	if len(returnErrors) > 0 {
		return fmt.Errorf("errors occurred during validation rule compilation: %s", strings.Join(returnErrors, "; "))
	}
	return nil
}

// ValidateInput takes a rule name and a raw string value,
// parses the value according to the rule's type,
// and evaluates it against the corresponding CEL expression.
func (v *Validator) ValidateInput(ruleName string, rawValue string, section string) error {
	mu.RLock()
	defer mu.RUnlock()
	// Retrieve the rule definition from the loaded configuration.
	rule, ok := v.config.ValidationRules[ruleName]
	if !ok {
		return fmt.Errorf("no validation rule found for '%s'", ruleName)
	}

	// Check if the rule applies to the specified section.
	sectionFound := rule.Section == section || rule.Section == "all"
	if !sectionFound {
		// If the rule has sections defined and the current section is not one of them,
		// we skip validation for this rule.
		return fmt.Errorf("no validation rule found for '%s' in section '%s'", ruleName, section)
	}

	// Retrieve the pre-compiled CEL program for this rule.
	prog, ok := v.compiledRules[ruleName]
	if !ok {
		return fmt.Errorf("CEL program for '%s' not found (this indicates an initialization error)", ruleName)
	}

	var celValue any
	var err error

	// Convert the raw string value into the appropriate CEL type based on the rule's 'Type'.
	switch rule.Type {
	case DurationType:
		// Parse the string into a Go time.Duration.
		goDuration, parseErr := time.ParseDuration(rawValue)
		if parseErr != nil {
			return fmt.Errorf("invalid duration format for '%s': %w", rawValue, parseErr)
		}
		// Convert Go time.Duration to protobuf duration.
		celValue = durationpb.New(goDuration)
	case IntType:
		// Parse the string into an int64.
		intValue, parseErr := strconv.ParseInt(rawValue, 10, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid integer format for '%s': %w", rawValue, parseErr)
		}
		// Convert int64 to CEL's types.Int.
		celValue = types.Int(intValue)
	case UintType:
		// Parse the string into a uint64.
		uintValue, parseErr := strconv.ParseUint(rawValue, 10, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid unsigned integer format for '%s': %w", rawValue, parseErr)
		}
		// Convert uint64 to CEL's types.Uint.
		celValue = types.Uint(uintValue)
	case BoolType:
		// Parse the string into a boolean.
		boolValue, parseErr := strconv.ParseBool(rawValue)
		if parseErr != nil {
			return fmt.Errorf("invalid boolean format for '%s': %w", rawValue, parseErr)
		}
		// Convert bool to CEL's types.Bool.
		celValue = types.Bool(boolValue)
	case StringType:
		celValue = rawValue // No conversion needed, just use the string directly.
	case FloatType:
		// Parse the string into a float64.
		floatValue, parseErr := strconv.ParseFloat(rawValue, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid float format for '%s': %w", rawValue, parseErr)
		}
		// Convert float64 to CEL's types.Double.
		celValue = types.Double(floatValue)
	case JSONType:
		// Parse the raw JSON string into a map.
		var jsonValue map[string]any
		if err := json.Unmarshal([]byte(rawValue), &jsonValue); err != nil {
			return fmt.Errorf("invalid JSON format for '%s': %w", rawValue, err)
		}
		// Convert the map to a CEL-compatible type.
		celValue = types.DefaultTypeAdapter.NativeToValue(jsonValue)
	default:
		return fmt.Errorf("unsupported rule type: %s for rule '%s'", rule.Type, ruleName)
	}

	// Evaluate the CEL program with the 'value' variable set to our converted input.
	// The evaluation result will be a boolean (true if validation passes, false otherwise)
	// and potentially an error.
	evalResult, _, err := prog.Eval(map[string]any{"value": celValue})
	if err != nil {
		return fmt.Errorf("CEL evaluation failed for rule '%s' with value '%s': %w", ruleName, rawValue, err)
	}

	// Check the evaluation result. It must be a boolean and true for validation to pass.
	if resultBool, ok := evalResult.Value().(bool); !ok {
		return fmt.Errorf("validation rule '%s' for value '%s' did not return a boolean, but %v (%T). Rule: '%s'", ruleName, rawValue, evalResult.Value(), evalResult.Value(), rule.Rule)
	} else if !resultBool {
		// Try to find the failing sub-expression for simple conjunctions.
		parts := strings.Split(rule.Rule, "&&")
		if len(parts) > 1 && v.env != nil {
			for _, part := range parts {
				part = strings.TrimSpace(part)
				ast, issues := v.env.Compile(part)
				if issues != nil && issues.Err() != nil {
					break
				}
				prog, err := v.env.Program(ast)
				if err != nil {
					break
				}
				res, _, err := prog.Eval(map[string]any{"value": celValue})
				if err != nil {
					break
				}
				if boolRes, ok := res.Value().(bool); ok && !boolRes {
					return fmt.Errorf("validation failed for rule '%s' with value '%s'. \nFailed part: '%s'", ruleName, rawValue, part)
				}
			}
		}
		return fmt.Errorf("validation failed for rule '%s' with value '%s'", ruleName, rawValue)
	}

	// If we reach here, validation was successful.
	return nil
}

// GetResult generates a result string based on the validation rule's template.
func (v *Validator) GetResult(ruleName string, value string, envValues map[string]any) (string, error) {
	mu.RLock()
	defer mu.RUnlock()
	rule, ok := v.config.ValidationRules[ruleName]
	if !ok {
		return "", fmt.Errorf("no validation rule found for '%s'", ruleName)
	}
	// Check if the rule has a pattern defined
	if rule.Template == "" {
		return ruleName + " " + value, nil
	}
	if envValues == nil {
		envValues = make(map[string]any)
	}
	for k, val := range v.templateVariables {
		envValues[k] = val
	}

	if len(envValues) > 0 {
		rule.Template = strings.ReplaceAll(rule.Template, "{{.}}", "{{.value}}")
	}
	// If a pattern is defined, use templating to fill it out
	tmpl, err := template.New("config").Parse(rule.Template)
	if err != nil {
		return "", fmt.Errorf("failed to parse template for rule '%s': %w", ruleName, err)
	}

	var data any
	if rule.Type == JSONType {
		// If the rule type is JSON, parse the value as JSON
		var jsonData map[string]any
		if err := json.Unmarshal([]byte(value), &jsonData); err != nil {
			return "", fmt.Errorf("failed to unmarshal JSON for rule '%s': %w", ruleName, err)
		}
		data = jsonData
		// Merge envValues into jsonData
		for k, v := range envValues {
			jsonData[k] = v
		}
	} else {
		data = value
		if len(envValues) > 0 {
			// If there are additional environment values, create a map to hold them
			dataMap := map[string]any{
				"value": value, // The main value is accessible via "."
			}
			for k, v := range envValues {
				dataMap[k] = v
			}
			data = dataMap
		}
	}

	var result bytes.Buffer
	if err := tmpl.Execute(&result, data); err != nil {
		return "", fmt.Errorf("failed to execute template for rule '%s': %w", ruleName, err)
	}

	return result.String(), nil
}

// FilterValues is a struct that holds the filter values for the GetSortedAnnotationKeys method
type FilterValues struct {
	Section   string
	Frontend  string
	Backend   string
	Namespace string
	Ingress   string
	Service   string
}

// GetSortedAnnotationKeys generates a result string based on the validation rule's template.
func (v *Validator) GetSortedAnnotationKeys(annotations map[string]string, filters FilterValues) []string {
	mu.RLock()
	defer mu.RUnlock()
	keys := []string{}
	for k := range annotations {
		rule, ok := v.config.ValidationRules[k]
		if !ok {
			continue
		}
		// Check if the rule applies to the specified section.
		if !(rule.Section == filters.Section || rule.Section == "all") {
			continue
		}
		// service can be Service, Frontend or Backend name
		if len(rule.Resources) > 0 {
			found := slices.Contains(rule.Resources, filters.Service)
			if !found && filters.Frontend != "" {
				found = slices.Contains(rule.Resources, filters.Frontend)
			}
			if !found && filters.Backend != "" {
				found = slices.Contains(rule.Resources, filters.Backend)
			}
			if !found {
				continue
			}
		}

		if len(rule.Ingresses) > 0 {
			found := slices.Contains(rule.Ingresses, filters.Ingress)
			if !found {
				continue
			}
		}

		if len(rule.Namespaces) > 0 && filters.Namespace != "" {
			found := slices.Contains(rule.Namespaces, filters.Namespace)
			if !found {
				continue
			}
		}

		keys = append(keys, k)
	}
	// now sort the keys based on SortPriority, if same priority, sort alphabetically
	sort.SliceStable(keys, func(i, j int) bool {
		ruleI := v.config.ValidationRules[keys[i]]
		ruleJ := v.config.ValidationRules[keys[j]]
		if ruleI.OrderPriority == ruleJ.OrderPriority {
			return keys[i] < keys[j]
		}
		return ruleI.OrderPriority > ruleJ.OrderPriority
	})

	return keys
}
