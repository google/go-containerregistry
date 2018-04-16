package name

import (
	"testing"
)

func TestParseReference(t *testing.T) {
	for _, name := range goodWeakValidationDigestNames {
		ref, err := ParseReference(name, WeakValidation)
		if err != nil {
			t.Errorf("ParseReference(%q); %v", name, err)
		}
		dig, err := NewDigest(name, WeakValidation)
		if err != nil {
			t.Errorf("NewDigest(%q); %v", name, err)
		}
		if ref != dig {
			t.Errorf("ParseReference(%q) != NewDigest(%q); got %v, want %v", name, name, ref, dig)
		}
	}

	for _, name := range goodStrictValidationDigestNames {
		ref, err := ParseReference(name, StrictValidation)
		if err != nil {
			t.Errorf("ParseReference(%q); %v", name, err)
		}
		dig, err := NewDigest(name, StrictValidation)
		if err != nil {
			t.Errorf("NewDigest(%q); %v", name, err)
		}
		if ref != dig {
			t.Errorf("ParseReference(%q) != NewDigest(%q); got %v, want %v", name, name, ref, dig)
		}
	}

	for _, name := range badDigestNames {
		if _, err := ParseReference(name, WeakValidation); err == nil {
			t.Errorf("ParseReference(%q); expected error, got none", name)
		}
	}

	for _, name := range goodWeakValidationTagNames {
		ref, err := ParseReference(name, WeakValidation)
		if err != nil {
			t.Errorf("ParseReference(%q); %v", name, err)
		}
		tag, err := NewTag(name, WeakValidation)
		if err != nil {
			t.Errorf("NewTag(%q); %v", name, err)
		}
		if ref != tag {
			t.Errorf("ParseReference(%q) != NewTag(%q); got %v, want %v", name, name, ref, tag)
		}
	}

	for _, name := range goodStrictValidationTagNames {
		ref, err := ParseReference(name, StrictValidation)
		if err != nil {
			t.Errorf("ParseReference(%q); %v", name, err)
		}
		tag, err := NewTag(name, StrictValidation)
		if err != nil {
			t.Errorf("NewTag(%q); %v", name, err)
		}
		if ref != tag {
			t.Errorf("ParseReference(%q) != NewTag(%q); got %v, want %v", name, name, ref, tag)
		}
	}

	for _, name := range badTagNames {
		if _, err := ParseReference(name, WeakValidation); err == nil {
			t.Errorf("ParseReference(%q); expected error, got none", name)
		}
	}
}
