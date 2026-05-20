package manifest

import (
	"fmt"
	"os"
	"testing"
)

// Manual test - run with: go test -v ./internal/manifest -run TestManifestDemo
func TestManifestDemo(t *testing.T) {
	// This test demonstrates the manifest API
	// Set a test data directory
	os.Setenv("SAT_DATA", "/tmp/sat-test-"+t.Name())
	defer os.RemoveAll(os.Getenv("SAT_DATA"))

	// Ensure directory exists
	if err := os.MkdirAll(DataDir(), 0755); err != nil {
		t.Fatal(err)
	}

	fmt.Println("\n=== Manifest API Demo ===")
	fmt.Println()

	// Test 1: Add packages
	fmt.Println("1. Adding packages to manifest:")
	Add("ripgrep", BuildSourceString("cargo", "", "14.1.0"))
	Add("fd", BuildSourceString("cargo", "", "9.0.0"))
	Add("bat", BuildSourceString("cargo", "", "0.24.0"))
	Add("vim", BuildSourceString("system", "", "9.0"))
	fmt.Println("   ✓ Added 4 packages")

	// Test 2: Get individual package
	fmt.Println("\n2. Getting individual package:")
	if src := Get("ripgrep"); src != "" {
		fmt.Printf("   ripgrep: %s\n", src)
		fmt.Printf("   - Source type: %s\n", GetSourceType(src))
		fmt.Printf("   - Version: %s\n", GetSourceVersion(src))
	}

	// Test 3: Check if package exists
	fmt.Println("\n3. Checking package existence:")
	fmt.Printf("   Has ripgrep? %v\n", Has("ripgrep"))
	fmt.Printf("   Has nonexistent? %v\n", Has("nonexistent"))

	// Test 4: Check multiple packages
	fmt.Println("\n4. Checking multiple packages:")
	tools := []string{"ripgrep", "fd", "bat", "vim"}
	for _, tool := range tools {
		if src := Get(tool); src != "" {
			sourceType := GetSourceType(src)
			version := GetSourceVersion(src)
			fmt.Printf("   %s [%s] v%s\n", tool, sourceType, version)
		}
	}

	// Test 5: Remove a package
	fmt.Println("\n5. Removing a package:")
	if err := Remove("fd"); err != nil {
		t.Errorf("Failed to remove fd: %v", err)
	}
	fmt.Println("   ✓ Removed fd")
	fmt.Printf("   Has fd? %v\n", Has("fd"))

	// Test 6: Source string parsing
	fmt.Println("\n6. Source string parsing:")
	testSources := []string{
		"cargo::14.1.0",
		"gh:BurntSushi/ripgrep:v14.1.0",
		"flatpak:com.github.app:1.2.3",
		"nixos::",
	}
	for _, src := range testSources {
		fmt.Printf("   %s\n", src)
		fmt.Printf("     - Type: %s\n", GetSourceType(src))
		fmt.Printf("     - Identity: %s\n", GetSourceIdentity(src))
		fmt.Printf("     - Version: %s\n", GetSourceVersion(src))
	}

	fmt.Println("\n=== All tests passed ===")
}
