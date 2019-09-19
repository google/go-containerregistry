// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package name

import (
	"strings"
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

	for _, name := range goodWeakValidationTagDigestNames {
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

	for _, name := range goodStrictValidationTagDigestNames {
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

func TestParseWriteReference(t *testing.T) {
	for _, name := range goodWeakValidationDigestNames {
		ref, err := ParseWriteReference(name, WeakValidation)
		if err != nil {
			t.Errorf("ParseWriteReference(%q); %v", name, err)
		}
		dig, err := NewDigest(name, WeakValidation)
		if err != nil {
			t.Errorf("NewDigest(%q); %v", name, err)
		}
		if ref != dig {
			t.Errorf("ParseWriteReference(%q) != NewDigest(%q); got %v, want %v", name, name, ref, dig)
		}
	}

	for _, name := range goodStrictValidationDigestNames {
		ref, err := ParseWriteReference(name, StrictValidation)
		if err != nil {
			t.Errorf("ParseWriteReference(%q); %v", name, err)
		}
		dig, err := NewDigest(name, StrictValidation)
		if err != nil {
			t.Errorf("NewDigest(%q); %v", name, err)
		}
		if ref != dig {
			t.Errorf("ParseWriteReference(%q) != NewDigest(%q); got %v, want %v", name, name, ref, dig)
		}
	}

	for _, name := range badDigestNames {
		if _, err := ParseWriteReference(name, WeakValidation); err == nil {
			t.Errorf("ParseWriteReference(%q); expected error, got none", name)
		}
	}

	for _, name := range goodWeakValidationTagNames {
		ref, err := ParseWriteReference(name, WeakValidation)
		if err != nil {
			t.Errorf("ParseWriteReference(%q); %v", name, err)
		}
		tag, err := NewTag(name, WeakValidation)
		if err != nil {
			t.Errorf("NewTag(%q); %v", name, err)
		}
		if ref != tag {
			t.Errorf("ParseWriteReference(%q) != NewTag(%q); got %v, want %v", name, name, ref, tag)
		}
	}

	for _, name := range goodStrictValidationTagNames {
		ref, err := ParseWriteReference(name, StrictValidation)
		if err != nil {
			t.Errorf("ParseWriteReference(%q); %v", name, err)
		}
		tag, err := NewTag(name, StrictValidation)
		if err != nil {
			t.Errorf("NewTag(%q); %v", name, err)
		}
		if ref != tag {
			t.Errorf("ParseWriteReference(%q) != NewTag(%q); got %v, want %v", name, name, ref, tag)
		}
	}

	for _, name := range goodStrictValidationTagDigestNames {
		ref, err := ParseWriteReference(name, StrictValidation)
		if err != nil {
			t.Errorf("ParseWriteReference(%q); %v", name, err)
		}
		base := strings.Split(name, digestDelim)[0]

		tag, err := NewTag(base, StrictValidation)
		if err != nil {
			t.Errorf("NewTag(%q); %v", base, err)
		}
		if ref != tag {
			t.Errorf("ParseWriteReference(%q) != NewTag(%q); got %v, want %v", name, base, ref, tag)
		}
	}

	for _, name := range goodWeakValidationTagDigestNames {
		ref, err := ParseWriteReference(name, WeakValidation)
		if err != nil {
			t.Errorf("ParseWriteReference(%q); %v", name, err)
		}
		base := strings.Split(name, digestDelim)[0]

		tag, err := NewTag(base, WeakValidation)
		if err != nil {
			t.Errorf("NewTag(%q); %v", base, err)
		}
		if ref != tag {
			t.Errorf("ParseWriteReference(%q) != NewTag(%q); got %v, want %v", name, base, ref, tag)
		}
	}

	for _, name := range badTagNames {
		if _, err := ParseWriteReference(name, WeakValidation); err == nil {
			t.Errorf("ParseWriteReference(%q); expected error, got none", name)
		}
	}
}
