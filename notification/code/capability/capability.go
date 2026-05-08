package capability

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"work/notification/code/contract"
	"work/notification/code/model"
)

//go:embed message_capabilities/*.json
var defaultCapabilityFS embed.FS

func All() ([]model.MessageCapability, error) {
	return loadAll(defaultCapabilityFS)
}

func Get(messageType string) (*model.MessageCapability, error) {
	if messageType == "" {
		return nil, fmt.Errorf("%w: message_type is required", contract.ErrInvalidRequest)
	}

	items, err := All()
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].MessageType == messageType {
			item := items[i]
			return &item, nil
		}
	}

	return nil, fmt.Errorf("%w: capability not found: %s", contract.ErrUnsupportedConfig, messageType)
}

func ValidateConfig(item *model.MessageConfig) error {
	if item == nil {
		return fmt.Errorf("%w: message config is required", contract.ErrInvalidRequest)
	}

	capability, err := Get(item.MessageType)
	if err != nil {
		return err
	}
	return ValidateConfigWith(capability, item)
}

func ValidateConfigWith(capability *model.MessageCapability, item *model.MessageConfig) error {
	if capability == nil {
		return fmt.Errorf("%w: message capability is required", contract.ErrInvalidRequest)
	}
	if item == nil {
		return fmt.Errorf("%w: message config is required", contract.ErrInvalidRequest)
	}
	if item.MessageType == "" {
		return fmt.Errorf("%w: message_type is required", contract.ErrInvalidRequest)
	}
	if capability.MessageType != item.MessageType {
		return fmt.Errorf("%w: capability mismatch for %s", contract.ErrUnsupportedConfig, item.MessageType)
	}
	if item.RealtimeEnabled && !capability.RealtimeSupported {
		return fmt.Errorf("%w: realtime is not supported for %s", contract.ErrUnsupportedConfig, item.MessageType)
	}
	if item.AggregateEnabled && !capability.AggregateSupported {
		return fmt.Errorf("%w: aggregate is not supported for %s", contract.ErrUnsupportedConfig, item.MessageType)
	}
	if item.AggregateEnabled && item.AggregatePeriodMinutes <= 0 {
		return fmt.Errorf("%w: aggregate_period_minutes is required", contract.ErrInvalidRequest)
	}
	if len(capability.AggregatePeriodMinutes) > 0 && item.AggregateEnabled && !containsInt(capability.AggregatePeriodMinutes, item.AggregatePeriodMinutes) {
		return fmt.Errorf("%w: unsupported aggregate_period_minutes for %s", contract.ErrUnsupportedConfig, item.MessageType)
	}
	if err := validateChannel(capability, item.Channel); err != nil {
		return err
	}
	return validateFilter(capability.FilterFields, item.Filter, item.MessageType)
}

func loadAll(fsys fs.FS) ([]model.MessageCapability, error) {
	entries, err := fs.ReadDir(fsys, "message_capabilities")
	if err != nil {
		return nil, err
	}

	items := make([]model.MessageCapability, 0, len(entries))
	seen := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join("message_capabilities", entry.Name())))
		if err != nil {
			return nil, err
		}
		var item model.MessageCapability
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("%w: parse capability %s: %v", contract.ErrUnsupportedConfig, entry.Name(), err)
		}
		if err := normalizeCapability(&item, entry.Name()); err != nil {
			return nil, err
		}
		if _, exists := seen[item.MessageType]; exists {
			return nil, fmt.Errorf("%w: duplicate capability for %s", contract.ErrUnsupportedConfig, item.MessageType)
		}
		seen[item.MessageType] = struct{}{}
		items = append(items, item)
	}

	slices.SortFunc(items, func(a, b model.MessageCapability) int {
		return strings.Compare(a.MessageType, b.MessageType)
	})
	return items, nil
}

func normalizeCapability(item *model.MessageCapability, filename string) error {
	if item == nil {
		return fmt.Errorf("%w: capability is required", contract.ErrInvalidRequest)
	}
	if item.MessageType == "" {
		return fmt.Errorf("%w: message_type is required in %s", contract.ErrUnsupportedConfig, filename)
	}
	base := strings.TrimSuffix(filename, ".json")
	if base != item.MessageType {
		return fmt.Errorf("%w: capability file %s does not match message_type %s", contract.ErrUnsupportedConfig, filename, item.MessageType)
	}
	if !item.RealtimeSupported && !item.AggregateSupported {
		return fmt.Errorf("%w: at least one capability is required for %s", contract.ErrUnsupportedConfig, item.MessageType)
	}
	if len(item.Channels) == 0 {
		return fmt.Errorf("%w: channels are required for %s", contract.ErrUnsupportedConfig, item.MessageType)
	}
	channelSeen := map[string]struct{}{}
	for i := range item.Channels {
		channel := strings.TrimSpace(item.Channels[i].Channel)
		if channel == "" {
			return fmt.Errorf("%w: channel is required for %s", contract.ErrUnsupportedConfig, item.MessageType)
		}
		if _, exists := channelSeen[channel]; exists {
			return fmt.Errorf("%w: duplicate channel %s for %s", contract.ErrUnsupportedConfig, channel, item.MessageType)
		}
		channelSeen[channel] = struct{}{}
		item.Channels[i].Channel = channel
	}

	fieldSeen := map[string]struct{}{}
	for i := range item.FilterFields {
		field := &item.FilterFields[i]
		field.Name = strings.TrimSpace(field.Name)
		field.Type = strings.TrimSpace(strings.ToLower(field.Type))
		if field.Name == "" {
			return fmt.Errorf("%w: filter field name is required for %s", contract.ErrUnsupportedConfig, item.MessageType)
		}
		if _, exists := fieldSeen[field.Name]; exists {
			return fmt.Errorf("%w: duplicate filter field %s for %s", contract.ErrUnsupportedConfig, field.Name, item.MessageType)
		}
		fieldSeen[field.Name] = struct{}{}
		if !validFilterType(field.Type) {
			return fmt.Errorf("%w: unsupported filter field type %s for %s", contract.ErrUnsupportedConfig, field.Type, item.MessageType)
		}
	}
	return nil
}

