package capability

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	"work/notification/code/pkg/notification/contract"
)

const channelPolicySchemaID = "https://notification.local/schema/channel_policy.json"

//go:embed message_capabilities/*/capability.json message_capabilities/*/schema.json shared_schemas/*.json
var defaultCapabilityFS embed.FS

type definition struct {
	Capability MessageCapability
	Schema     json.RawMessage
}

func All() ([]MessageCapability, error) {
	defs, err := loadAll(defaultCapabilityFS)
	if err != nil {
		return nil, err
	}
	items := make([]MessageCapability, 0, len(defs))
	for _, def := range defs {
		items = append(items, def.Capability)
	}
	return items, nil
}

func Get(messageType string) (*MessageCapability, error) {
	def, err := getDefinition(messageType)
	if err != nil {
		return nil, err
	}
	item := def.Capability
	return &item, nil
}

func ValidateConfig(item *MessageConfig) error {
	if item == nil {
		return fmt.Errorf("%w: message config is required", contract.ErrInvalidRequest)
	}
	if item.UpdatedBy != "" || !item.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: updated_by and updated_at are managed by the system", contract.ErrInvalidRequest)
	}

	def, err := getDefinition(item.MessageType)
	if err != nil {
		return err
	}
	schema, err := compileSchema(def.Schema, item.MessageType)
	if err != nil {
		return err
	}
	result, err := schema.Validate(gojsonschema.NewGoLoader(validationInputFromConfig(item)))
	if err != nil {
		return fmt.Errorf("%w: validate config schema for %s: %v", contract.ErrUnsupportedConfig, item.MessageType, err)
	}
	if result.Valid() {
		return nil
	}

	problems := make([]string, 0, len(result.Errors()))
	for _, item := range result.Errors() {
		problems = append(problems, item.String())
	}
	return fmt.Errorf("%w: config validation failed for %s: %s", contract.ErrUnsupportedConfig, item.MessageType, strings.Join(problems, "; "))
}

func loadAll(fsys fs.FS) ([]definition, error) {
	capabilityFiles, err := findCapabilityFiles(fsys)
	if err != nil {
		return nil, err
	}

	items := make([]definition, 0, len(capabilityFiles))
	seen := map[string]struct{}{}
	for _, capabilityPath := range capabilityFiles {
		schemaPath := filepath.ToSlash(filepath.Join(filepath.Dir(capabilityPath), "schema.json"))
		capabilityRaw, err := fs.ReadFile(fsys, capabilityPath)
		if err != nil {
			return nil, fmt.Errorf("%w: read capability %s: %v", contract.ErrUnsupportedConfig, capabilityPath, err)
		}
		schemaRaw, err := fs.ReadFile(fsys, schemaPath)
		if err != nil {
			return nil, fmt.Errorf("%w: read schema %s: %v", contract.ErrUnsupportedConfig, schemaPath, err)
		}

		var item MessageCapability
		if err := json.Unmarshal(capabilityRaw, &item); err != nil {
			return nil, fmt.Errorf("%w: parse capability %s: %v", contract.ErrUnsupportedConfig, capabilityPath, err)
		}
		if err := normalizeCapability(&item, capabilityPath); err != nil {
			return nil, err
		}
		if _, exists := seen[item.MessageType]; exists {
			return nil, fmt.Errorf("%w: duplicate capability for %s", contract.ErrUnsupportedConfig, item.MessageType)
		}
		if _, err := compileSchema(schemaRaw, item.MessageType); err != nil {
			return nil, err
		}
		seen[item.MessageType] = struct{}{}
		items = append(items, definition{Capability: item, Schema: schemaRaw})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Capability.MessageType < items[j].Capability.MessageType
	})
	return items, nil
}

func getDefinition(messageType string) (*definition, error) {
	if messageType == "" {
		return nil, fmt.Errorf("%w: message_type is required", contract.ErrInvalidRequest)
	}

	items, err := loadAll(defaultCapabilityFS)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Capability.MessageType == messageType {
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("%w: capability not found: %s", contract.ErrUnsupportedConfig, messageType)
}

func findCapabilityFiles(fsys fs.FS) ([]string, error) {
	var files []string
	err := fs.WalkDir(fsys, "message_capabilities", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == "capability.json" {
			files = append(files, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func normalizeCapability(item *MessageCapability, source string) error {
	if item == nil {
		return fmt.Errorf("%w: capability is required", contract.ErrInvalidRequest)
	}
	if item.MessageType == "" {
		return fmt.Errorf("%w: message_type is required in %s", contract.ErrUnsupportedConfig, source)
	}
	if !item.RealtimeSupported && !item.AggregateSupported {
		return fmt.Errorf("%w: at least one capability is required for %s", contract.ErrUnsupportedConfig, item.MessageType)
	}
	if len(item.Channels) == 0 {
		return fmt.Errorf("%w: channels are required for %s", contract.ErrUnsupportedConfig, item.MessageType)
	}

	seen := map[string]struct{}{}
	for i := range item.Channels {
		channel := strings.TrimSpace(item.Channels[i].Channel)
		if channel == "" {
			return fmt.Errorf("%w: channel is required for %s", contract.ErrUnsupportedConfig, item.MessageType)
		}
		if _, exists := seen[channel]; exists {
			return fmt.Errorf("%w: duplicate channel %s for %s", contract.ErrUnsupportedConfig, channel, item.MessageType)
		}
		seen[channel] = struct{}{}
		item.Channels[i].Channel = channel
	}
	return nil
}

func validationInputFromConfig(item *MessageConfig) validationInput {
	return validationInput{
		TenantID:               item.TenantID,
		MessageType:            item.MessageType,
		RealtimeEnabled:        item.RealtimeEnabled,
		AggregateEnabled:       item.AggregateEnabled,
		AggregatePeriodMinutes: item.AggregatePeriodMinutes,
		Filter:                 item.Filter,
		Channel:                item.Channel,
	}
}

func compileSchema(schemaRaw json.RawMessage, messageType string) (*gojsonschema.Schema, error) {
	sharedRaw, err := fs.ReadFile(defaultCapabilityFS, "shared_schemas/channel_policy.schema.json")
	if err != nil {
		return nil, fmt.Errorf("%w: read shared channel policy schema: %v", contract.ErrUnsupportedConfig, err)
	}

	loader := gojsonschema.NewSchemaLoader()
	loader.Draft = gojsonschema.Draft7
	loader.AutoDetect = false
	if err := loader.AddSchema(channelPolicySchemaID, gojsonschema.NewStringLoader(string(sharedRaw))); err != nil {
		return nil, fmt.Errorf("%w: load shared channel policy schema for %s: %v", contract.ErrUnsupportedConfig, messageType, err)
	}

	schemaLoader := gojsonschema.NewStringLoader(string(schemaRaw))
	schema, err := loader.Compile(schemaLoader)
	if err != nil {
		return nil, fmt.Errorf("%w: compile config schema for %s: %v", contract.ErrUnsupportedConfig, messageType, err)
	}
	return schema, nil
}

type validationInput struct {
	TenantID               string        `json:"tenant_id,omitempty"`
	MessageType            string        `json:"message_type"`
	RealtimeEnabled        bool          `json:"realtime_enabled"`
	AggregateEnabled       bool          `json:"aggregate_enabled"`
	AggregatePeriodMinutes int           `json:"aggregate_period_minutes,omitempty"`
	Filter                 json.RawMessage `json:"filter,omitempty"`
	Channel                ChannelPolicy `json:"channel"`
}
