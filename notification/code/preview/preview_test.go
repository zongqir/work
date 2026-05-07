package preview

import (
	"path/filepath"
	"testing"
)

func TestFromFiles(t *testing.T) {
	root := ".."

	result, err := FromFiles(
		filepath.Join(root, "sample_request.json"),
		filepath.Join(root, "sample_result.json"),
		filepath.Join(root, "sample_policy.json"),
		filepath.Join(root, "templates"),
		true,
	)
	if err != nil {
		t.Fatalf("FromFiles failed: %v", err)
	}
	if result.TemplateContext == nil {
		t.Fatal("expected template context")
	}
	if len(result.Rendered) != 1 {
		t.Fatalf("expected 1 rendered channel, got %d", len(result.Rendered))
	}
}