func validateChannel(capability *model.MessageCapability, channel model.ChannelPolicy) error {
	if channel.Channel == "" {
		return fmt.Errorf("%w: channel is required", contract.ErrUnsupportedConfig)
	}
	for _, item := range capability.Channels {
		if item.Channel == channel.Channel {
			return nil
		}
	}
	return fmt.Errorf("%w: unsupported channel %s for %s", contract.ErrUnsupportedConfig, channel.Channel, capability.MessageType)
}

func validateFilter(fields []model.FilterFieldCapability, raw json.RawMessage, messageType string) error {
	if len(fields) == 0 {
		if len(raw) == 0 || string(raw) == "null" {
			return nil
		}
		var any map[string]json.RawMessage
		if err := json.Unmarshal(raw, &any); err != nil {
			return fmt.Errorf("%w: parse filter for %s: %v", contract.ErrInvalidRequest, messageType, err)
		}
		if len(any) > 0 {
			return fmt.Errorf("%w: filter is not supported for %s", contract.ErrUnsupportedConfig, messageType)
		}
		return nil
	}
	if len(raw) == 0 || string(raw) == "null" {
		for _, field := range fields {
			if field.Required {
				return fmt.Errorf("%w: filter field %s is required for %s", contract.ErrInvalidRequest, field.Name, messageType)
			}
		}
		return nil
	}

	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("%w: parse filter for %s: %v", contract.ErrInvalidRequest, messageType, err)
	}

	fieldMap := make(map[string]model.FilterFieldCapability, len(fields))
	for _, field := range fields {
		fieldMap[field.Name] = field
	}
	for name := range values {
		field, ok := fieldMap[name]
		if !ok {
			return fmt.Errorf("%w: unsupported filter field %s for %s", contract.ErrUnsupportedConfig, name, messageType)
		}
		if err := validateField(field, values[name]); err != nil {
			return fmt.Errorf("%w: %v", contract.ErrUnsupportedConfig, err)
		}
	}
	for _, field := range fields {
		if field.Required {
			if _, ok := values[field.Name]; !ok {
				return fmt.Errorf("%w: filter field %s is required for %s", contract.ErrInvalidRequest, field.Name, messageType)
			}
		}
	}
	return nil
}

func validateField(field model.FilterFieldCapability, raw json.RawMessage) error {
	switch field.Type {
	case "string":
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("filter field %s must be string", field.Name)
		}
		if len(field.Options) > 0 && !containsString(field.Options, value) {
			return fmt.Errorf("filter field %s has unsupported value %q", field.Name, value)
		}
	case "integer":
		var value json.Number
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("filter field %s must be integer", field.Name)
		}
		if _, err := strconv.ParseInt(value.String(), 10, 64); err != nil {
			return fmt.Errorf("filter field %s must be integer", field.Name)
		}
	case "number":
		var value json.Number
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("filter field %s must be number", field.Name)
		}
		if _, err := value.Float64(); err != nil {
			return fmt.Errorf("filter field %s must be number", field.Name)
		}
	case "boolean":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("filter field %s must be boolean", field.Name)
		}
	case "string_array":
		var values []string
		if err := json.Unmarshal(raw, &values); err != nil {
			return fmt.Errorf("filter field %s must be string array", field.Name)
		}
		for _, value := range values {
			if len(field.Options) > 0 && !containsString(field.Options, value) {
				return fmt.Errorf("filter field %s has unsupported value %q", field.Name, value)
			}
		}
	case "integer_array":
		var values []json.Number
		if err := json.Unmarshal(raw, &values); err != nil {
			return fmt.Errorf("filter field %s must be integer array", field.Name)
		}
		for _, value := range values {
			if _, err := strconv.ParseInt(value.String(), 10, 64); err != nil {
				return fmt.Errorf("filter field %s must be integer array", field.Name)
			}
		}
	case "number_array":
		var values []json.Number
		if err := json.Unmarshal(raw, &values); err != nil {
			return fmt.Errorf("filter field %s must be number array", field.Name)
		}
		for _, value := range values {
			if _, err := value.Float64(); err != nil {
				return fmt.Errorf("filter field %s must be number array", field.Name)
			}
		}
	case "boolean_array":
		var values []bool
		if err := json.Unmarshal(raw, &values); err != nil {
			return fmt.Errorf("filter field %s must be boolean array", field.Name)
		}
	case "object":
		var value map[string]json.RawMessage
		if err := json.Unmarshal(raw, &value); err != nil {
			return fmt.Errorf("filter field %s must be object", field.Name)
		}
	default:
		return fmt.Errorf("unsupported filter field type %s", field.Type)
	}
	return nil
}

func validFilterType(value string) bool {
	switch value {
	case "string", "integer", "number", "boolean", "string_array", "integer_array", "number_array", "boolean_array", "object":
		return true
	default:
		return false
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func containsInt(items []int, target int) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
